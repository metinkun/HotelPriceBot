package gezinomi

// Statik otel detayi (detay sayfasi RSC payload'indaki `hotelDetail`) ve isimle arama
// (`Hotel/SearchAutoComplete`). Bkz. docs/providers/gezinomi.md.

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

const (
	pathAutoComplete = "Hotel/SearchAutoComplete"
	// imgBase: `pictures[].name` uzantisiz dosya adidir; site bunu bu CDN kalibiyla gosterir.
	imgBase = "https://images.gezinomi.com/fit-in/1600x900/filters:quality(100)/filters:format(webp)/assets/"
)

// ---- hotelDetail tipleri (sadece kullanilan alanlar) ----

type gzPicture struct {
	Name              string  `json:"name"`
	CategoryName      string  `json:"pictureCategoryName"`
	IsCampaignPicture bool    `json:"isCampaignPicture"`
	ExternalImageURL  *string `json:"externalImageUrl"`
}

type gzDescription struct {
	TypeID   int     `json:"descriptionTypeId"`
	TypeName string  `json:"descriptionTypeName"`
	Text     *string `json:"text"` // HTML veya "$<id>" referansi
}

type gzProperty struct {
	Name        string   `json:"name"`
	GroupName   string   `json:"hotelPropertyGroupName"`
	IsThereAFee bool     `json:"isThereAFee"`
	Description *string  `json:"description"`
	Value       *float64 `json:"value"`
	UnitID      *int     `json:"unitId"`
}

type gzRoom struct {
	RoomID      int         `json:"hotelRoomId"`
	RoomName    string      `json:"roomName"`
	Description string      `json:"description"`
	Capacity    int         `json:"capacity"`
	AdultCount  int         `json:"adultCount"`
	Pictures    []gzPicture `json:"pictures"`
	Properties  []struct {
		Name string `json:"hotelRoomPropertyName"`
	} `json:"hotelRoomProperties"`
}

type gzDetail struct {
	HotelID        int             `json:"hotelId"`
	HotelName      string          `json:"hotelName"`
	ProductName    string          `json:"productName"`
	NavigationName string          `json:"navigationName"` // "Kıbrıs/Girne"
	HotelLocation  string          `json:"hotelLocation"`
	Latitude       string          `json:"latitude"`
	Longitude      string          `json:"longitude"`
	MetaKeyword    string          `json:"metaKeyword"`
	Pictures       []gzPicture     `json:"pictures"`
	Descriptions   []gzDescription `json:"descriptions"`
	PropertyList   []gzProperty    `json:"propertyList"`
	RatingGroups   []struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	} `json:"commentsRatignGroup"`
	RatingTotal *struct {
		AvgRate float64 `json:"avgRate"`
	} `json:"commentsRatingTotal"`
	Categories []struct {
		Name string `json:"hotelCategoryName"`
	} `json:"hotelCategories"`
	Prominent []struct {
		Description string `json:"description"`
	} `json:"hotelPropertyProminentCacheModels"`
	Rooms      []gzRoom `json:"hotelRoomCacheModels"`
	ChildDesc  string   `json:"childDesc"`
	Child1Min  int      `json:"childAge1Min"`
	Child1Max  int      `json:"childAge1Max"`
	Child2Min  int      `json:"childAge2Min"`
	Child2Max  int      `json:"childAge2Max"`
	AdultHotel bool     `json:"isAdultHotel"`
	Activities []struct {
		Title string `json:"title"`
	} `json:"hotelActivities"`
	CancelDescription string `json:"cancelDescription"`
}

var reImageExt = regexp.MustCompile(`(?i)\.(jpe?g|png|webp|gif)$`)

// pictureURL gorsel adini tam CDN URL'ine cevirir (uzantisiz adlara "-b0.jpg" eklenir).
func pictureURL(p gzPicture) string {
	if p.ExternalImageURL != nil && strings.TrimSpace(*p.ExternalImageURL) != "" {
		return cleanImageURL(*p.ExternalImageURL)
	}
	n := strings.TrimSpace(p.Name)
	if n == "" {
		return ""
	}
	if reImageExt.MatchString(n) {
		return imgBase + n
	}
	return imgBase + n + "-b0.jpg"
}

