package etstur

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"pricebot/internal/httpx"
	"pricebot/internal/provider"
)

// Statik otel detayi: services/api/hotel/detail/<slug> JSON'u (sayfadaki __NEXT_DATA__
// ile ayni veri). Oda listesi icin tarih zorunlu oldugundan room/multi sabit bir
// pencereyle cagrilir (bkz. staticStayWindow).

const (
	detailAPIURL    = "https://www.etstur.com/services/api/hotel/detail/"
	autocompleteURL = "https://www.etstur.com/v2/autocomplete"
	siteBase        = "https://www.etstur.com/"
)

type facilityItem struct {
	Label       string `json:"label"`
	Description string `json:"description"`
	PaidType    string `json:"paidType"`
}

type facilityGroup struct {
	MainTitle string         `json:"mainTitle"`
	PaidType  string         `json:"paidType"`
	Items     []facilityItem `json:"items"`
}

type landmark struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Distance any    `json:"distance"`
}

type detailData struct {
	HotelID         any    `json:"hotelId"`
	Name            string `json:"name"`
	URL             string `json:"url"`
	MetaDescription string `json:"metaDescription"`
	StarRating      any    `json:"starRating"`
	Location        *struct {
		LocationStr  string `json:"locationStr"`
		CustomRegion string `json:"customRegion"`
		State        string `json:"state"`
		City         string `json:"city"`
		Country      string `json:"country"`
	} `json:"location"`
	Images []string `json:"images"`
	Infos  []struct {
		Title       string `json:"title"`
		Active      bool   `json:"active"`
		GeneralInfo *struct {
			Themes []struct {
				Label string `json:"label"`
			} `json:"themes"`
			GeneralInfoText string          `json:"generalInfoText"`
			Facilities      []facilityGroup `json:"facilities"`
		} `json:"generalInfo"`
		ImportantInfo *struct {
			CheckInTime  string `json:"checkInTime"`
			CheckOutTime string `json:"checkOutTime"`
			Desc         string `json:"desc"`
		} `json:"importantInfo"`
		FacilityCategories []struct {
			Title *struct {
				Label string `json:"label"`
			} `json:"title"`
			FreeText   string          `json:"freeText"`
			Facilities []facilityGroup `json:"facilities"`
		} `json:"facilityCategories"`
	} `json:"infos"`
	Events []struct {
		Title     string `json:"title"`
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
	} `json:"events"`
	PoiMap *struct {
		GeoLocation *struct {
			Lat float64 `json:"lat"`
			Lng float64 `json:"lng"`
		} `json:"geoLocation"`
		Landmarks map[string][]landmark `json:"landmarks"`
	} `json:"poiMap"`
	Reviews *struct {
		Average *struct {
			Score       float64 `json:"score"`
			Name        string  `json:"name"`
			ReviewCount int     `json:"reviewCount"`
		} `json:"average"`
		ScoreTypes []struct {
			Name  string  `json:"name"`
			Score float64 `json:"score"`
		} `json:"scoreTypes"`
	} `json:"reviews"`
	Contact *struct {
		Email        string   `json:"email"`
		Phone        string   `json:"phone"`
		AddressLines []string `json:"addressLines"`
	} `json:"contact"`
	Faqs []struct {
		Question string `json:"question"`
		Answer   string `json:"answer"`
	} `json:"faqs"`
}

type detailResponse struct {
	Success      bool        `json:"success"`
	ErrorMessage *string     `json:"errorMessage"`
	Result       *detailData `json:"result"`
}

// roomStatic room/multi yanitindaki odanin statik alanlari.
type roomStatic struct {
	RoomID           string   `json:"roomId"`
	RoomName         string   `json:"roomName"`
	Description      string   `json:"description"`
	RoomSize         *int     `json:"roomSize"`
	MaxAdultCapacity *int     `json:"maxAdultCapacity"`
	MaxChildCapacity *int     `json:"maxChildCapacity"`
	Images           []string `json:"images"`
	Facilities       []struct {
		Label string `json:"label"`
		Paid  bool   `json:"paid"`
	} `json:"facilities"`
	SubBoards []struct {
		BoardType *struct {
			Label string `json:"label"`
		} `json:"boardType"`
	} `json:"subBoards"`
}

