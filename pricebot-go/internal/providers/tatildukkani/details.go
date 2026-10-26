package tatildukkani

// Statik otel detayi (provider.DetailProvider).
//
// Kaynak: POST /api/seo_page/slug (tarihsiz) yanitindaki `data.result` nesnesi: ad, yildiz, adres, koordinat,
// gorseller (fullPath + imageHotelTypes grup adi), ozellik gruplari (facality: evet/hayir, ucretli/ucretsiz,
// sayi, secim, serbest metin ve zengin metin "…Bilgisi" alanlari), temalar, pansiyonlar, etkinlikler,
// misafir puanlari. Odalar bu yanitta yoktur (site: "Odalari goruntulemek icin tarih giriniz"); oda listesi
// yalnizca tarihli fiyat akisinda (socket) gelir, bu yuzden Rooms bos birakilir.
// Cikti deterministiktir: siralamalar sequence/line/id ve ada gore sabitlenir; yorumlar (kisisel veri) alinmaz.

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"pricebot/internal/provider"
)

// pageDetail detay icin kullanilan `data` alanlari.
type pageDetail struct {
	Type   string     `json:"type"`
	TypeID int        `json:"typeId"`
	Title  string     `json:"title"`
	Slug   string     `json:"slug"`
	Result pageResult `json:"result"`
}

type pageResult struct {
	ID               int               `json:"id"`
	Name             string            `json:"name"`
	Star             string            `json:"star"`
	Latitude         string            `json:"latitude"`
	Longitude        string            `json:"longitude"`
	Address          string            `json:"address"`
	HotelDestination *namedItem        `json:"hotelDestination"`
	Destination      []namedItem       `json:"destination"`
	ImageHotelTypes  map[string]string `json:"imageHotelTypes"`
	Reviews          *pageReviews      `json:"reviews"`
	Images           []pageImage       `json:"images"`
	SingleImage      *pageImage        `json:"singleImage"`
	Facality         []facGroup        `json:"facality"`
	Themes           []namedItem       `json:"themes"`
	Concepts         []string          `json:"concepts"`
	EventGroups      []eventGroup      `json:"eventGroups"`
	SeoPage          *struct {
		Slug string `json:"slug"`
	} `json:"seoPage"`
}

type namedItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type pageReviews struct {
	Review       []reviewPoint `json:"review"`
	TotalComment int           `json:"totalComment"`
	TotalReviews int           `json:"totalReviews"`
	TotalPoint   float64       `json:"totalPoint"`
}

type reviewPoint struct {
	Name  string  `json:"name"`
	Point float64 `json:"point"`
}

type pageImage struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	FullPath    string `json:"fullPath"`
	HotelType   *int   `json:"hotelType"`
	Line        int    `json:"line"`
	Type        string `json:"type"`
}

type facGroup struct {
	ID       int       `json:"id"`
	Name     string    `json:"name"`
	Sequence int       `json:"sequence"`
	Facality []facItem `json:"facality"`
}

type facItem struct {
	ID            int             `json:"id"`
	FeatureID     int             `json:"featureId"`
	Name          string          `json:"name"`
	Sequence      int             `json:"sequence"`
	InputYesNo    *bool           `json:"inputYesNo"`
	InputNumber   json.RawMessage `json:"inputNumber"` // sayi veya "826 Room" gibi metin
	InputPaidFree *bool           `json:"inputPaidFree"`
	InputText     *string         `json:"inputText"`
	InputRichtext *string         `json:"inputRichtext"`
	InputSelect   *string         `json:"inputSelect"`
}

type eventGroup struct {
	Title  string `json:"title"`
	Status bool   `json:"status"`
	Date   []struct {
		Date string `json:"date"`
	} `json:"date"`
}

