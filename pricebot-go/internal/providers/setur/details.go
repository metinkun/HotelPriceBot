package setur

// Statik otel detayi (detay sayfasi RSC payload'indaki `initialData.data`) ve isimle arama
// (`SearchesService/searchesHotel`). Bkz. docs/providers/setur.md.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"pricebot/internal/httpx"
	"pricebot/internal/provider"
)

const searchHotelURL = base + "/api/services/v4/SearchesService/searchesHotel"

// ---- RSC icindeki initialData.data tipleri (sadece kullanilan alanlar) ----

type seturFeature struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Main    bool   `json:"main"`
	Charged bool   `json:"charged"`
}

type seturCategory struct {
	ID          int            `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"` // HTML veya "$<id>" referansi
	Features    []seturFeature `json:"features"`
}

type seturImage struct {
	URL          string `json:"url"`
	CategoryName string `json:"hotelImageCategoryName"`
}

type seturRoom struct {
	ID          int          `json:"id"`
	Name        string       `json:"name"`
	MinAreaSize int          `json:"minAreaSize"`
	MaxAreaSize int          `json:"maxAreaSize"`
	Images      []seturImage `json:"images"`
}

type seturEvent struct {
	Title      string `json:"title"`
	ArtistName string `json:"artistName"`
}

type seturDistance struct {
	Name     string  `json:"name"`
	Distance float64 `json:"distance"`
}

type seturDetail struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	Address           string `json:"address"`
	City              string `json:"city"`
	Country           string `json:"country"`
	Region            string `json:"region"`
	Latitude          float64
	Longitude         float64
	StarCount         int    `json:"starCount"`
	CategoryName      string `json:"categoryName"`
	CheckinBeginTime  string `json:"checkinBeginTime"`
	CheckoutTime      string `json:"checkoutTime"`
	GeneralConditions string `json:"generalConditions"` // HTML veya "$<id>"
	SeasonFeatures    []struct {
		Name       string          `json:"name"`
		Categories []seturCategory `json:"categories"`
	} `json:"seasonFeatures"`
	MainFacilities []seturFeature  `json:"mainFacilities"`
	Images         []seturImage    `json:"images"`
	Rooms          []seturRoom     `json:"rooms"`
	Events         []seturEvent    `json:"events"`
	DistanceList   []seturDistance `json:"distanceList"`
}

var (
	reListingData = regexp.MustCompile(`"initialData":\s*\{\s*"data":\s*\[`)
	reNullData    = regexp.MustCompile(`"initialData":\s*\{\s*"data":\s*null`)
)

// parseDetailPage sayfa HTML'inden initialData.data'yi cozer ve "$<id>" metin referanslarini yerine koyar.
func parseDetailPage(page string) (*seturDetail, error) {
	payload := rscPayload(page)
	if payload == "" {
		return nil, fmt.Errorf("Setur sayfasinda RSC payload bulunamadi")
	}
	// Yayinda olmayan otel: sayfa liste sayfasina (girne-otelleri) yonlenir -> initialData bir dizi olur;
	// veya data:null,isError:true gelir. Ikisi de "satista degil" demektir, parse hatasi degil.
	if reListingData.MatchString(payload) {
		return nil, fmt.Errorf("Setur'da otel yayinda degil (sayfa otel listesine yonlendiriliyor)")
	}
	if reNullData.MatchString(payload) {
		return nil, fmt.Errorf("Setur'da otel yayinda degil (detay verisi yok)")
	}
	var wrap struct {
		Data *seturDetail `json:"data"`
	}
	if err := rscExtract(payload, `"initialData":`, &wrap); err != nil {
		return nil, fmt.Errorf("Setur detay verisi: %w", err)
	}
	if wrap.Data == nil || wrap.Data.Name == "" {
		return nil, fmt.Errorf("Setur detay verisi bos (otel detay sayfasi URL'i olmali)")
	}
	texts := rscTexts(payload)
	d := wrap.Data
	d.GeneralConditions = rscRef(d.GeneralConditions, texts)
	for i := range d.SeasonFeatures {
		for j := range d.SeasonFeatures[i].Categories {
			c := &d.SeasonFeatures[i].Categories[j]
			c.Description = rscRef(c.Description, texts)
		}
	}
	return d, nil
}

