// Package mail SMTP ile basit e-posta gonderimi (sifre sifirlama linki vb.).
// SMTP_HOST bos ise gonderim yapilmaz; cagiran link'i ekranda gosterir.
package mail

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string // "SmartRate <no-reply@smartrate.com>"
	TLSMode  string // starttls (varsayilan) | tls | none
}

type Mailer struct{ cfg Config }

var ErrNotConfigured = errors.New("SMTP ayarlanmamis (SMTP_HOST bos)")

func New(cfg Config) *Mailer { return &Mailer{cfg: cfg} }

func (m *Mailer) Configured() bool { return m != nil && strings.TrimSpace(m.cfg.Host) != "" }

// Send duz metin + HTML (multipart/alternative) e-posta gonderir.
func (m *Mailer) Send(to, subject, text, html string) error {
	if !m.Configured() {
		return ErrNotConfigured
	}
	port := m.cfg.Port
	if port == 0 {
		port = 587
	}
	from := m.cfg.From
	if from == "" {
		from = m.cfg.Username
	}
	fromAddr := from
	if i := strings.Index(from, "<"); i >= 0 && strings.HasSuffix(from, ">") {
		fromAddr = from[i+1 : len(from)-1]
	}
	boundary := fmt.Sprintf("sr%d", time.Now().UnixNano())
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nDate: %s\r\n", from, to, encodeHeader(subject), time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%s\r\n\r\n", boundary)
	fmt.Fprintf(&b, "--%s\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n%s\r\n", boundary, text)
	if html != "" {
		fmt.Fprintf(&b, "--%s\r\nContent-Type: text/html; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n%s\r\n", boundary, html)
	}
	fmt.Fprintf(&b, "--%s--\r\n", boundary)

	addr := net.JoinHostPort(m.cfg.Host, fmt.Sprint(port))
	var auth smtp.Auth
	if m.cfg.Username != "" {
		auth = smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	}
	mode := strings.ToLower(m.cfg.TLSMode)
	if mode == "" {
		mode = "starttls"
		if port == 465 {
			mode = "tls"
		}
	}
	if mode == "tls" {
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 15 * time.Second}, "tcp", addr, &tls.Config{ServerName: m.cfg.Host})
		if err != nil {
			return err
		}
		c, err := smtp.NewClient(conn, m.cfg.Host)
		if err != nil {
			return err
		}
		defer c.Close()
		return send(c, auth, fromAddr, to, b.String())
	}
	conn, err := net.DialTimeout("tcp", addr, 15*time.Second)
	if err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		return err
	}
	defer c.Close()
	if mode == "starttls" {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: m.cfg.Host}); err != nil {
				return err
			}
		}
	}
	return send(c, auth, fromAddr, to, b.String())
}

func send(c *smtp.Client, auth smtp.Auth, from, to, msg string) error {
	if auth != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(auth); err != nil {
				return err
			}
		}
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// encodeHeader Turkce karakterli konu satiri icin RFC 2047.
func encodeHeader(s string) string {
	for _, r := range s {
		if r > 127 {
			return "=?UTF-8?B?" + base64(s) + "?="
		}
	}
	return s
}

func base64(s string) string {
	const tbl = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	src := []byte(s)
	var out strings.Builder
	for i := 0; i < len(src); i += 3 {
		var n uint32
		rem := len(src) - i
		for j := 0; j < 3; j++ {
			n <<= 8
			if j < rem {
				n |= uint32(src[i+j])
			}
		}
		for j := 0; j < 4; j++ {
			if j <= rem {
				out.WriteByte(tbl[(n>>(18-6*uint(j)))&63])
			} else {
				out.WriteByte('=')
			}
		}
	}
	return out.String()
}
