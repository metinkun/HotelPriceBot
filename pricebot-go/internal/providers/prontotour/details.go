package prontotour

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"

	"pricebot/internal/provider"
)

// Statik detay: tarihsiz Hotel/HotelDetail yaniti otel kimligi + gorseller + ozellik listesi +
// konum/mesafe + (varsayilan arama) oda listesini verir. Sayfa JS'i (search-config.min.js
// app.HotelDetail.Detail) ile ayni kurallar uygulanir:
//   - Kategori "Genel Bilgiler"/"Genel Ozellikler": Type CONTENT -> "Otel Hakkinda" aciklamasi,
//     adinda "Saat"/"Hayvan" gecenler -> tesis kurallari; digerleri genel ozellik.
//   - Kategori "Konum Bilgileri": ad + deger ciftleri (LATITUDE/LONGITUDE gizli).
//   - Diger kategoriler: ozellik gruplari (IsPaid -> "(Ucretli)").

// Facility HotelDetailFacilityList elemani. IsActive/IsPaid API'de bazen bool bazen "True" string
// geldigi icin esnek tipte okunur.
type Facility struct {
	Name     string   `json:"Name"`
	Type     string   `json:"Type"` // YES | AVAILABLE | PAID | TEXT | CONTENT
	IsActive flexBool `json:"IsActive"`
	Category string   `json:"Category"`
	Value    *string  `json:"Value"`
	IsPaid   flexBool `json:"IsPaid"`
	Rank     *int     `json:"Rank"`
}

type flexBool bool

func (b *flexBool) UnmarshalJSON(data []byte) error {
	s := strings.Trim(strings.TrimSpace(string(data)), `"`)
	*b = flexBool(strings.EqualFold(s, "true") || s == "1")
	return nil
}

type detailImage struct {
	Original string `json:"Original"`
}

type facilityDistance struct {
	Name     string `json:"Name"`
	Distance string `json:"Distance"`
}

type roomFacility struct {
	Category string   `json:"Category"`
	Name     string   `json:"Name"`
	Value    *string  `json:"Value"`
	IsPaid   flexBool `json:"IsPaid"`
	Type     string   `json:"Type"`
}

type policyStruct struct {
	Title       string `json:"Title"`
	Description string `json:"Descripiton"` // API'deki yazim hatasi
}

type generalInformation struct {
	RoomsNumber   any `json:"RoomsNumber"`
	YearBuilt     any `json:"YearBuilt"`
	YearRenovated any `json:"YearRenovated"`
}

// hotelSummaryDetail HotelListResponse'un statik detay icin okunan ek alanlari.
type hotelSummaryDetail struct {
	Name         string  `json:"Name"`
	HotelCode    string  `json:"HotelCode"`
	Address      *string `json:"Address"`
	Star         *string `json:"Star"`
	ImgUrl       *string `json:"ImgUrl"`
	Phone        *string `json:"Phone"`
	Latitude     *string `json:"Latitude"`
	Longitude    *string `json:"Longitude"`
	Description  *string `json:"Description"`
	CheckinHours *string `json:"CheckinHours"`
	ChecoutHours *string `json:"ChecoutHours"`
	Pets         *string `json:"Pets"`
	Country      *string `json:"Country"`
	City         *string `json:"City"`
}

type roomDetailFull struct {
	RoomName          string         `json:"RoomName"`
	Mealy             string         `json:"Mealy"`
	RoomSize          any            `json:"RoomSize"`
	RoomInfo          *string        `json:"RoomInfo"`
	BedType           *string        `json:"BedType"`
	Image             *string        `json:"Image"`
	ImageList         []detailImage  `json:"ImageList"`
	AdultCount        int            `json:"AdultCount"`
	ChildCount        int            `json:"ChildCount"`
	TourLogicFacility []roomFacility `json:"TourLogicFacility"`
}

// HotelDetailFull tarihsiz HotelDetail yanitinin statik detay icin okunan hali.
type HotelDetailFull struct {
	HotelListResponse       *hotelSummaryDetail `json:"HotelListResponse"`
	HotelDetailImageList    []detailImage       `json:"HotelDetailImageList"`
	HotelDetailFacilityList []Facility          `json:"HotelDetailFacilityList"`
	FacilityDistance        []facilityDistance  `json:"FacilityDistance"`
	HotelRoomDetailsList    []roomDetailFull    `json:"HotelRoomDetailsList"`
	HotelServiceType        int                 `json:"HotelServiceType"`
	Opportunities           []string            `json:"Opportunities"`
	ListPolicyStruct        []policyStruct      `json:"ListPolicyStruct"`
	GeneralInformation      *generalInformation `json:"GeneralInformation"`
}