type roomMultiResponse struct {
	Success bool `json:"success"`
	Result  *struct {
		RoomGroups []struct {
			Rooms []roomStatic `json:"rooms"`
		} `json:"roomGroups"`
	} `json:"result"`
}

// slugFromURL "https://www.etstur.com/Acapulco-Resort-Convention-Spa?x=1" -> "Acapulco-Resort-Convention-Spa"
func slugFromURL(u string) string {
	u = stripQuery(u)
	u = strings.TrimSuffix(u, "/")
	if i := strings.LastIndex(u, "/"); i >= 0 {
		u = u[i+1:]
	}
	return u
}

func (p *Provider) fetchDetail(ctx context.Context, slug, sourceURL string) (*detailData, error) {
	headers := httpx.Merge(defaultHeaders)
	if p.cookie != "" {
		headers["cookie"] = p.cookie
	}
	resp, err := httpx.Get(ctx, detailAPIURL+slug, headers, 25*time.Second)
	if err == nil {
		var out detailResponse
		if jerr := json.Unmarshal(resp.Body, &out); jerr == nil && out.Success && out.Result != nil && out.Result.Name != "" {
			return out.Result, nil
		}
	}
	// Yedek: detay sayfasindaki __NEXT_DATA__ (ayni sema)
	if sourceURL == "" {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("Etstur detay API yaniti okunamadi")
	}
	return fetchDetailFromPage(ctx, sourceURL)
}

