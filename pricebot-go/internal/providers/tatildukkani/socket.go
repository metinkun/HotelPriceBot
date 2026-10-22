package tatildukkani

// Oda/fiyat listesi HTTP ile degil, Socket.IO (wss://ws.tatildukkani.com) uzerinden gelir.
// Akis (sitenin fillSocketData'si): login -> available-hotel{uid} -> available-hotel yaniti.
// Burada Engine.IO v4 / Socket.IO v4 protokolunun gereken kadari elle uygulanir
// (gobwas/ws zaten chromedp bagimliligi olarak mevcut; yeni modul eklenmez).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

const (
	wsURL = "wss://ws.tatildukkani.com/socket.io/?EIO=4&transport=websocket"
	// socketTimeout tum ws akisi icin ust sinir (gozlenen sure ~1-2 sn).
	socketTimeout = 40 * time.Second
)

// flexFloat sayi veya "33150.00" gibi string gelen fiyat alanlari icin.
type flexFloat float64

func (f *flexFloat) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	var v float64
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return fmt.Errorf("sayi degil: %s", b)
	}
	*f = flexFloat(v)
	return nil
}

// Room socket yanitindaki bir oda+pansiyon teklifi (konaklama toplami, TRY).
type Room struct {
	PriceCode     string    `json:"priceCode"`
	RoomCode      string    `json:"roomCode"`
	RoomTitle     string    `json:"roomTitle"`
	ConceptCode   string    `json:"conceptCode"`
	ConceptTitle  string    `json:"conceptTitle"`
	Status        string    `json:"status"`
	Currency      string    `json:"currency"`
	Fee           flexFloat `json:"fee"`           // satis fiyati
	RealFee       flexFloat `json:"realFee"`       // ayni deger, string
	DiscountPrice flexFloat `json:"discountPrice"` // indirimli (genelde fee ile ayni)
	PosterPrice   flexFloat `json:"posterPrice"`   // liste (afis) fiyati
	PosterRate    flexFloat `json:"posterRate"`    // afis indirim yuzdesi
	DiscountRate  flexFloat `json:"discountRate"`
	RoomCampaign  []struct {
		Name    string    `json:"name"`
		Rate    flexFloat `json:"rate"`
		EndDate string    `json:"endDate"`
	} `json:"roomCampaign"`
	Days []struct {
		Date string    `json:"date"`
		Fee  flexFloat `json:"fee"`
	} `json:"days"`
}

// AvailableHotel "available-hotel" olayinin govdesi.
type AvailableHotel struct {
	UID        string `json:"uid"`
	HotelID    int    `json:"hotelId"`
	Status     string `json:"status"` // AVAILABLE | NOT_AVAILABLE | ...
	StatusDesc any    `json:"statusDesc"`
	HashCode   string `json:"hashCode"`
	RefererID  string `json:"refererId"`
	Callback   bool   `json:"callback"`
	Rooms      []Room `json:"rooms"`
}

// sioEvent 42["evt", "<json string>"] paketini olusturur (site veriyi string olarak gonderir).
func sioEvent(evt string, payload any) ([]byte, error) {
	s, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	arr, err := json.Marshal([]string{evt, string(s)})
	if err != nil {
		return nil, err
	}
	return append([]byte("42"), arr...), nil
}

// parseEvent 42["evt", payload] -> evt, payload (payload string ise icindeki JSON'a cozulur).
func parseEvent(msg []byte) (string, json.RawMessage, bool) {
	if !strings.HasPrefix(string(msg), "42") {
		return "", nil, false
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(msg[2:], &arr); err != nil || len(arr) == 0 {
		return "", nil, false
	}
	var evt string
	if err := json.Unmarshal(arr[0], &evt); err != nil {
		return "", nil, false
	}
	var payload json.RawMessage
	if len(arr) > 1 {
		payload = arr[1]
		var inner string
		if err := json.Unmarshal(payload, &inner); err == nil {
			payload = json.RawMessage(inner)
		}
	}
	return evt, payload, true
}

// waitAvailability ws'e baglanir, login + available-hotel gonderir ve oda listesini bekler.
func waitAvailability(ctx context.Context, socketToken, uid string) (*AvailableHotel, error) {
	ctx, cancel := context.WithTimeout(ctx, socketTimeout)
	defer cancel()
	deadline, _ := ctx.Deadline()

	hdr := http.Header{}
	hdr.Set("Origin", siteBase)
	hdr.Set("User-Agent", ua)
	dialer := ws.Dialer{Timeout: 15 * time.Second, Header: ws.HandshakeHeaderHTTP(hdr)}
	conn, _, _, err := dialer.Dial(ctx, wsURL)
	if err != nil {
		return nil, fmt.Errorf("Tatildukkani ws baglantisi kurulamadi: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(deadline)

	send := func(b []byte) error { return wsutil.WriteClientText(conn, b) }
	emit := func(evt string, payload any) error {
		b, err := sioEvent(evt, payload)
		if err != nil {
			return err
		}
		return send(b)
	}

	var last *AvailableHotel
	for {
		msg, err := wsutil.ReadServerText(conn)
		if err != nil {
			if last != nil {
				return last, nil
			}
			if ctx.Err() != nil {
				return nil, fmt.Errorf("Tatildukkani ws yaniti zaman asimi (uid %s)", uid)
			}
			return nil, fmt.Errorf("Tatildukkani ws okuma hatasi: %w", err)
		}
		s := string(msg)
		switch {
		case s == "2": // Engine.IO ping -> pong
			if err := send([]byte("3")); err != nil {
				return nil, err
			}
		case strings.HasPrefix(s, "0"): // Engine.IO open -> Socket.IO namespace connect
			if err := send([]byte("40")); err != nil {
				return nil, err
			}
		case strings.HasPrefix(s, "40"): // namespace connected -> login
			if err := emit("login", map[string]any{
				"socketToken": socketToken,
				"available":   []map[string]any{{"type": "HOTEL", "token": nil}},
			}); err != nil {
				return nil, err
			}
		case strings.HasPrefix(s, "41"), strings.HasPrefix(s, "44"):
			return nil, fmt.Errorf("Tatildukkani ws oturumu reddedildi: %s", truncate(s, 200))
		case strings.HasPrefix(s, "42"):
			evt, payload, ok := parseEvent(msg)
			if !ok {
				continue
			}
			switch evt {
			case "is_login":
				if err := emit("available-hotel", map[string]any{"uid": uid, "socketToken": socketToken}); err != nil {
					return nil, err
				}
			case "available-hotel":
				var av AvailableHotel
				if err := json.Unmarshal(payload, &av); err != nil {
					return nil, fmt.Errorf("Tatildukkani available-hotel parse edilemedi: %w", err)
				}
				// Site de ayni geri bildirimi yapar; sunucu tarafinda arama kaydini kapatir.
				_ = emit("available-hotel-callback", map[string]any{"hotelAvailableToken": av.HashCode, "refererId": av.RefererID})
				if len(av.Rooms) > 0 || av.Status != "" {
					return &av, nil
				}
				last = &av
			}
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