func fetchHotelDetailFull(ctx context.Context, code, serviceType string) (*HotelDetailFull, error) {
	req := hotelDetailRequest{
		Code: code, HotelServiceType: serviceType,
		Room:            []Room{{HotelTravelers: buildTravelers(2, 0, nil)}},
		CurrencyRequest: currencyTL,
		SourceType:      "H",
	}
	var out HotelDetailFull
	if err := postJSON(ctx, hotelDetailURL, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetDetails tarihsiz HotelDetail yanitini ortak semaya cevirir.
func (p *Provider) GetDetails(ctx context.Context, m *provider.Mapping) (*provider.HotelDetails, error) {
	st := serviceTypeOf(m)
	if st == "" {
		return nil, fmt.Errorf("Mapping metadata'da serviceType (veya -h<kod>-<tip> iceren sourceUrl) eksik")
	}
	d, err := fetchHotelDetailFull(ctx, m.ProviderHotelID, st)
	if err != nil {
		return nil, err
	}
	if d.HotelListResponse == nil || strings.TrimSpace(d.HotelListResponse.Name) == "" {
		return nil, fmt.Errorf("Prontotour otel bulunamadi (kod %s, tip %s)", m.ProviderHotelID, st)
	}
	out := normalizeDetails(d)
	out.ProviderHotelID = m.ProviderHotelID
	out.SourceURL = m.MetaString("sourceUrl")
	if out.SourceURL == "" {
		out.SourceURL = detailURL(d.HotelListResponse.Name, m.ProviderHotelID, st)
	}
	return out, nil
}

// cleanImageURL cache-buster parametrelerini temizler, kalan sorgu bos ise atar.
func cleanImageURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.RawQuery != "" {
		q := u.Query()
		for k := range q {
			switch strings.ToLower(k) {
			case "v", "ver", "version", "t", "ts", "_", "cb", "cache", "rnd", "r", "timestamp":
				q.Del(k)
			}
		}
		u.RawQuery = q.Encode()
	}
	return u.String()
}

func normalizeDetails(d *HotelDetailFull) *provider.HotelDetails {
	h := d.HotelListResponse
	out := &provider.HotelDetails{
		Provider:   "prontotour",
		Name:       strings.Join(strings.Fields(h.Name), " "),
		Images:     []provider.Image{},
		Facilities: []provider.FacilityGroup{},
		Rooms:      []provider.RoomInfo{},
		FAQ:        []provider.QA{},
		Sections:   []provider.Section{},
	}
	if h.Address != nil {
		out.Address = strings.TrimSpace(*h.Address)
	}
	if h.Star != nil {
		if n, err := strconv.Atoi(strings.TrimSpace(*h.Star)); err == nil && n > 0 {
			out.Stars = &n
		}
	}
	if lat, lng, ok := parseGeo(h.Latitude, h.Longitude); ok {
		out.Location = &provider.GeoPoint{Lat: lat, Lng: lng}
	}
	if h.Phone != nil && strings.TrimSpace(*h.Phone) != "" {
		out.Contact = &provider.Contact{Phone: strings.TrimSpace(*h.Phone)}
	}

	// Gorseller (sira API'den; ayni URL bir kez).
	seenImg := map[string]bool{}
	addImg := func(u string) {
		u = cleanImageURL(u)
		if u == "" || seenImg[u] {
			return
		}
		seenImg[u] = true
		out.Images = append(out.Images, provider.Image{URL: u})
	}
	for _, im := range d.HotelDetailImageList {
		addImg(im.Original)
	}
	if h.ImgUrl != nil {
		addImg(*h.ImgUrl)
	}

	// Politikalar
	pol := &provider.Policies{}
	if h.CheckinHours != nil && strings.TrimSpace(*h.CheckinHours) != "" {
		pol.CheckIn = strings.TrimSpace(*h.CheckinHours)
	}
	if h.ChecoutHours != nil && strings.TrimSpace(*h.ChecoutHours) != "" {
		pol.CheckOut = strings.TrimSpace(*h.ChecoutHours)
	}
	if h.Pets != nil && strings.TrimSpace(*h.Pets) != "" {
		pol.PetPolicy = strings.TrimSpace(*h.Pets)
	}

	// Ozellik listesi: kategori sirasi ilk gorulme sirasina gore (API sirasi sabit).
	type group struct {
		title string
		items []string
		pairs []provider.KV
	}
	var order []string
	groups := map[string]*group{}
	get := func(cat string) *group {
		if g, ok := groups[cat]; ok {
			return g
		}
		g := &group{title: cat}
		groups[cat] = g
		order = append(order, cat)
		return g
	}
	var rules []provider.KV
	var location []provider.KV
	description := ""
	for _, f := range d.HotelDetailFacilityList {
		name := strings.TrimSpace(f.Name)
		if name == "" || !bool(f.IsActive) {
			continue
		}
		val := ""
		if f.Value != nil {
			val = strings.TrimSpace(*f.Value)
		}
		cat := strings.TrimSpace(f.Category)
		if cat == "" {
			cat = "Genel Özellikler"
		}
		switch {
		case f.Type == "CONTENT":
			txt := htmlToText(val)
			if txt == "" {
				txt = htmlToText(name)
			}
			if txt != "" {
				if description != "" {
					description += "\n\n"
				}
				description += txt
			}
		case cat == "Konum Bilgileri":
			up := strings.ToUpper(name)
			switch {
			case strings.Contains(up, "LATITUDE"), strings.Contains(up, "LONGITUDE"):
				if out.Location == nil {
					// Ozellik listesindeki koordinat yedek olarak kullanilir.
					if strings.Contains(up, "LATITUDE") {
						if v, err := strconv.ParseFloat(strings.ReplaceAll(val, ",", "."), 64); err == nil {
							out.Location = &provider.GeoPoint{Lat: v}
						}
					}
				}
				if out.Location != nil && strings.Contains(up, "LONGITUDE") && out.Location.Lng == 0 {
					if v, err := strconv.ParseFloat(strings.ReplaceAll(val, ",", "."), 64); err == nil {
						out.Location.Lng = v
					}
				}
			default:
				location = append(location, provider.KV{Key: titleCaseTR(name), Value: val})
				switch {
				case strings.HasPrefix(up, "SEH") || strings.HasPrefix(up, "ŞEH"):
					if !strings.Contains(up, "MESAFE") && out.City == "" {
						out.City = val
					}
				case strings.HasPrefix(up, "ÜLKE") || strings.HasPrefix(up, "ULKE"):
					out.Country = val
				}
			}
		case (cat == "Genel Bilgiler" || cat == "Genel Özellikler") && (strings.Contains(name, "Saat") || strings.Contains(name, "Hayvan")):
			rules = append(rules, provider.KV{Key: name, Value: val})
			switch {
			case strings.Contains(name, "Giriş") && pol.CheckIn == "":
				pol.CheckIn = val
			case strings.Contains(name, "Çıkış") && pol.CheckOut == "":
				pol.CheckOut = val
			case strings.Contains(name, "Hayvan") && pol.PetPolicy == "":
				pol.PetPolicy = strings.TrimSpace(name + " " + val)
			}
		default:
			g := get(cat)
			if f.Type == "TEXT" && val != "" {
				g.pairs = append(g.pairs, provider.KV{Key: name, Value: htmlToText(val)})
				continue
			}
			item := name
			if bool(f.IsPaid) {
				item += " (Ücretli)"
			}
			g.items = append(g.items, item)
		}
	}
	if h.Description != nil && strings.TrimSpace(*h.Description) != "" && description == "" {
		description = htmlToText(*h.Description)
	}
	out.Description = description
	if out.Location != nil && (out.Location.Lat == 0 || out.Location.Lng == 0) {
		out.Location = nil
	}
	if out.Country == "" {
		for _, kv := range location {
			if strings.EqualFold(kv.Key, "Ülke") {
				out.Country = kv.Value
			}
		}
	}

	if description != "" {
		out.Sections = append(out.Sections, provider.Section{Title: "Otel Hakkında", Kind: "text", Text: description})
	}
	for _, cat := range order {
		g := groups[cat]
		if len(g.items) > 0 {
			out.Facilities = append(out.Facilities, provider.FacilityGroup{Title: g.title, Items: g.items})
		}
		if len(g.pairs) > 0 {
			out.Sections = append(out.Sections, provider.Section{Title: g.title, Kind: "pairs", Pairs: g.pairs})
		}
	}
	if len(rules) > 0 {
		out.Sections = append(out.Sections, provider.Section{Title: "Tesis Kuralları", Kind: "pairs", Pairs: rules})
	}
	if len(location) > 0 {
		out.Sections = append(out.Sections, provider.Section{Title: "Konum Bilgileri", Kind: "pairs", Pairs: location})
	}
	if len(d.FacilityDistance) > 0 {
		pairs := make([]provider.KV, 0, len(d.FacilityDistance))
		for _, fd := range d.FacilityDistance {
			if strings.TrimSpace(fd.Name) == "" {
				continue
			}
			pairs = append(pairs, provider.KV{Key: strings.TrimSpace(fd.Name), Value: strings.TrimSpace(fd.Distance)})
		}
		if len(pairs) > 0 {
			out.Sections = append(out.Sections, provider.Section{Title: "Mesafeler", Kind: "pairs", Pairs: pairs})
		}
	}
	if gi := d.GeneralInformation; gi != nil {
		pairs := []provider.KV{}
		for _, kv := range [][2]any{{"Oda Sayısı", gi.RoomsNumber}, {"İnşa Yılı", gi.YearBuilt}, {"Renovasyon Yılı", gi.YearRenovated}} {
			if s := anyString(kv[1]); s != "" && s != "-" {
				pairs = append(pairs, provider.KV{Key: kv[0].(string), Value: s})
			}
		}
		if len(pairs) > 0 {
			out.Sections = append(out.Sections, provider.Section{Title: "Genel Bilgiler", Kind: "pairs", Pairs: pairs})
		}
	}
	if len(d.Opportunities) > 0 {
		items := []string{}
		for _, o := range d.Opportunities {
			if t := htmlToText(o); t != "" {
				items = append(items, t)
			}
		}
		if len(items) > 0 {
			out.Sections = append(out.Sections, provider.Section{Title: "Fırsatlar", Kind: "list", Items: items})
		}
	}
	if len(d.ListPolicyStruct) > 0 {
		pairs := []provider.KV{}
		for _, ps := range d.ListPolicyStruct {
			t, desc := strings.TrimSpace(ps.Title), htmlToText(ps.Description)
			if t == "" && desc == "" {
				continue
			}
			pairs = append(pairs, provider.KV{Key: t, Value: desc})
			lt := strings.ToLower(t)
			if strings.Contains(lt, "çocuk") && pol.ChildPolicy == "" {
				pol.ChildPolicy = desc
			}
			if strings.Contains(lt, "iptal") && pol.Cancellation == "" {
				pol.Cancellation = desc
			}
		}
		if len(pairs) > 0 {
			out.Sections = append(out.Sections, provider.Section{Title: "Politikalar", Kind: "pairs", Pairs: pairs})
		}
	}

	// Odalar: ayni oda farkli pansiyonlarla tekrar eder; ada gore birlestirilir, ad sirasina gore siralanir.
	type roomAcc struct {
		info     provider.RoomInfo
		features map[string]bool
		images   map[string]bool
	}
	rooms := map[string]*roomAcc{}
	boards := map[string]bool{}
	for _, r := range d.HotelRoomDetailsList {
		name := strings.TrimSpace(r.RoomName)
		if name == "" {
			continue
		}
		if b := strings.TrimSpace(r.Mealy); b != "" {
			boards[b] = true
		}
		acc, ok := rooms[name]
		if !ok {
			acc = &roomAcc{info: provider.RoomInfo{Name: name}, features: map[string]bool{}, images: map[string]bool{}}
			rooms[name] = acc
		}
		if acc.info.Size == nil {
			if n := roomSizeOf(r.RoomSize); n > 0 {
				acc.info.Size = &n
			}
		}
		if acc.info.Description == "" && r.RoomInfo != nil {
			acc.info.Description = htmlToText(*r.RoomInfo)
		}
		if acc.info.Capacity == "" && (r.AdultCount > 0 || r.ChildCount > 0) {
			acc.info.Capacity = fmt.Sprintf("%d yetişkin", r.AdultCount)
			if r.ChildCount > 0 {
				acc.info.Capacity += fmt.Sprintf(" + %d çocuk", r.ChildCount)
			}
		}
		if r.BedType != nil && strings.TrimSpace(*r.BedType) != "" {
			acc.features["Yatak: "+strings.TrimSpace(*r.BedType)] = true
		}
		for _, f := range r.TourLogicFacility {
			n := strings.TrimSpace(f.Name)
			if n == "" || f.Category == "Yeme & İçme Konsepti" {
				continue // pansiyona bagli
			}
			if bool(f.IsPaid) {
				n += " (Ücretli)"
			}
			acc.features[n] = true
		}
		if r.Image != nil {
			if u := cleanImageURL(*r.Image); u != "" {
				acc.images[u] = true
			}
		}
		for _, im := range r.ImageList {
			if u := cleanImageURL(im.Original); u != "" {
				acc.images[u] = true
			}
		}
	}
	names := make([]string, 0, len(rooms))
	for n := range rooms {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		acc := rooms[n]
		acc.info.Features = sortedKeys(acc.features)
		acc.info.Images = sortedKeys(acc.images)
		out.Rooms = append(out.Rooms, acc.info)
	}
	out.Boards = sortedKeys(boards)
	if len(out.Boards) == 0 {
		out.Boards = nil
	}
	if pol.CheckIn != "" || pol.CheckOut != "" || pol.ChildPolicy != "" || pol.PetPolicy != "" || pol.Cancellation != "" || len(pol.Other) > 0 {
		out.Policies = pol
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func anyString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

var reDigits = regexp.MustCompile(`\d+`)

func roomSizeOf(v any) int {
	s := anyString(v)
	if s == "" {
		return 0
	}
	if d := reDigits.FindString(s); d != "" {
		n, _ := strconv.Atoi(d)
		return n
	}
	return 0
}

// parseGeo "35,33031" / "33.420868" -> float.
func parseGeo(lat, lng *string) (float64, float64, bool) {
	if lat == nil || lng == nil {
		return 0, 0, false
	}
	a, err1 := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(*lat), ",", "."), 64)
	b, err2 := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(*lng), ",", "."), 64)
	if err1 != nil || err2 != nil || a == 0 || b == 0 {
		return 0, 0, false
	}
	return a, b, true
}