func fetchDetailFromPage(ctx context.Context, u string) (*detailData, error) {
	resp, err := httpx.Do(ctx, http.MethodGet, u, httpx.Options{Headers: pageHeaders, Timeout: 25 * time.Second})
	if err != nil {
		return nil, err
	}
	raw, err := extractNextData(resp.Text())
	if err != nil {
		return nil, err
	}
	var data struct {
		Props struct {
			PageProps struct {
				Data *detailData `json:"data"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(raw, &data); err != nil || data.Props.PageProps.Data == nil {
		return nil, fmt.Errorf("Etstur __NEXT_DATA__ parse edilemedi")
	}
	return data.Props.PageProps.Data, nil
}

func extractNextData(html string) ([]byte, error) {
	i := strings.Index(html, "__NEXT_DATA__")
	if i == -1 {
		return nil, fmt.Errorf("Etstur sayfasinda __NEXT_DATA__ bulunamadi (URL gecersiz olabilir)")
	}
	j := strings.Index(html[i:], ">") + i + 1
	k := strings.Index(html[j:], "</script>")
	if k == -1 {
		return nil, fmt.Errorf("Etstur __NEXT_DATA__ parse edilemedi")
	}
	return []byte(html[j : j+k]), nil
}

// fetchRoomsStatic room/multi ile sabit pencerede oda listesini ceker; hata durumunda bos liste.
func (p *Provider) fetchRoomsStatic(ctx context.Context, hotelID string) []roomStatic {
	ci, co := staticStayWindow(time.Now())
	body, _ := json.Marshal(map[string]any{
		"hotelId":  hotelID,
		"checkIn":  ci,
		"checkOut": co,
		"rooms":    []roomRequest{{AdultCount: 2, ChildCount: 0, ChildAges: []int{}, InfantCount: 0}},
	})
	resp, err := httpx.Post(ctx, roomURL, body, httpx.Merge(defaultHeaders), 30*time.Second)
	if err != nil {
		return nil
	}
	var data roomMultiResponse
	if err := json.Unmarshal(resp.Body, &data); err != nil || data.Result == nil {
		return nil
	}
	var rooms []roomStatic
	for _, g := range data.Result.RoomGroups {
		rooms = append(rooms, g.Rooms...)
	}
	return rooms
}

// GetDetails Etstur otel detayini ortak semaya cevirir.
func (p *Provider) GetDetails(ctx context.Context, m *provider.Mapping) (*provider.HotelDetails, error) {
	src := m.MetaString("sourceUrl")
	slug := m.MetaString("slug")
	if slug == "" && src != "" {
		slug = slugFromURL(src)
	}
	if slug == "" {
		return nil, fmt.Errorf("Etstur slug cozulemedi (metadata.slug veya sourceUrl gerekli)")
	}
	d, err := p.fetchDetail(ctx, slug, src)
	if err != nil {
		return nil, err
	}
	hotelID := m.ProviderHotelID
	if hotelID == "" {
		hotelID = anyToString(d.HotelID)
	}
	if src == "" {
		src = siteBase + slug
	}
	rooms := p.fetchRoomsStatic(ctx, hotelID)
	return normalizeDetails(d, rooms, hotelID, src), nil
}

func anyToString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	}
	return ""
}

func anyToInt(v any) *int {
	var n int
	switch t := v.(type) {
	case float64:
		n = int(t)
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(t))
		if err != nil {
			return nil
		}
		n = i
	default:
		return nil
	}
	if n <= 0 {
		return nil
	}
	return &n
}

func formatDistance(v any) string {
	switch t := v.(type) {
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64) + " km"
	case string:
		if t = strings.TrimSpace(t); t != "" {
			return t + " km"
		}
	}
	return ""
}

// facilityLabel "Cocuk Bakicisi (Rezervasyonlu) (Ucretli)" gibi tek satirlik etiket uretir.
func facilityLabel(prefix string, it facilityItem) string {
	s := provider.DecodeEntities(it.Label)
	if s == "" {
		return ""
	}
	if prefix != "" {
		s = prefix + ": " + s
	}
	if d := provider.DecodeEntities(it.Description); d != "" && !strings.EqualFold(d, it.Label) {
		s += " (" + d + ")"
	}
	if pt := strings.TrimSpace(it.PaidType); pt != "" {
		s += " (" + pt + ")"
	}
	return s
}

func groupItems(groups []facilityGroup) []string {
	var items []string
	for _, g := range groups {
		for _, it := range g.Items {
			if s := facilityLabel(provider.DecodeEntities(g.MainTitle), it); s != "" {
				items = append(items, s)
			}
		}
	}
	return uniq(items)
}

var reSentence = regexp.MustCompile(`[^.!?\n]*[Ee]vcil[^.!?\n]*[.!?]?`)

func normalizeDetails(d *detailData, rooms []roomStatic, hotelID, sourceURL string) *provider.HotelDetails {
	out := &provider.HotelDetails{
		Provider: "etstur", ProviderHotelID: hotelID, SourceURL: sourceURL, FetchedAt: time.Now().UTC(),
		Name:       provider.DecodeEntities(d.Name),
		Images:     []provider.Image{},
		Facilities: []provider.FacilityGroup{},
		Rooms:      []provider.RoomInfo{},
		FAQ:        []provider.QA{},
		Sections:   []provider.Section{},
	}
	out.Stars = anyToInt(d.StarRating)
	if d.Location != nil {
		out.City = provider.DecodeEntities(d.Location.City)
		out.Region = provider.DecodeEntities(d.Location.CustomRegion)
		if out.Region == "" {
			out.Region = provider.DecodeEntities(d.Location.State)
		}
		out.Country = provider.DecodeEntities(d.Location.Country)
	}
	if d.Contact != nil {
		var lines []string
		for _, l := range d.Contact.AddressLines {
			if l = provider.DecodeEntities(l); l != "" {
				lines = append(lines, l)
			}
		}
		out.Address = strings.Join(lines, ", ")
		c := &provider.Contact{Phone: strings.TrimSpace(d.Contact.Phone), Email: strings.TrimSpace(d.Contact.Email)}
		if c.Phone != "" || c.Email != "" {
			out.Contact = c
		}
	}
	if d.PoiMap != nil && d.PoiMap.GeoLocation != nil && d.PoiMap.GeoLocation.Lat != 0 {
		out.Location = &provider.GeoPoint{Lat: d.PoiMap.GeoLocation.Lat, Lng: d.PoiMap.GeoLocation.Lng}
	}
	if d.Reviews != nil && d.Reviews.Average != nil && d.Reviews.Average.Score > 0 {
		r := round1(d.Reviews.Average.Score / 10)
		out.Rating = &r
		if d.Reviews.Average.ReviewCount > 0 {
			n := d.Reviews.Average.ReviewCount
			out.RatingCount = &n
		}
	}
	for _, img := range d.Images {
		if u := stripQuery(absURL(siteBase, img)); u != "" {
			out.Images = append(out.Images, provider.Image{URL: u})
		}
	}

	// Aktif sezon bilgisi (yoksa ilk)
	infoIdx := -1
	for i := range d.Infos {
		if d.Infos[i].Active {
			infoIdx = i
			break
		}
	}
	if infoIdx == -1 && len(d.Infos) > 0 {
		infoIdx = 0
	}
	pol := &provider.Policies{Other: []string{}}
	if infoIdx >= 0 {
		inf := d.Infos[infoIdx]
		if inf.GeneralInfo != nil {
			out.Description = cleanHTML(inf.GeneralInfo.GeneralInfoText)
			if out.Description != "" {
				out.Sections = append(out.Sections, provider.Section{Title: "Genel Bilgiler", Kind: "text", Text: out.Description})
			}
			var themes []string
			for _, t := range inf.GeneralInfo.Themes {
				themes = append(themes, provider.DecodeEntities(t.Label))
			}
			if themes = uniq(themes); len(themes) > 0 {
				out.Sections = append(out.Sections, provider.Section{Title: "Temalar", Kind: "list", Items: themes})
			}
			for _, g := range inf.GeneralInfo.Facilities {
				title := provider.DecodeEntities(g.MainTitle)
				if title == "" {
					title = "Genel Özellikler"
				}
				if items := groupItems([]facilityGroup{{Items: g.Items}}); len(items) > 0 {
					out.Facilities = append(out.Facilities, provider.FacilityGroup{Title: title, Items: items})
				}
			}
		}
		for _, fc := range inf.FacilityCategories {
			title := ""
			if fc.Title != nil {
				title = provider.DecodeEntities(fc.Title.Label)
			}
			if title == "" {
				continue
			}
			items := groupItems(fc.Facilities)
			text := cleanHTML(fc.FreeText)
			if len(items) > 0 {
				out.Facilities = append(out.Facilities, provider.FacilityGroup{Title: title, Items: items})
			}
			if text != "" || len(items) > 0 {
				kind := "text"
				if text == "" {
					kind = "list"
				}
				out.Sections = append(out.Sections, provider.Section{Title: title, Kind: kind, Text: text, Items: items})
			}
			lt := strings.ToLower(title)
			if strings.Contains(lt, "çocuk") && text != "" && pol.ChildPolicy == "" {
				pol.ChildPolicy = text
			}
			if m := reSentence.FindString(text); m != "" && pol.PetPolicy == "" {
				pol.PetPolicy = strings.TrimSpace(m)
			}
		}
		if inf.ImportantInfo != nil {
			pol.CheckIn = strings.TrimSpace(inf.ImportantInfo.CheckInTime)
			pol.CheckOut = strings.TrimSpace(inf.ImportantInfo.CheckOutTime)
			if text := cleanHTML(inf.ImportantInfo.Desc); text != "" {
				out.Sections = append(out.Sections, provider.Section{Title: "Önemli Bilgiler", Kind: "text", Text: text})
				for _, para := range strings.Split(text, "\n") {
					pol.Other = append(pol.Other, para)
				}
				if pol.PetPolicy == "" {
					if m := reSentence.FindString(text); m != "" {
						pol.PetPolicy = strings.TrimSpace(m)
					}
				}
			}
		}
	}
	if pol.CheckIn != "" || pol.CheckOut != "" || pol.ChildPolicy != "" || pol.PetPolicy != "" || len(pol.Other) > 0 {
		out.Policies = pol
	}

	// Etkinlikler (tarihe gore sirali)
	type ev struct{ start, title string }
	var evs []ev
	for _, e := range d.Events {
		t := provider.DecodeEntities(e.Title)
		if t == "" {
			continue
		}
		if e.StartDate != "" {
			t += " (" + e.StartDate
			if e.EndDate != "" && e.EndDate != e.StartDate {
				t += " – " + e.EndDate
			}
			t += ")"
		}
		evs = append(evs, ev{e.StartDate, t})
	}
	sort.SliceStable(evs, func(i, j int) bool {
		if evs[i].start != evs[j].start {
			return evs[i].start < evs[j].start
		}
		return evs[i].title < evs[j].title
	})
	if len(evs) > 0 {
		items := make([]string, 0, len(evs))
		for _, e := range evs {
			items = append(items, e.title)
		}
		out.Sections = append(out.Sections, provider.Section{Title: "Etkinlik Takvimi", Kind: "list", Items: uniq(items)})
	}

	// Konum / onemli noktalar (tur + ada gore sirali; API her istekte farkli sirada donebiliyor)
	if d.PoiMap != nil && len(d.PoiMap.Landmarks) > 0 {
		var pairs []provider.KV
		for _, typ := range sortedKeys(d.PoiMap.Landmarks) {
			lms := d.PoiMap.Landmarks[typ]
			sort.SliceStable(lms, func(i, j int) bool { return lms[i].Name < lms[j].Name })
			for _, lm := range lms {
				if n := provider.DecodeEntities(lm.Name); n != "" {
					pairs = append(pairs, provider.KV{Key: n, Value: formatDistance(lm.Distance)})
				}
			}
		}
		if len(pairs) > 0 {
			out.Sections = append(out.Sections, provider.Section{Title: "Konum / Önemli Noktalar", Kind: "pairs", Pairs: pairs})
		}
	}

	// Misafir degerlendirmesi (ortalama + kategori puanlari; tekil yorumlar alinmaz)
	if d.Reviews != nil && d.Reviews.Average != nil && d.Reviews.Average.Score > 0 {
		pairs := []provider.KV{{Key: "Genel", Value: fmt.Sprintf("%.1f", round1(d.Reviews.Average.Score/10))}}
		for _, st := range d.Reviews.ScoreTypes {
			if n := provider.DecodeEntities(st.Name); n != "" {
				pairs = append(pairs, provider.KV{Key: n, Value: fmt.Sprintf("%.1f", round1(st.Score/10))})
			}
		}
		if d.Reviews.Average.ReviewCount > 0 {
			pairs = append(pairs, provider.KV{Key: "Yorum Sayısı", Value: strconv.Itoa(d.Reviews.Average.ReviewCount)})
		}
		out.Sections = append(out.Sections, provider.Section{Title: "Misafir Değerlendirmesi", Kind: "pairs", Pairs: pairs})
	}

	for _, q := range d.Faqs {
		qq, aa := cleanHTML(q.Question), cleanHTML(q.Answer)
		if qq != "" && aa != "" {
			out.FAQ = append(out.FAQ, provider.QA{Question: qq, Answer: aa})
		}
	}

	out.Rooms, out.Boards = normalizeRoomsStatic(rooms)
	return out
}

// normalizeRoomsStatic room/multi odalarini pansiyon ekinden arindirip (ayni oda farkli
// pansiyonla tekrar eder) ada gore sirali tekil listeye cevirir.
func normalizeRoomsStatic(rooms []roomStatic) ([]provider.RoomInfo, []string) {
	boards := map[string]bool{}
	for _, r := range rooms {
		for _, sb := range r.SubBoards {
			if sb.BoardType != nil && sb.BoardType.Label != "" {
				boards[provider.DecodeEntities(sb.BoardType.Label)] = true
			}
		}
	}
	byName := map[string]*provider.RoomInfo{}
	for _, r := range rooms {
		name := provider.DecodeEntities(r.RoomName)
		for b := range boards {
			name = strings.TrimSuffix(name, " - "+b)
		}
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		ri, ok := byName[name]
		if !ok {
			ri = &provider.RoomInfo{Name: name, Features: []string{}, Images: []string{}}
			if r.RoomSize != nil && *r.RoomSize > 0 {
				s := *r.RoomSize
				ri.Size = &s
			}
			if r.MaxAdultCapacity != nil && *r.MaxAdultCapacity > 0 {
				ri.Capacity = fmt.Sprintf("%d yetişkin", *r.MaxAdultCapacity)
				if r.MaxChildCapacity != nil && *r.MaxChildCapacity > 0 {
					ri.Capacity += fmt.Sprintf(", %d çocuk", *r.MaxChildCapacity)
				}
			}
			ri.Description = cleanHTML(r.Description)
			byName[name] = ri
		}
		for _, f := range r.Facilities {
			l := provider.DecodeEntities(f.Label)
			if l == "" {
				continue
			}
			if f.Paid && !strings.Contains(l, "*") && !strings.Contains(strings.ToLower(l), "ücretli") {
				l += " (Ücretli)"
			}
			ri.Features = append(ri.Features, l)
		}
		for _, img := range r.Images {
			if u := stripQuery(absURL(siteBase, img)); u != "" {
				ri.Images = append(ri.Images, u)
			}
		}
	}
	out := make([]provider.RoomInfo, 0, len(byName))
	for _, name := range sortedKeys(byName) {
		ri := byName[name]
		ri.Features = uniq(ri.Features)
		ri.Images = uniq(ri.Images)
		out = append(out, *ri)
	}
	return out, sortedKeys(boards)
}

// ---- Isimle arama ----

type suggestion struct {
	Title   string `json:"title"`
	Type    string `json:"type"`
	URL     string `json:"url"`
	Country string `json:"country"`
	City    string `json:"city"`
	Region  string `json:"region"`
}

type suggestResponse struct {
	Success bool         `json:"success"`
	Result  []suggestion `json:"result"`
}

func (p *Provider) autocomplete(ctx context.Context, q string) ([]suggestion, error) {
	u := autocompleteURL + "?q=" + strings.ReplaceAll(strings.TrimSpace(q), " ", "%20")
	headers := httpx.Merge(defaultHeaders)
	delete(headers, "content-type")
	resp, err := httpx.Get(ctx, u, headers, 20*time.Second)
	if err != nil {
		return nil, err
	}
	var out suggestResponse
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("Etstur autocomplete yaniti parse edilemedi: %w", err)
	}
	return out.Result, nil
}

// hotelCandidates sadece HOTEL tipindeki onerileri adaya cevirir.
func hotelCandidates(list []suggestion) []provider.HotelCandidate {
	out := []provider.HotelCandidate{}
	seen := map[string]bool{}
	for _, s := range list {
		if s.Type != "HOTEL" || s.URL == "" || seen[s.URL] {
			continue
		}
		seen[s.URL] = true
		name := provider.DecodeEntities(s.Title)
		city := provider.DecodeEntities(s.City)
		// "Merit Royal Premium Hotel & Spa, Alsancak" -> ad + semt
		if i := strings.LastIndex(name, ", "); i > 0 {
			suffix := strings.TrimSpace(name[i+2:])
			if suffix != "" && len(suffix) <= 40 && !strings.ContainsAny(suffix, "0123456789") {
				name = strings.TrimSpace(name[:i])
				if city == "" {
					city = suffix
				} else if !strings.EqualFold(city, suffix) {
					city = suffix + ", " + city
				}
			}
		}
		out = append(out, provider.HotelCandidate{Name: name, URL: siteBase + strings.TrimPrefix(s.URL, "/"), City: city})
	}
	return out
}

// SearchHotels /v2/autocomplete?q= ile otel adaylarini doner; sonuc yoksa sorguyu
// kelime kelime kisaltarak tekrar dener.
func (p *Provider) SearchHotels(ctx context.Context, query string) ([]provider.HotelCandidate, error) {
	words := strings.Fields(strings.TrimSpace(query))
	if len(words) == 0 {
		return nil, fmt.Errorf("Arama metni bos")
	}
	var lastErr error
	for n := len(words); n >= 1; n-- {
		q := strings.Join(words[:n], " ")
		if len([]rune(q)) < 3 {
			break
		}
		list, err := p.autocomplete(ctx, q)
		if err != nil {
			lastErr = err
			continue
		}
		if c := hotelCandidates(list); len(c) > 0 {
			return c, nil
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return []provider.HotelCandidate{}, nil
}

var (
	_ provider.DetailProvider = (*Provider)(nil)
	_ provider.HotelSearcher  = (*Provider)(nil)
)