// parseHotelDetail sayfa HTML'inden hotelDetail'i cozer ve "$<id>" metin referanslarini yerine koyar.
func parseHotelDetail(page string) (*gzDetail, error) {
	payload := rscPayload(page)
	if payload == "" {
		return nil, fmt.Errorf("Gezinomi sayfasinda RSC payload bulunamadi")
	}
	var d gzDetail
	if err := rscExtract(payload, `"hotelDetail":`, &d); err != nil {
		return nil, fmt.Errorf("Gezinomi detay verisi: %w", err)
	}
	if d.HotelID == 0 || d.HotelName == "" {
		return nil, fmt.Errorf("Gezinomi detay verisi bos (otel detay sayfasi URL'i olmali)")
	}
	texts := rscTexts(payload)
	for i := range d.Descriptions {
		if t := d.Descriptions[i].Text; t != nil {
			r := rscRef(*t, texts)
			d.Descriptions[i].Text = &r
		}
	}
	return &d, nil
}

var unitNames = map[int]string{6: "m", 7: "km", 13: "m²"}

// propertyLabel "Hava Alanına Uzaklığı: 30 km", "Şehir Merkezine Uzaklığı: 8 km (Girne)", "Bar (Ücretli)".
func propertyLabel(p gzProperty) string {
	s := oneLine(p.Name)
	if s == "" {
		return ""
	}
	if p.Value != nil && *p.Value != 0 {
		s += ": " + strconv.FormatFloat(*p.Value, 'f', -1, 64)
		if p.UnitID != nil {
			if u, ok := unitNames[*p.UnitID]; ok {
				s += " " + u
			}
		}
	}
	if p.Description != nil {
		if dsc := oneLine(*p.Description); dsc != "" {
			s += " (" + dsc + ")"
		}
	}
	if p.IsThereAFee {
		s += " (Ücretli)"
	}
	return s
}

var (
	reRoomSize = regexp.MustCompile(`(\d+)(?:\s*-\s*\d+)?\s*m2`)
	reStars    = regexp.MustCompile(`(\d)\s*Yıldız`)
	reCheckIn  = regexp.MustCompile(`(?i)giriş saati\s*(\d{1,2}[:.]\d{2})`)
	reCheckOut = regexp.MustCompile(`(?i)çıkış saati(?:\s+ise)?(?:\s+en geç)?\s*(\d{1,2}[:.]\d{2})`)
)