var reSpaces = regexp.MustCompile(`[ \t\x{00a0}]+`)
var reBlankLines = regexp.MustCompile(`\n\s*\n+`)

// htmlToText HTML parcasini duz metne cevirir (script/style atilir, blok etiketleri satir sonu olur).
func htmlToText(h string) string {
	h = strings.TrimSpace(strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(h))
	if h == "" {
		return ""
	}
	if !strings.Contains(h, "<") {
		return strings.TrimSpace(reSpaces.ReplaceAllString(provider.DecodeEntities(h), " "))
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(h))
	if err != nil {
		return strings.TrimSpace(provider.DecodeEntities(provider.StripTags(h)))
	}
	doc.Find("script,style,noscript,iframe").Remove()
	doc.Find("br").Each(func(_ int, s *goquery.Selection) { s.ReplaceWithHtml("\n") })
	doc.Find("p,div,li,tr,h1,h2,h3,h4,h5,h6,ul,ol,table,section,article").Each(func(_ int, s *goquery.Selection) {
		s.PrependHtml("\n")
		s.AppendHtml("\n")
	})
	txt := doc.Text()
	lines := strings.Split(txt, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(reSpaces.ReplaceAllString(l, " "))
	}
	txt = strings.Join(lines, "\n")
	txt = reBlankLines.ReplaceAllString(txt, "\n")
	return strings.TrimSpace(txt)
}

