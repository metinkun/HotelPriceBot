package enuygun

// Statik otel detayi (`/otel/api-detail/<slug>.json`) ve isimle arama (autocomplete API'leri).
// Bkz. docs/providers/enuygun.md.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"pricebot/internal/httpx"
	"pricebot/internal/provider"
)

const (
	// autocompleteCoreURL sitenin yeni otel autocomplete servisi (hot-autocomplete-api).
	autocompleteCoreURL = "https://hot-autocomplete-api.enuygun.com/api/v1/autocomplete"
	// autocompleteLegacyURL eski uc (yedek): data.HOTEL[] listesi.
	autocompleteLegacyURL = base + "/otel/autocomplete"
)

// ---- api-detail tipleri (statik detay icin kullanilan alanlar) ----

// flexString string veya sayi gelebilen alanlar icin (nesne/dizi gelirse bos).
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		*f = flexString(s)
		return nil
	}
	var n json.Number
	if json.Unmarshal(b, &n) == nil {
		*f = flexString(n.String())
	}
	return nil
}

type enFacility struct {
	Details           string     `json:"details"`
	Value             flexString `json:"value"`
	IsPaymentRequired bool       `json:"isPaymentRequired"`
}

type enImage struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

type enRoom struct {
	Type struct {
		Name string `json:"name"`
	} `json:"type"`
	Images      []enImage    `json:"images"`
	Facilities  []enFacility `json:"facilities"`
	Description string       `json:"description"`
}

type enQA struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type enPOI struct {
	Name     string  `json:"name"`
	Distance float64 `json:"calculated_distance"`
}

type apiDetailFull struct {
	Data struct {
		Hotel struct {
			Details struct {
				ID      int    `json:"id"`
				Name    string `json:"name"`
				Address struct {
					Address string `json:"address"`
					Country string `json:"country"`
					Town    string `json:"town"`
					City    string `json:"city"`
				} `json:"address"`
				Coordinate struct {
					Latitude  float64 `json:"latitude"`
					Longitude float64 `json:"longitude"`
				} `json:"coordinate"`
				Facilities struct {
					HotelFacilities []struct {
						Type       string       `json:"type"`
						Facilities []enFacility `json:"facilities"`
					} `json:"hotelFacilities"`
				} `json:"facilities"`
				PopularFacilities []enFacility `json:"popularFacilities"`
				Suitabilities     []struct {
					Tag  string `json:"tag"`
					Name string `json:"name"`
				} `json:"suitabilities"`
				Images            []enImage `json:"images"`
				StarRating        int       `json:"starRating"`
				ReviewScore       float64   `json:"reviewScore"`
				ReviewsTotalCount int       `json:"reviewsTotalCount"`
				CheckInTime       string    `json:"checkInTime"`
				CheckOutTime      string    `json:"checkOutTime"`
				HotelConcepts     []struct {
					Concept     string  `json:"concept"`
					Description *string `json:"description"`
				} `json:"hotelConcepts"`
				ImportantInfo string `json:"importantInfo"`
				HotelPolicy   struct {
					Checkin struct {
						BeginTime string `json:"begin_time"`
						EndTime   string `json:"end_time"`
					} `json:"checkin"`
					Checkout            flexString `json:"checkout"`
					Instructions        string     `json:"instructions"`
					SpecialInstructions string     `json:"special_instructions"`
					CheckinMinAge       int        `json:"checkin_min_age"`
					KnowBeforeYouGo     string     `json:"know_before_you_go"`
				} `json:"hotelPolicy"`
				Faqs struct {
					Auto   json.RawMessage `json:"auto"`
					Manuel json.RawMessage `json:"manuel"`
				} `json:"faqs"`
			} `json:"details"`
			Rooms []enRoom `json:"rooms"`
		} `json:"hotel"`
		Region struct {
			Name string `json:"name"`
		} `json:"region"`
		HotelDescription string `json:"hotelDescription"`
		HotelPhones      []struct {
			PhoneNumber string `json:"phoneNumber"`
		} `json:"hotelPhones"`
		POI    map[string][]enPOI `json:"poi"`
		Events []struct {
			Title string `json:"title"`
		} `json:"events"`
	} `json:"data"`
}

func parseAPIDetailFull(body []byte) (*apiDetailFull, error) {
	var d apiDetailFull
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, fmt.Errorf("Enuygun api-detail yaniti parse edilemedi: %w", err)
	}
	if d.Data.Hotel.Details.ID == 0 || d.Data.Hotel.Details.Name == "" {
		return nil, fmt.Errorf("Enuygun otel detayi bos (otel detay sayfasi URL'i olmali)")
	}
	return &d, nil
}