// normalizeDetails Gezinomi verisini ortak HotelDetails semasina cevirir.
func normalizeDetails(d *gzDetail, providerHotelID, sourceURL string) *provider.HotelDetails {
	if providerHotelID == "" {
		providerHotelID = strconv.Itoa(d.HotelID)
	}
	out := &provider.HotelDetails{
		Provider: "gezinomi", ProviderHotelID: providerHotelID, SourceURL: sourceURL,
		Name:       oneLine(d.HotelName),
		Images:     []provider.Image{},
		Facilities: []provider.FacilityGroup{},
		Rooms:      []provider.RoomInfo{},
		FAQ:        []provider.QA{},
		Sections:   []provider.Section{},
	}
	if parts := strings.Split(d.NavigationName, "/"); len(parts) == 2 {
		out.Region, out.City = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	} else {
		out.City = strings.TrimSpace(d.NavigationName)
	}
	if strings.EqualFold(strings.TrimSpace(d.HotelLocation), "Kıbrıs") {
		out.Country = "Kıbrıs"
	}
	if m := reStars.FindStringSubmatch(d.MetaKeyword); m != nil {
		if n, _ := strconv.Atoi(m[1]); n > 0 {
			out.Stars = &n
		}
	}
	if d.RatingTotal != nil && d.RatingTotal.AvgRate > 0 {
		r := float64(int(d.RatingTotal.AvgRate*100+0.5)) / 100
		out.Rating = &r
	}
	cnt := 0
	for _, g := range d.RatingGroups {
		cnt += g.Count
	}
	if cnt > 0 {
		out.RatingCount = &cnt
	}
	lat, _ := strconv.ParseFloat(strings.TrimSpace(d.Latitude), 64)
	lng, _ := strconv.ParseFloat(strings.TrimSpace(d.Longitude), 64)
	if lat != 0 || lng != 0 {
		out.Location = &provider.GeoPoint{Lat: lat, Lng: lng}
	}

	// Gorseller (kampanya gorselleri haric; oda gorselleri de bu listede)
	seen := map[string]bool{}
	for _, p := range d.Pictures {
		if p.IsCampaignPicture {
			continue
		}
		u := pictureURL(p)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out.Images = append(out.Images, provider.Image{URL: u, Caption: oneLine(p.CategoryName)})
	}

	// Ozellik gruplari (ilk gorulme sirasiyla)
	groupIdx := map[string]int{}
	for _, p := range d.PropertyList {
		title := oneLine(p.GroupName)
		label := propertyLabel(p)
		if title == "" || label == "" {
			continue
		}
		i, ok := groupIdx[title]
		if !ok {
			i = len(out.Facilities)
			groupIdx[title] = i
			out.Facilities = append(out.Facilities, provider.FacilityGroup{Title: title, Items: []string{}})
		}
		out.Facilities[i].Items = append(out.Facilities[i].Items, label)
	}
	for i := range out.Facilities {
		out.Facilities[i].Items = uniqStrings(out.Facilities[i].Items)
	}

	// Odalar
	for _, r := range d.Rooms {
		name := oneLine(r.RoomName)
		if name == "" {
			continue
		}
		ri := provider.RoomInfo{Name: name, Description: oneLine(r.Description), Features: []string{}, Images: []string{}}
		if m := reRoomSize.FindStringSubmatch(r.Description); m != nil {
			if n, _ := strconv.Atoi(m[1]); n > 0 {
				ri.Size = &n
			}
		}
		if r.Capacity > 0 {
			ri.Capacity = fmt.Sprintf("%d kişi", r.Capacity)
			if r.AdultCount > 0 && r.AdultCount != r.Capacity {
				ri.Capacity += fmt.Sprintf(" (en fazla %d yetişkin)", r.AdultCount)
			}
		}
		for _, p := range r.Properties {
			ri.Features = append(ri.Features, oneLine(p.Name))
		}
		ri.Features = uniqStrings(ri.Features)
		for _, p := range r.Pictures {
			if u := pictureURL(p); u != "" {
				ri.Images = append(ri.Images, u)
			}
		}
		ri.Images = uniqStrings(ri.Images)
		out.Rooms = append(out.Rooms, ri)
	}

	// Aciklamalar -> Sections; politikalar aciklamalardan
	pol := &provider.Policies{Cancellation: oneLine(d.CancelDescription)}
	allText := []string{}
	for _, ds := range d.Descriptions {
		if ds.Text == nil {
			continue
		}
		title := oneLine(ds.TypeName)
		sec := htmlSection(title, *ds.Text)
		if sec == nil {
			continue
		}
		out.Sections = append(out.Sections, *sec)
		plain := htmlToText(*ds.Text)
		allText = append(allText, plain)
		if strings.Contains(strings.ToLower(title), "evcil") && pol.PetPolicy == "" {
			pol.PetPolicy = oneLine(*ds.Text)
		}
	}
	joined := strings.Join(allText, "\n")
	if m := reCheckIn.FindStringSubmatch(joined); m != nil {
		pol.CheckIn = strings.ReplaceAll(m[1], ".", ":")
	}
	if m := reCheckOut.FindStringSubmatch(joined); m != nil {
		pol.CheckOut = strings.ReplaceAll(m[1], ".", ":")
	}
	// Cocuk politikasi: kisa childDesc + ucretsiz yas araliklari ("Bebek/Çocuk" aciklamasi Sections'ta kalir).
	if cd := oneLine(d.ChildDesc); cd != "" || d.Child1Max > 0 {
		s := cd
		if d.Child1Max > 0 {
			s = strings.TrimSpace(s + fmt.Sprintf(" 1. çocuk %d-%d yaş", d.Child1Min, d.Child1Max))
			if d.Child2Max > 0 {
				s += fmt.Sprintf(", 2. çocuk %d-%d yaş", d.Child2Min, d.Child2Max)
			}
			s += "."
		}
		pol.ChildPolicy = s
	}
	if d.AdultHotel {
		pol.Other = append(pol.Other, "Sadece yetişkin oteli")
	}
	out.Policies = pol
	if len(out.Sections) > 0 {
		out.Description = out.Sections[0].Text
	}

	prominent := []string{}
	for _, p := range d.Prominent {
		prominent = append(prominent, oneLine(p.Description))
	}
	cats := []string{}
	for _, c := range d.Categories {
		cats = append(cats, oneLine(c.Name))
	}
	// Kampanyalar (hotelCampaignForAgaveRedis) saat/gune gore degisen indirim oranlari icerir; statik degil, atlanir.
	acts := []string{}
	for _, a := range d.Activities {
		acts = append(acts, oneLine(a.Title))
	}
	out.Sections = appendSection(out.Sections,
		listSection("Öne Çıkan Özellikler", prominent),
		listSection("Kategoriler", cats),
		listSection("Etkinlikler", acts),
	)
	return out
}