func featureName(f seturFeature) string {
	n := strings.TrimSpace(f.Name)
	if n != "" && f.Charged {
		n += " (Ücretli)"
	}
	return n
}

// normalizeDetails Setur verisini ortak HotelDetails semasina cevirir.
func normalizeDetails(d *seturDetail, providerHotelID, sourceURL string) *provider.HotelDetails {
	out := &provider.HotelDetails{
		Provider: "setur", ProviderHotelID: providerHotelID, SourceURL: sourceURL,
		Name:        oneLine(d.Name),
		Description: htmlToText(d.GeneralConditions),
		Address:     oneLine(d.Address),
		City:        strings.TrimSpace(d.City),
		Region:      strings.TrimSpace(d.Region),
		Country:     strings.TrimSpace(d.Country),
		Images:      []provider.Image{},
		Facilities:  []provider.FacilityGroup{},
		Rooms:       []provider.RoomInfo{},
		FAQ:         []provider.QA{},
		Sections:    []provider.Section{},
	}
	if providerHotelID == "" && d.ID > 0 {
		out.ProviderHotelID = strconv.Itoa(d.ID)
	}
	if d.StarCount > 0 {
		s := d.StarCount
		out.Stars = &s
	}
	if d.Latitude != 0 || d.Longitude != 0 {
		out.Location = &provider.GeoPoint{Lat: d.Latitude, Lng: d.Longitude}
	}

	// Gorseller (kategori adi aciklama olarak)
	seenImg := map[string]bool{}
	for _, im := range d.Images {
		u := cleanImageURL(im.URL)
		if u == "" || seenImg[u] {
			continue
		}
		seenImg[u] = true
		out.Images = append(out.Images, provider.Image{URL: u, Caption: strings.TrimSpace(im.CategoryName)})
	}

	// Ozellik gruplari + aciklamali kategoriler (Sections)
	pol := &provider.Policies{CheckIn: strings.TrimSpace(d.CheckinBeginTime), CheckOut: strings.TrimSpace(d.CheckoutTime)}
	var catSections []provider.Section
	for _, sf := range d.SeasonFeatures {
		for _, c := range sf.Categories {
			title := strings.TrimSpace(c.Name)
			items := []string{}
			for _, f := range c.Features {
				if n := featureName(f); n != "" {
					items = append(items, n)
				}
			}
			if items = uniqStrings(items); len(items) > 0 {
				out.Facilities = append(out.Facilities, provider.FacilityGroup{Title: title, Items: items})
			}
			if sec := htmlSection(title, c.Description); sec != nil {
				catSections = append(catSections, *sec)
				if strings.Contains(strings.ToLower(title), "evcil") && pol.PetPolicy == "" {
					pol.PetPolicy = oneLine(c.Description)
				}
			}
		}
	}

	// Odalar: ayni adli kayitlar (farkli tedarikci) birlestirilir, gorseller tekillestirilir.
	roomIdx := map[string]int{}
	for _, r := range d.Rooms {
		name := oneLine(r.Name)
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
		ri := provider.RoomInfo{Name: name, Images: uniqStrings(imgs)}
		if r.MinAreaSize > 0 {
			sz := r.MinAreaSize
			ri.Size = &sz
		}
		roomIdx[name] = len(out.Rooms)
		out.Rooms = append(out.Rooms, ri)
	}

	if pol.CheckIn != "" || pol.CheckOut != "" || pol.PetPolicy != "" {
		out.Policies = pol
	}

	// Sections: acentenin basliklari
	out.Sections = appendSection(out.Sections, htmlSection("Genel Bilgiler", d.GeneralConditions))
	out.Sections = append(out.Sections, catSections...)
	mains := []string{}
	for _, f := range d.MainFacilities {
		mains = append(mains, featureName(f))
	}
	out.Sections = appendSection(out.Sections, listSection("Öne Çıkan Özellikler", mains))
	dist := []provider.KV{}
	for _, x := range d.DistanceList {
		if n := strings.TrimSpace(x.Name); n != "" && x.Distance > 0 {
			dist = append(dist, provider.KV{Key: n, Value: strconv.FormatFloat(x.Distance, 'f', 1, 64) + " km"})
		}
	}
	out.Sections = appendSection(out.Sections, pairsSection("Mesafeler", dist))
	events := []string{}
	for _, e := range d.Events {
		t := oneLine(e.Title)
		if a := oneLine(e.ArtistName); a != "" && !strings.Contains(strings.ToLower(t), strings.ToLower(a)) {
			t += " – " + a
		}
		events = append(events, t)
	}
	out.Sections = appendSection(out.Sections, listSection("Etkinlikler", events))
	return out
}

