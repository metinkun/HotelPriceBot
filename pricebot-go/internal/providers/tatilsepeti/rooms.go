package tatilsepeti

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"pricebot/internal/httpx"
	"pricebot/internal/provider"
	"pricebot/internal/types"
)

// Oda + pansiyon listesi: sitenin kendi detay sayfasina form POST (hotelDomesticDetail.js:
// TsAjax.post(location.pathname, {Search: "oda:2-5;tarih:15.12.2026,18.12.2026;click:true"})).
// Yanit JSON; `roomList` alani her odayi bir kart olarak iceren sunucu tarafli HTML'dir.
// Cerez/token gerekmez (yine de oturumlu postJSON kullanilir: engellenme halinde yenilenir).

type roomListResponse struct {
	RoomList     string `json:"roomList"`
	ShowLoadMore bool   `json:"showLoadMore"`
	PriceRange   string `json:"PriceRange"`
}

// searchParam "oda:<yetiskin>[-<cocukYasi>...];tarih:dd.MM.yyyy,dd.MM.yyyy;click:true"
func searchParam(checkIn, checkOut string, adults int, childAges []int) string {
	oda := fmt.Sprint(adults)
	for _, a := range childAges {
		oda += "-" + fmt.Sprint(a)
	}
	return "oda:" + oda + ";tarih:" + toTSDate(checkIn) + "," + toTSDate(checkOut) + ";click:true"
}

// fetchRoomList otel sayfasina POST ile tarihli oda listesini ceker.
func (p *Provider) fetchRoomList(ctx context.Context, pageURL, checkIn, checkOut string, adults int, childAges []int) (*roomListResponse, error) {
	if pageURL == "" || pageURL == homeURL {
		return nil, fmt.Errorf("TatilSepeti oda listesi icin otel sayfasi URL'i (sourceUrl) gerekli")
	}
	form := httpx.FormValues{{"Search", searchParam(checkIn, checkOut, adults, childAges)}}
	raw, err := p.postJSON(ctx, strings.SplitN(pageURL, "?", 2)[0], form, pageURL, 30*time.Second)
	if err != nil {
		return nil, err
	}
	var d roomListResponse
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("TatilSepeti oda listesi parse edilemedi: %w", err)
	}
	return &d, nil
}

var (
	reDiscountPct = regexp.MustCompile(`(\d+)\s*%|%\s*(\d+)`)
	reSoldOutCard = regexp.MustCompile(`(?i)m[üu]sait\s*de[ğg]il|sat[ıi][şs]a\s*kapal[ıi]|tarih\s*de[ğg]i[şs]tir|dolu`)
)

func cleanPrice(s *goquery.Selection) float64 {
	txt := strings.TrimSpace(s.Text())
	return provider.ParseTRPrice(txt)
}

func discountOf(s *goquery.Selection) float64 {
	m := reDiscountPct.FindStringSubmatch(s.Text())
	if m == nil {
		return 0
	}
	v := m[1]
	if v == "" {
		v = m[2]
	}
	return provider.ParseTRPrice(v)
}

// parseRoomList roomList HTML'indeki kartlari oda tekliflerine cevirir.
// Tek pansiyon: __price-div__single; coklu pansiyon: __pension-types__badge + takip eden __multiple blogu.
// Rezervasyon dugmesi yoksa (Tarih Degistir / kapasite / fiyat yok) teklif sold_out; fiyat yine de okunur.
func parseRoomList(html, defaultBoard string) []types.RoomOffer {
	rooms := []types.RoomOffer{}
	if strings.TrimSpace(html) == "" {
		return rooms
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return rooms
	}
	seen := map[string]bool{}
	add := func(room, board string, block *goquery.Selection, soldOut bool) {
		price := cleanPrice(block.Find(`[class*="discount-price"]`).First())
		list := cleanPrice(block.Find(`[class*="default-price"]`).First())
		if price <= 0 && list <= 0 {
			return
		}
		if price <= 0 {
			price = list
		}
		if board == "" {
			board = defaultBoard
		}
		key := fmt.Sprintf("%s|%s|%v", room, board, price)
		if seen[key] {
			return
		}
		seen[key] = true
		st := types.RoomAvailable
		if soldOut || block.Find(`[class*="reservation-div__btn"]`).Length() == 0 {
			st = types.RoomSoldOut
		}
		ro := types.NewRoom(room, board, price, list, "TL", st)
		if r := discountOf(block.Find(`[class*="all-prices__discount"]`).First()); r > 0 {
			ro.DiscountRate = r
		}
		rooms = append(rooms, ro)
	}
	doc.Find(`.hotel-detail-cards[data-roomtypeid]`).Each(func(_ int, card *goquery.Selection) {
		room := strings.TrimSpace(card.Find(`[class*="content-div__title"] h3`).First().Text())
		if room == "" {
			room = strings.TrimSpace(card.Find("h3").First().Text())
		}
		alert := strings.TrimSpace(card.Find(`.hotel-detail-cards-alert-bottom, p.hotelDetailYellowError`).Text())
		soldOut := alert != "" && reSoldOutCard.MatchString(alert)
		single := card.Find(`[class*="price-div__single"]`).First()
		if single.Length() > 0 {
			board := strings.TrimSpace(card.Find(`[class*="all-inclusive"]`).First().Text())
			add(room, board, single, soldOut)
		}
		card.Find(`[class*="pension-types"]:not([class*="__badge"])`).Each(func(_ int, pension *goquery.Selection) {
			board := strings.TrimSpace(pension.Find("span").First().Text())
			multiple := pension.NextAllFiltered(`[class*="multiple"]`).First()
			if multiple.Length() == 0 {
				return
			}
			add(room, board, multiple, soldOut)
		})
	})
	return types.SortRooms(rooms)
}

// attachRooms fiyat sonucuna tum oda tekliflerini ekler (ek bir istek; hata olursa liste bos kalir).
func (p *Provider) attachRooms(ctx context.Context, pageURL, checkIn, checkOut string, adults int, childAges []int, price *types.NormalizedPrice) {
	if price == nil {
		return
	}
	rl, err := p.fetchRoomList(ctx, pageURL, checkIn, checkOut, adults, childAges)
	if err != nil {
		return
	}
	price.Rooms = parseRoomList(rl.RoomList, types.Deref(price.BoardType))
}