// titleCaseTR "HAVALİMANINA MESAFE" -> "Havalimanına Mesafe" (sayfadaki text-transform: capitalize karsiligi).
func titleCaseTR(s string) string {
	words := strings.Fields(strings.ToLowerSpecial(unicode.TurkishCase, s))
	for i, w := range words {
		r := []rune(w)
		r[0] = unicode.TurkishCase.ToUpper(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

// ---- Isimle arama ----

// Kibris otellerinde servis tipi 8; autocomplete tipi vermez, tarihsiz HotelDetail ile dogrulanir.
var serviceTypeCandidates = []string{"8", "1", "2", "3", "4", "5", "6", "7", "9", "10", "11", "12"}

const siteBase = "https://www.prontotour.com"

var reSlugDrop = regexp.MustCompile(`[^A-Za-z0-9 -]+`)

// seoSlug sitenin getSeoUrl karsiligi: Turkce harfler sadelestirilir, noktalama atilir, kelimeler buyuk harfle baslar.
func seoSlug(name string) string {
	r := strings.NewReplacer("İ", "i", "I", "i", "ı", "i", "ğ", "g", "Ğ", "g", "ü", "u", "Ü", "u", "ş", "s", "Ş", "s", "ö", "o", "Ö", "o", "ç", "c", "Ç", "c", "–", "-", "—", "-", "&", "", "'", "", "’", "")
	s := reSlugDrop.ReplaceAllString(r.Replace(strings.TrimSpace(name)), "")
	parts := []string{}
	for _, w := range strings.FieldsFunc(s, func(c rune) bool { return c == ' ' || c == '-' }) {
		w = strings.ToLower(w)
		parts = append(parts, strings.ToUpper(w[:1])+w[1:])
	}
	if len(parts) == 0 {
		return "Otel"
	}
	return strings.Join(parts, "-")
}

func detailURL(name, code, serviceType string) string {
	return siteBase + "/" + seoSlug(name) + "-h" + code + "-" + serviceType
}

// SearchHotels AutoComplete/Search (Type H, IdDestType 5 = otel) ile isimle arar; her aday icin
// servis tipi tarihsiz HotelDetail ile dogrulanir ve detay URL'i kurulur.
func (p *Provider) SearchHotels(ctx context.Context, query string) ([]provider.HotelCandidate, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("arama metni bos")
	}
	items, err := autoCompleteHotels(ctx, query)
	if err != nil {
		return nil, err
	}
	// Sonuc yoksa sorguyu sondan kisaltarak tekrar dene.
	words := strings.Fields(query)
	for len(items) == 0 && len(words) > 1 {
		words = words[:len(words)-1]
		if items, err = autoCompleteHotels(ctx, strings.Join(words, " ")); err != nil {
			return nil, err
		}
	}
	out := []provider.HotelCandidate{}
	for _, it := range items {
		st, name := resolveServiceType(ctx, it.Code)
		if st == "" {
			continue
		}
		if name == "" {
			name = strings.TrimSpace(it.Name)
		}
		out = append(out, provider.HotelCandidate{
			Name: name, URL: detailURL(name, it.Code, st), City: strings.TrimSpace(it.Parent1), ProviderHotelID: it.Code,
		})
	}
	return out, nil
}

type autoCompleteHotel struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	IdDestType int    `json:"IdDestType"`
	Parent1    string `json:"Parent1"`
	Parent2    string `json:"Parent2"`
}

func autoCompleteHotels(ctx context.Context, key string) ([]autoCompleteHotel, error) {
	body := map[string]any{"Key": key, "Type": "H", "Id": 0, "Source": 1}
	var items []autoCompleteHotel
	if err := postJSON(ctx, autoCompleteURL, body, &items); err != nil {
		return nil, err
	}
	return filterHotels(items), nil
}

// filterHotels yalnizca otel kayitlarini (type H, IdDestType 5, kodu olan) birakir; tekrarlari atar.
func filterHotels(items []autoCompleteHotel) []autoCompleteHotel {
	out := []autoCompleteHotel{}
	seen := map[string]bool{}
	for _, it := range items {
		if it.Type != "H" || it.IdDestType != 5 || strings.TrimSpace(it.Code) == "" || seen[it.Code] {
			continue
		}
		seen[it.Code] = true
		out = append(out, it)
	}
	return out
}

// resolveServiceType adaylari sirayla dener; ilk dolu yanitin tipini ve API'deki otel adini doner.
// Yanlis tipte HotelListResponse null gelir; dogru tipte ama icerigi/musaitligi olmayan otelde (orn. Elexus)
// HotelListResponse bos bir kabuk (Name null) gelir — bu durumda tip kabul edilir, ad autocomplete'ten kalir.
func resolveServiceType(ctx context.Context, code string) (string, string) {
	shellType := ""
	for _, st := range serviceTypeCandidates {
		d, err := fetchHotelDetail(ctx, code, st, "", "", buildTravelers(1, 0, nil), nil)
		if err != nil {
			return "", ""
		}
		if d.HotelListResponse == nil {
			continue
		}
		if strings.TrimSpace(d.HotelListResponse.Name) != "" {
			return st, strings.Join(strings.Fields(d.HotelListResponse.Name), " ")
		}
		if shellType == "" {
			shellType = st
		}
	}
	return shellType, ""
}