func facilityLabel(f enFacility) string {
	s := oneLine(f.Details)
	if s == "" {
		return ""
	}
	if v := strings.TrimSpace(string(f.Value)); v != "" {
		s += ": " + v
	}
	if f.IsPaymentRequired && !strings.Contains(strings.ToLower(s), "ücretli") {
		s += " (Ücretli)"
	}
	return s
}

func parseQAs(raw json.RawMessage) []enQA {
	var qs []enQA
	if len(raw) == 0 || json.Unmarshal(raw, &qs) != nil {
		return nil
	}
	return qs
}

// normalizeDetails api-detail verisini ortak HotelDetails semasina cevirir.
func normalizeDetails(d *apiDetailFull, providerHotelID, sourceURL string) *provider.HotelDetails {
	h := &d.Data.Hotel.Details
	if providerHotelID == "" {
		providerHotelID = strconv.Itoa(h.ID)
	}
	out := &provider.HotelDetails{
		Provider: "enuygun", ProviderHotelID: providerHotelID, SourceURL: sourceURL,
		Name:        oneLine(h.Name),
		Description: htmlToText(d.Data.HotelDescription),
		Address:     oneLine(h.Address.Address),
		City:        strings.TrimSpace(h.Address.Town),
		Region:      strings.TrimSpace(d.Data.Region.Name),
		Country:     strings.TrimSpace(h.Address.Country),
		Images:      []provider.Image{},
		Facilities:  []provider.FacilityGroup{},
		Rooms:       []provider.RoomInfo{},
		Boards:      []string{},
		FAQ:         []provider.QA{},
		Sections:    []provider.Section{},
	}
	if out.City == "" {
		out.City = strings.TrimSpace(h.Address.City)
	}
	if h.StarRating > 0 {
		s := h.StarRating
		out.Stars = &s
	}
	if h.ReviewScore > 0 {
		r := h.ReviewScore
		out.Rating = &r
	}
	if h.ReviewsTotalCount > 0 {
		c := h.ReviewsTotalCount
		out.RatingCount = &c
	}
	if h.Coordinate.Latitude != 0 || h.Coordinate.Longitude != 0 {
		out.Location = &provider.GeoPoint{Lat: h.Coordinate.Latitude, Lng: h.Coordinate.Longitude}
	}
	for _, ph := range d.Data.HotelPhones {
		if p := strings.TrimSpace(ph.PhoneNumber); p != "" {
			out.Contact = &provider.Contact{Phone: p}
			break
		}
	}

	seen := map[string]bool{}
	for _, im := range h.Images {
		u := cleanImageURL(im.URL)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out.Images = append(out.Images, provider.Image{URL: u, Caption: oneLine(im.Name)})
	}

	for _, g := range h.Facilities.HotelFacilities {
		items := []string{}
		for _, f := range g.Facilities {
			items = append(items, facilityLabel(f))
		}
		if items = uniqStrings(items); len(items) > 0 {
			out.Facilities = append(out.Facilities, provider.FacilityGroup{Title: oneLine(g.Type), Items: items})
		}
	}

	// Odalar: ayni ad (bosluk farki) tekrar ediyor; ilk kayit alinir, gorseller birlestirilir.
	roomIdx := map[string]int{}
	for _, r := range d.Data.Hotel.Rooms {
		name := oneLine(r.Type.Name)
		if name == "" {
			continue
		}
		imgs := []string{}
		for _, im := range r.Images {
			if u := cleanImageURL(im.URL); u != "" {
				imgs = append(imgs, u)
			}
		}
		if i, ok := roomIdx[name]; ok {
			out.Rooms[i].Images = uniqStrings(append(out.Rooms[i].Images, imgs...))
			continue
		}
		ri := provider.RoomInfo{Name: name, Description: htmlToText(r.Description), Features: []string{}, Images: uniqStrings(imgs)}
		for _, f := range r.Facilities {
			if strings.HasPrefix(strings.ToLower(f.Details), "oda büyüklüğü") {
				if n, err := strconv.Atoi(strings.TrimSpace(string(f.Value))); err == nil && n > 0 {
					ri.Size = &n
				}
				continue
			}
			ri.Features = append(ri.Features, facilityLabel(f))
		}
		ri.Features = uniqStrings(ri.Features)
		roomIdx[name] = len(out.Rooms)
		out.Rooms = append(out.Rooms, ri)
	}

	concepts := []provider.KV{}
	for _, c := range h.HotelConcepts {
		name := oneLine(c.Concept)
		if name == "" {
			continue
		}
		out.Boards = append(out.Boards, name)
		desc := ""
		if c.Description != nil {
			desc = htmlToText(*c.Description)
		}
		concepts = append(concepts, provider.KV{Key: name, Value: desc})
	}
	out.Boards = uniqStrings(out.Boards)

	pol := &provider.Policies{CheckOut: strings.TrimSpace(string(h.HotelPolicy.Checkout))}
	if b := strings.TrimSpace(h.HotelPolicy.Checkin.BeginTime); b != "" {
		pol.CheckIn = b
		if e := strings.TrimSpace(h.HotelPolicy.Checkin.EndTime); e != "" && e != b {
			pol.CheckIn = b + " - " + e
		}
	}
	if pol.CheckIn == "" {
		pol.CheckIn = strings.TrimSpace(h.CheckInTime)
	}
	if pol.CheckOut == "" {
		pol.CheckOut = strings.TrimSpace(h.CheckOutTime)
	}
	if h.HotelPolicy.CheckinMinAge > 0 {
		pol.Other = append(pol.Other, fmt.Sprintf("Giriş için en az yaş: %d", h.HotelPolicy.CheckinMinAge))
	}
	if s := oneLine(h.HotelPolicy.SpecialInstructions); s != "" {
		pol.Other = append(pol.Other, s)
	}
	out.Policies = pol

	// SSS: fiyat verisinden uretilen ("Verilerimize göre ... Son güncelleme") cevaplar her gun degisir, atlanir.
	for _, qa := range append(parseQAs(h.Faqs.Manuel), parseQAs(h.Faqs.Auto)...) {
		q, a := oneLine(qa.Question), htmlToText(qa.Answer)
		if q == "" || a == "" || strings.Contains(a, "Verilerimize göre") {
			continue
		}
		out.FAQ = append(out.FAQ, provider.QA{Question: q, Answer: a})
	}

	// Sections
	out.Sections = appendSection(out.Sections,
		htmlSection("Otel Hakkında", d.Data.HotelDescription),
		htmlSection("Önemli Bilgiler", h.ImportantInfo),
		htmlSection("Bilmeniz Gerekenler", h.HotelPolicy.KnowBeforeYouGo),
		htmlSection("Otel Politikaları", h.HotelPolicy.Instructions),
		pairsSection("Konaklama Konseptleri", concepts),
	)
	pop := []string{}
	for _, f := range h.PopularFacilities {
		pop = append(pop, facilityLabel(f))
	}
	tags := []string{}
	for _, s := range h.Suitabilities {
		t := oneLine(s.Tag)
		if t == "" {
			t = oneLine(s.Name)
		}
		tags = append(tags, t)
	}
	out.Sections = appendSection(out.Sections, listSection("Popüler Özellikler", pop), listSection("Otel Etiketleri", tags))
	keys := make([]string, 0, len(d.Data.POI))
	for k := range d.Data.POI {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	near := []provider.KV{}
	for _, k := range keys {
		for _, p := range d.Data.POI[k] {
			if n := oneLine(p.Name); n != "" && p.Distance > 0 {
				near = append(near, provider.KV{Key: n, Value: strconv.FormatFloat(p.Distance, 'f', 1, 64) + " km"})
			}
		}
	}
	events := []string{}
	for _, e := range d.Data.Events {
		events = append(events, oneLine(e.Title))
	}
	out.Sections = appendSection(out.Sections, pairsSection("Yakın Çevre", near), listSection("Etkinlikler", events))
	return out
}

// fetchAPIDetail api-detail JSON'unu ham olarak ceker.
func fetchAPIDetail(ctx context.Context, slug string) ([]byte, error) {
	q := url.Values{"slug": {slug}, "type": {"hotel"}}
	endpoint := fmt.Sprintf(apiDetailURL, url.PathEscape(slug)) + "?" + q.Encode()
	headers := httpx.Merge(apiHeaders, map[string]string{"referer": base + "/otel/detay/" + slug + "/"})
	resp, err := httpx.Do(ctx, http.MethodGet, endpoint, httpx.Options{Headers: headers, Timeout: timeout})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// GetDetails api-detail verisini ceker ve ortak semaya cevirir (metadata.slug veya sourceUrl gerekir).
func (p *Provider) GetDetails(ctx context.Context, m *provider.Mapping) (*provider.HotelDetails, error) {
	slug := m.MetaString("slug")
	src := m.MetaString("sourceUrl")
	if slug == "" {
		slug = slugFromURL(src)
	}
	if slug == "" {
		return nil, fmt.Errorf("Mapping metadata'da slug/sourceUrl eksik")
	}
	body, err := fetchAPIDetail(ctx, slug)
	if err != nil {
		return nil, err
	}
	d, err := parseAPIDetailFull(body)
	if err != nil {
		return nil, err
	}
	if src == "" {
		src = base + "/otel/detay/" + slug + "/"
	}
	out := normalizeDetails(d, m.ProviderHotelID, src)
	out.FetchedAt = time.Now().UTC()
	return out, nil
}

// ---- Isimle arama ----

type coreACResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Items []struct {
			Type string `json:"type"` // hotel | town | region | brand ...
			ID   int    `json:"id"`
			Name string `json:"name"`
			Path string `json:"path"` // "Girne, Kıbrıs"
			Slug string `json:"slug"`
			URL  string `json:"url"` // "/otel/detay/<slug>/"
		} `json:"items"`
	} `json:"data"`
}