// GetDetails detay sayfasini (sourceUrl) ceker ve ortak semaya cevirir.
func (p *Provider) GetDetails(ctx context.Context, m *provider.Mapping) (*provider.HotelDetails, error) {
	src := m.MetaString("sourceUrl")
	if src == "" {
		return nil, fmt.Errorf("Mapping metadata'da sourceUrl eksik (Gezinomi detay sayfasi URL'i gerekli)")
	}
	resp, err := httpx.Do(ctx, http.MethodGet, src, httpx.Options{Headers: pageHeaders, Timeout: 45 * time.Second})
	if err != nil {
		return nil, err
	}
	d, err := parseHotelDetail(resp.Text())
	if err != nil {
		return nil, err
	}
	out := normalizeDetails(d, m.ProviderHotelID, src)
	out.FetchedAt = time.Now().UTC()
	return out, nil
}

// ---- Isimle arama ----

type acItem struct {
	ID         int    `json:"id"`
	Text       string `json:"text"`
	URL        string `json:"url"`
	Location   string `json:"location"`   // "Kıbrıs/Girne"
	ResultType string `json:"resultType"` // Otel | Otel Kategori | Bölge ...
	RouteLink  string `json:"routeLink"`  // "kibris/girne"
	RouteMain  string `json:"routeMain"`  // hotel-detail | hotel-list
}

type acResponse struct {
	Data    json.RawMessage `json:"data"`
	Status  string          `json:"status"`
	Message string          `json:"message"`
}

// parseAutoComplete yanittaki otel kayitlarini aday listesine cevirir (data duz veya ic ice dizi olabilir).
func parseAutoComplete(body []byte) ([]provider.HotelCandidate, error) {
	var r acResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("Gezinomi arama yaniti parse edilemedi: %w", err)
	}
	if r.Status != "Success" {
		if strings.Contains(r.Message, "en az 3 karakter") {
			return []provider.HotelCandidate{}, nil
		}
		return nil, fmt.Errorf("Gezinomi arama hatasi: %s", strings.TrimSpace(r.Status+" "+r.Message))
	}
	var items []acItem
	if err := json.Unmarshal(r.Data, &items); err != nil {
		var nested [][]acItem
		if err2 := json.Unmarshal(r.Data, &nested); err2 != nil {
			return nil, fmt.Errorf("Gezinomi arama verisi beklenmedik bicimde")
		}
		for _, n := range nested {
			items = append(items, n...)
		}
	}
	out := []provider.HotelCandidate{}
	for _, it := range items {
		if it.ResultType != "Otel" || it.RouteMain != "hotel-detail" || it.URL == "" {
			continue
		}
		slug := strings.Trim(it.URL, "/")
		route := strings.Trim(it.RouteLink, "/")
		u := site + "/otel/" + slug
		if route != "" {
			u = site + "/otel/" + route + "/" + slug
		}
		city := strings.TrimSpace(it.Location)
		if i := strings.LastIndex(city, "/"); i >= 0 {
			city = strings.TrimSpace(city[i+1:])
		}
		out = append(out, provider.HotelCandidate{Name: oneLine(it.Text), URL: u, City: city})
	}
	return out, nil
}

// SearchHotels otel adina gore SearchAutoComplete sonuclarindan otelleri doner. Uc imzasiz calisir;
// "Invalid" donerse (anahtar zorunlu hale gelirse) oturum/anahtar ile bir kez daha denenir.
// Not: yanitta mapping'de kullanilan hotelId yoktur; ProviderHotelID bos kalir, URL cozumlemesi doldurur.
func (p *Provider) SearchHotels(ctx context.Context, query string) ([]provider.HotelCandidate, error) {
	q := strings.TrimSpace(query)
	if len([]rune(q)) < 3 {
		return []provider.HotelCandidate{}, nil
	}
	body, _ := json.Marshal(map[string]string{"query": q})
	resp, err := httpx.Post(ctx, api+"/"+pathAutoComplete, body, apiHeaders, 25*time.Second)
	if err != nil {
		return nil, err
	}
	if isInvalid(resp.Body) && p.signer != nil {
		if r2, err2 := p.signer.post(ctx, pathAutoComplete, body, 25*time.Second); err2 == nil {
			resp = r2
		}
	}
	return parseAutoComplete(resp.Body)
}
