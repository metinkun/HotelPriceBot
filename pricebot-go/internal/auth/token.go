package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Minimal HS256 JWT (admin oturumu icin). Dis bagimlilik yok.

type Claims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Role  string `json:"role"`
	Exp   int64  `json:"exp"`
	Iat   int64  `json:"iat"`
	Dev   int    `json:"dev,omitempty"` // portal: kayitli cihaz id (cihaz dogrulamasi)
}

var ErrTokenInvalid = errors.New("gecersiz veya suresi dolmus oturum")

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func sign(secret []byte, data string) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(data))
	return b64(m.Sum(nil))
}

// SignToken HS256 token uretir.
func SignToken(secret []byte, c Claims) (string, error) {
	h := b64([]byte(`{"alg":"HS256","typ":"JWT"}`))
	p, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	data := h + "." + b64(p)
	return data + "." + sign(secret, data), nil
}

// ParseToken imzayi ve sureyi dogrular.
func ParseToken(secret []byte, token string) (*Claims, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return nil, ErrTokenInvalid
	}
	data := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(sign(secret, data)), []byte(parts[2])) {
		return nil, ErrTokenInvalid
	}
	pb, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrTokenInvalid
	}
	var c Claims
	if err := json.Unmarshal(pb, &c); err != nil {
		return nil, ErrTokenInvalid
	}
	if c.Exp > 0 && time.Now().Unix() > c.Exp {
		return nil, ErrTokenInvalid
	}
	return &c, nil
}

// HashKey API anahtarinin sha256 hex ozeti.
func HashKey(key string) string {
	s := sha256.Sum256([]byte(strings.TrimSpace(key)))
	return hexEncode(s[:])
}

const hexdigits = "0123456789abcdef"

func hexEncode(b []byte) string {
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexdigits[v>>4]
		out[i*2+1] = hexdigits[v&0x0f]
	}
	return string(out)
}