// GetDetails tarihsiz detay sayfasini ceker ve ortak semaya cevirir.
func (p *Provider) GetDetails(ctx context.Context, m *provider.Mapping) (*provider.HotelDetails, error) {
	slug := slugOf(m)
	if slug == "" {
		return nil, fmt.Errorf("Mapping metadata'da slug/sourceUrl eksik")
	}
	u := base + "/" + slug
	resp, err := httpx.Do(ctx, http.MethodGet, u, httpx.Options{Headers: pageHeaders, Timeout: pageTimeout})
	if err != nil {
		return nil, err
	}
	d, err := parseDetailPage(resp.Text())
	if err != nil {
		return nil, err
	}
	src := m.MetaString("sourceUrl")
	if src == "" {
		src = u
	}
	out := normalizeDetails(d, m.ProviderHotelID, src)
	out.FetchedAt = time.Now().UTC()
	return out, nil
}

// ---- Isimle arama ----

type searchHit struct {
	ID      any    `json:"id"`
	Text    string `json:"text"`
	Type    string `json:"type"` // Hotel | HotelCity | HotelChain | HotelRegion | HotelTheme
	URL     string `json:"url"`  // slug
	City    string `json:"city"`
	Country string `json:"country"`
}

func idString(v any) string {
	switch x := v.(type) {
	case float64:
		return strconv.FormatInt(int64(x), 10)
	case string:
		return x
	case json.Number:
		return x.String()
	}
	return ""
}

var reMultiWS = regexp.MustCompile(`\s+`)

// parseSearchHits searchesHotel yanitindan yalnizca otel kayitlarini aday listesine cevirir.
func parseSearchHits(body []byte) ([]provider.HotelCandidate, error) {
	var hits []searchHit
	if err := json.Unmarshal(body, &hits); err != nil {
		return nil, fmt.Errorf("Setur arama yaniti parse edilemedi: %w", err)
	}
	out := []provider.HotelCandidate{}
	for _, h := range hits {
		if h.Type != "Hotel" || h.URL == "" {
			continue
		}
		city := strings.TrimSpace(h.City)
		if c := strings.TrimSpace(h.Country); c != "" && city != "" {
			city += ", " + c
		} else if c != "" {
			city = c
		}
		out = append(out, provider.HotelCandidate{
			Name:            strings.TrimSpace(reMultiWS.ReplaceAllString(h.Text, " ")),
			URL:             base + "/" + strings.TrimPrefix(h.URL, "/"),
			City:            city,
			ProviderHotelID: idString(h.ID),
		})
	}
	return out, nil
}

// SearchHotels otel adina gore Setur autocomplete (searchesHotel) sonuclarindan otelleri doner.
func (p *Provider) SearchHotels(ctx context.Context, query string) ([]provider.HotelCandidate, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return []provider.HotelCandidate{}, nil
	}
	body, _ := json.Marshal(map[string]any{"args": []map[string]string{{"s": q}}})
	resp, err := httpx.Post(ctx, searchHotelURL, body, map[string]string{
		"user-agent": ua, "content-type": "application/json", "accept": "application/json",
		"origin": base, "referer": base + "/",
	}, 25*time.Second)
	if err != nil {
		return nil, err
	}
	return parseSearchHits(resp.Body)
}