// GetDetails otel detay sayfasini ortak semaya normalize eder.
func (p *Provider) GetDetails(ctx context.Context, m *provider.Mapping) (*provider.HotelDetails, error) {
	slug, err := slugOf(m)
	if err != nil {
		return nil, err
	}
	data, err := fetchPageData(ctx, slug, "", "", 0, "")
	if err != nil {
		return nil, err
	}
	src := m.MetaString("sourceUrl")
	if src == "" {
		src = siteBase + "/" + slug
	}
	out, err := buildDetails(data, src)
	if err != nil {
		return nil, err
	}
	out.FetchedAt = time.Now()
	return out, nil
}

var (
	reCheckIn  = regexp.MustCompile(`(?i)giri[şs] saati\D{0,25}(\d{1,2}[:.]\d{2})`)
	reCheckOut = regexp.MustCompile(`(?i)[çc][ıi]k[ıi][şs] saati\D{0,25}(\d{1,2}[:.]\d{2})`)
	reQuery    = regexp.MustCompile(`[?#].*$`)
)

// sentenceWith metindeki, (kucuk harfli) anahtari iceren cumleyi doner; bulunamazsa metnin tamami.
func sentenceWith(text, key string) string {
	for _, s := range strings.Split(text, ". ") {
		if strings.Contains(strings.ToLower(s), key) {
			s = strings.TrimSpace(s)
			if !strings.HasSuffix(s, ".") {
				s += "."
			}
			return s
		}
	}
	return text
}

// rawScalar json ham degeri okunur metne cevirir ("826 Room", 1982 -> "1982").
func rawScalar(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	var f float64
	if json.Unmarshal(raw, &f) == nil {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return strings.Trim(string(raw), `"`)
}

// facLabel ozellik satiri etiketi; gosterilmeyecekse "".
func facLabel(it facItem) string {
	name := provider.DecodeEntities(it.Name)
	if name == "" || it.InputRichtext != nil {
		return ""
	}
	if v := rawScalar(it.InputNumber); v != "" {
		return name + ": " + v
	}
	if it.InputSelect != nil && strings.TrimSpace(*it.InputSelect) != "" {
		return name + ": " + provider.DecodeEntities(*it.InputSelect)
	}
	if it.InputText != nil && strings.TrimSpace(*it.InputText) != "" {
		return name + ": " + provider.DecodeEntities(*it.InputText)
	}
	if it.InputPaidFree != nil {
		if *it.InputPaidFree {
			return name + " (Ücretli)"
		}
		return name + " (Ücretsiz)"
	}
	if it.InputYesNo != nil {
		if *it.InputYesNo {
			return name
		}
		return ""
	}
	return name
}

func sortedGroups(groups []facGroup) []facGroup {
	out := append([]facGroup(nil), groups...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Sequence != out[j].Sequence {
			return out[i].Sequence < out[j].Sequence
		}
		return out[i].ID < out[j].ID
	})
	for gi := range out {
		items := append([]facItem(nil), out[gi].Facality...)
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].Sequence != items[j].Sequence {
				return items[i].Sequence < items[j].Sequence
			}
			return items[i].Name < items[j].Name
		})
		out[gi].Facality = items
	}
	return out
}