// parseCoreAutocomplete hot-autocomplete-api yanitindan otelleri ayiklar.
func parseCoreAutocomplete(body []byte) ([]provider.HotelCandidate, error) {
	var r coreACResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("Enuygun autocomplete yaniti parse edilemedi: %w", err)
	}
	if !r.Success {
		return nil, fmt.Errorf("Enuygun autocomplete success=false")
	}
	out := []provider.HotelCandidate{}
	for _, it := range r.Data.Items {
		if it.Type != "hotel" || (it.URL == "" && it.Slug == "") {
			continue
		}
		u := base + "/otel/detay/" + it.Slug + "/"
		if it.URL != "" {
			u = base + "/" + strings.TrimPrefix(it.URL, "/")
		}
		id := ""
		if it.ID > 0 {
			id = strconv.Itoa(it.ID)
		}
		out = append(out, provider.HotelCandidate{Name: oneLine(it.Name), URL: u, City: oneLine(it.Path), ProviderHotelID: id})
	}
	return out, nil
}

type legacyACResponse struct {
	Data struct {
		Hotel []struct {
			ID        flexString `json:"id"`
			ShortName string     `json:"short_name"`
			Name      string     `json:"name"`
			Slug      string     `json:"slug"`
			City      string     `json:"city"`
			Town      string     `json:"town"`
		} `json:"HOTEL"`
	} `json:"data"`
}

// parseLegacyAutocomplete /otel/autocomplete yanitindan (data.HOTEL) otelleri ayiklar.
func parseLegacyAutocomplete(body []byte) ([]provider.HotelCandidate, error) {
	var r legacyACResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("Enuygun autocomplete (eski uc) yaniti parse edilemedi: %w", err)
	}
	out := []provider.HotelCandidate{}
	for _, h := range r.Data.Hotel {
		if h.Slug == "" {
			continue
		}
		name := oneLine(h.ShortName)
		if name == "" {
			name = oneLine(h.Name)
		}
		city := strings.TrimSpace(h.Town)
		if c := strings.TrimSpace(h.City); c != "" && c != city {
			if city == "" {
				city = c
			} else {
				city += ", " + c
			}
		}
		out = append(out, provider.HotelCandidate{Name: name, URL: base + "/otel/detay/" + h.Slug + "/", City: city, ProviderHotelID: string(h.ID)})
	}
	return out, nil
}

// SearchHotels once hot-autocomplete-api, sonuc yoksa/hata ise eski /otel/autocomplete ucunu kullanir.
func (p *Provider) SearchHotels(ctx context.Context, query string) ([]provider.HotelCandidate, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return []provider.HotelCandidate{}, nil
	}
	hdr := httpx.Merge(apiHeaders, map[string]string{"referer": base + "/otel/"})
	delete(hdr, "content-type")
	coreURL := autocompleteCoreURL + "?" + url.Values{"q": {q}, "limit": {"20"}}.Encode()
	var firstErr error
	if resp, err := httpx.Get(ctx, coreURL, hdr, 20*time.Second); err == nil {
		cands, perr := parseCoreAutocomplete(resp.Body)
		if perr == nil && len(cands) > 0 {
			return cands, nil
		}
		firstErr = perr
	} else {
		firstErr = err
	}
	legacyURL := autocompleteLegacyURL + "?" + url.Values{"term": {q}, "locale": {"tr-tr"}}.Encode()
	resp, err := httpx.Get(ctx, legacyURL, hdr, 20*time.Second)
	if err != nil {
		if firstErr != nil {
			return nil, fmt.Errorf("%v; eski uc: %w", firstErr, err)
		}
		return nil, err
	}
	return parseLegacyAutocomplete(resp.Body)
}