// buildDetails saf donusum (test edilebilir): seo_page/slug `data` -> HotelDetails.
func buildDetails(data json.RawMessage, sourceURL string) (*provider.HotelDetails, error) {
	var pd pageDetail
	if err := json.Unmarshal(data, &pd); err != nil {
		return nil, fmt.Errorf("Tatildukkani sayfa verisi parse edilemedi: %w", err)
	}
	if pd.Type != "HOTEL" || pd.TypeID <= 0 {
		return nil, fmt.Errorf("Tatildukkani URL'i otel detay sayfasi degil (tip: %q)", pd.Type)
	}
	r := &pd.Result
	name := provider.DecodeEntities(r.Name)
	if name == "" {
		name = provider.DecodeEntities(pd.Title)
	}
	out := &provider.HotelDetails{
		Provider:        "tatildukkani",
		ProviderHotelID: strconv.Itoa(pd.TypeID),
		SourceURL:       sourceURL,
		Name:            name,
		Address:         provider.DecodeEntities(r.Address),
		Images:          []provider.Image{},
		Facilities:      []provider.FacilityGroup{},
		Rooms:           []provider.RoomInfo{},
		Boards:          []string{},
		FAQ:             []provider.QA{},
		Sections:        []provider.Section{},
	}
	if n, err := strconv.Atoi(strings.TrimSpace(r.Star)); err == nil && n > 0 {
		out.Stars = &n
	}
	if r.HotelDestination != nil {
		out.City = provider.DecodeEntities(r.HotelDestination.Name)
	}
	if len(r.Destination) > 0 {
		out.Country = provider.DecodeEntities(r.Destination[0].Name)
		if len(r.Destination) > 1 {
			if reg := provider.DecodeEntities(r.Destination[len(r.Destination)-1].Name); reg != out.City {
				out.Region = reg
			}
		}
	}
	if lat, e1 := strconv.ParseFloat(strings.TrimSpace(r.Latitude), 64); e1 == nil {
		if lng, e2 := strconv.ParseFloat(strings.TrimSpace(r.Longitude), 64); e2 == nil && (lat != 0 || lng != 0) {
			out.Location = &provider.GeoPoint{Lat: lat, Lng: lng}
		}
	}
	if r.Reviews != nil && r.Reviews.TotalPoint > 0 {
		v := math.Round(r.Reviews.TotalPoint*2*10) / 10 // 5 uzerinden -> 10
		out.Rating = &v
		if r.Reviews.TotalComment > 0 {
			c := r.Reviews.TotalComment
			out.RatingCount = &c
		}
	}

	// Gorseller: kapak (singleImage) once, sonra tur/line/id sirasiyla.
	imgs := append([]pageImage(nil), r.Images...)
	sort.SliceStable(imgs, func(i, j int) bool {
		ti, tj := 0, 0
		if imgs[i].HotelType != nil {
			ti = *imgs[i].HotelType
		}
		if imgs[j].HotelType != nil {
			tj = *imgs[j].HotelType
		}
		if ti != tj {
			return ti < tj
		}
		if imgs[i].Line != imgs[j].Line {
			return imgs[i].Line < imgs[j].Line
		}
		return imgs[i].ID < imgs[j].ID
	})
	if r.SingleImage != nil && r.SingleImage.FullPath != "" {
		imgs = append([]pageImage{*r.SingleImage}, imgs...)
	}
	seen := map[string]bool{}
	for _, im := range imgs {
		if im.Type != "" && im.Type != "image" {
			continue
		}
		u := reQuery.ReplaceAllString(strings.TrimSpace(im.FullPath), "")
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		cap := ""
		if im.HotelType != nil {
			cap = r.ImageHotelTypes[strconv.Itoa(*im.HotelType)]
		}
		out.Images = append(out.Images, provider.Image{URL: u, Caption: cap})
	}

	// Ozellik gruplari + bolumler (ayni sirayla; zengin metinler kendi basliklariyla ayri bolum).
	groups := sortedGroups(r.Facality)
	var generalInfo, warnings, childInfo string
	for _, g := range groups {
		title := provider.DecodeEntities(g.Name)
		items := []string{}
		for _, it := range g.Facality {
			if l := facLabel(it); l != "" {
				items = append(items, l)
			}
		}
		if len(items) > 0 {
			out.Facilities = append(out.Facilities, provider.FacilityGroup{Title: title, Items: items})
			out.Sections = append(out.Sections, provider.Section{Title: title, Kind: "list", Items: items})
		}
		for _, it := range g.Facality {
			if it.InputRichtext == nil {
				continue
			}
			iname := provider.DecodeEntities(it.Name)
			stitle := iname
			if stitle == title && len(items) > 0 { // grup listesiyle ayni baslik olmasin (orn. "Konum Bilgisi")
				stitle += " (Açıklama)"
			}
			if s, ok := sectionFromHTML(stitle, *it.InputRichtext); ok {
				out.Sections = append(out.Sections, s)
			}
			switch strings.ToLower(iname) {
			case "genel bilgi":
				generalInfo = *it.InputRichtext
			case "genel uyarılar", "genel uyarilar":
				warnings = *it.InputRichtext
			case "bebek ve çocuk bilgisi":
				childInfo = *it.InputRichtext
			}
		}
	}
	out.Description = htmlText(generalInfo)

	// Pansiyonlar.
	for _, c := range r.Concepts {
		if c = provider.DecodeEntities(c); c != "" {
			out.Boards = append(out.Boards, c)
		}
	}

	// Politikalar: "Genel Uyarilar" metninden giris/cikis saati, evcil hayvan; diger satirlar Other.
	pol := &provider.Policies{Other: []string{}}
	if warnings != "" {
		for _, ln := range htmlLines(warnings) {
			ln = strings.TrimSpace(strings.TrimLeft(ln, "*- "))
			if ln == "" {
				continue
			}
			low := strings.ToLower(ln)
			if m := reCheckIn.FindStringSubmatch(ln); m != nil && pol.CheckIn == "" {
				pol.CheckIn = strings.ReplaceAll(m[1], ".", ":")
			}
			if m := reCheckOut.FindStringSubmatch(ln); m != nil && pol.CheckOut == "" {
				pol.CheckOut = strings.ReplaceAll(m[1], ".", ":")
			}
			if strings.Contains(low, "evcil hayvan") && pol.PetPolicy == "" {
				pol.PetPolicy = sentenceWith(ln, "evcil hayvan")
			}
			pol.Other = append(pol.Other, ln)
		}
	}
	if t := htmlText(childInfo); t != "" {
		pol.ChildPolicy = t
	}
	if pol.CheckIn != "" || pol.CheckOut != "" || pol.PetPolicy != "" || pol.ChildPolicy != "" || len(pol.Other) > 0 {
		out.Policies = pol
	}

	// Etkinlikler (aktif gruplar; tarih + baslik sirali).
	events := []string{}
	for _, e := range r.EventGroups {
		if !e.Status || strings.TrimSpace(e.Title) == "" {
			continue
		}
		for _, d := range e.Date {
			events = append(events, d.Date+" — "+provider.DecodeEntities(e.Title))
		}
	}
	sort.Strings(events)
	if len(events) > 0 {
		out.Sections = append(out.Sections, provider.Section{Title: "Etkinlikler", Kind: "list", Items: events})
	}

	// Misafir degerlendirmeleri (yorum metinleri/isimler alinmaz).
	if r.Reviews != nil && (r.Reviews.TotalPoint > 0 || len(r.Reviews.Review) > 0) {
		pairs := []provider.KV{}
		if r.Reviews.TotalPoint > 0 {
			pairs = append(pairs, provider.KV{Key: "Genel Puan", Value: strconv.FormatFloat(r.Reviews.TotalPoint, 'f', -1, 64) + " / 5"})
		}
		if r.Reviews.TotalComment > 0 {
			pairs = append(pairs, provider.KV{Key: "Yorum Sayısı", Value: strconv.Itoa(r.Reviews.TotalComment)})
		}
		revs := append([]reviewPoint(nil), r.Reviews.Review...)
		sort.SliceStable(revs, func(i, j int) bool { return revs[i].Name < revs[j].Name })
		for _, rv := range revs {
			pairs = append(pairs, provider.KV{Key: provider.DecodeEntities(rv.Name), Value: strconv.FormatFloat(rv.Point, 'f', -1, 64)})
		}
		out.Sections = append(out.Sections, provider.Section{Title: "Misafir Değerlendirmeleri", Kind: "pairs", Pairs: pairs})
	}
	return out, nil
}
