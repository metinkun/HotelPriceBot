package touristica

// Detay sayfasi / oda listesi / otomatik tamamlama HTML'lerinin saf (yan etkisiz) ayristirmasi.
// Cikti deterministiktir: siralama DOM sirasina gore, tekrarlar ilk gorunumde tutulur,
// gorsel URL'lerinde sorgu parametresi ve boyut segmenti (/800x450/) temizlenir.

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"pricebot/internal/provider"
)

var (
	reBr        = regexp.MustCompile(`(?i)<br\s*/?>`)
	reLiOpen    = regexp.MustCompile(`(?i)<li[^>]*>`)
	reBlockEnd  = regexp.MustCompile(`(?i)</(li|p|div|h[1-6]|ul|ol|section|tr)>`)
	reBlockOpen = regexp.MustCompile(`(?i)<(p|div|h[1-6]|ul|ol|section|tr)[^>]*>`)
	reSpaces    = regexp.MustCompile(`[ \t\r\f\v\x{00a0}]+`)
	reImgSize   = regexp.MustCompile(`/\d{2,4}x\d{2,4}/`)
	reRoomSize  = regexp.MustCompile(`(\d{2,4})(?:\s*[-–]\s*\d{2,4})?\s*(?:m²|m2|metrekare)`)
	reTime      = regexp.MustCompile(`(\d{1,2})[:.](\d{2})(?:[^\d]|$)`)
	reCheckIn   = regexp.MustCompile(`(?i)(?:yerleşme|yerlesme|giriş saati|giris saati|giriş|check-?in)[^\n]{0,80}?(\d{1,2}[:.]\d{2})(?:[^\d]|$)`)
	reCheckOut  = regexp.MustCompile(`(?i)(?:çıkış|cikis|ayrılış|ayrilis|check-?out)[^\n]{0,120}?(\d{1,2}[:.]\d{2})(?:[^\d]|$)`)
	reChildPol  = regexp.MustCompile(`(?i)çocuk.*(politika|kural|indirim|yaş)`)
	reCancel    = regexp.MustCompile(`(?i)iptal`)
	reHotelType = regexp.MustCompile(`"@type"\s*:\s*"Hotel"`)
)

// cleanText bosluklari sadelestirir ve HTML varliklarini cozer.
func cleanText(s string) string {
	s = provider.DecodeEntities(s)
	s = reSpaces.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// htmlToLines basit HTML govdeyi satirlara cevirir: <br>, <li>, <p> satir sonu; <li> satirlari "• " ile baslar.
func htmlToLines(h string) []string {
	h = reBr.ReplaceAllString(h, "\n")
	h = reLiOpen.ReplaceAllString(h, "\n• ")
	h = reBlockEnd.ReplaceAllString(h, "\n")
	h = reBlockOpen.ReplaceAllString(h, "\n")
	h = provider.StripTags(h)
	var out []string
	for _, ln := range strings.Split(h, "\n") {
		ln = cleanText(ln)
		if ln == "" || ln == "•" {
			continue
		}
		out = append(out, ln)
	}
	return out
}

// linesToSection satirlardan Section uretir: hepsi madde ise list, degilse text.
func linesToSection(title string, lines []string) provider.Section {
	s := provider.Section{Title: title}
	if len(lines) == 0 {
		s.Kind = "text"
		return s
	}
	allBullets := true
	items := make([]string, 0, len(lines))
	for _, ln := range lines {
		if strings.HasPrefix(ln, "• ") {
			items = append(items, strings.TrimSpace(strings.TrimPrefix(ln, "• ")))
		} else {
			allBullets = false
		}
	}
	if allBullets {
		s.Kind = "list"
		s.Items = items
		return s
	}
	s.Kind = "text"
	s.Text = strings.Join(lines, "\n")
	return s
}

// normalizeImageURL sorgu parametresini (cache-buster) ve boyut segmentini kaldirir, mutlak URL yapar.
func normalizeImageURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" || strings.HasPrefix(u, "data:") {
		return ""
	}
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	if strings.HasPrefix(u, "//") {
		u = "https:" + u
	} else if strings.HasPrefix(u, "/") {
		u = siteRoot + u
	}
	return reImgSize.ReplaceAllString(u, "/")
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// parseTRFloat "35,33029" -> 35.33029
func parseTRFloat(s string) (float64, bool) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}

func normTime(hhmm string) string { return strings.Replace(hhmm, ".", ":", 1) }

// ldHotel ld+json Hotel semasindan alinan alanlar.
type ldHotel struct {
	Name        string
	Description string
	Image       string
	Rating      *float64
	RatingCount *int
	Street      string
	Locality    string
	Country     string
}

func parseLDHotel(txt string) ldHotel {
	var out ldHotel
	var m map[string]any
	if err := json.Unmarshal([]byte(txt), &m); err != nil {
		return out
	}
	str := func(v any) string {
		switch t := v.(type) {
		case string:
			return cleanText(t)
		case float64:
			return strconv.FormatFloat(t, 'f', -1, 64)
		}
		return ""
	}
	num := func(v any) (float64, bool) {
		switch t := v.(type) {
		case float64:
			return t, true
		case string:
			f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
			return f, err == nil
		}
		return 0, false
	}
	out.Name = str(m["name"])
	out.Description = str(m["description"])
	out.Image = str(m["image"])
	if addr, ok := m["address"].(map[string]any); ok {
		out.Street = str(addr["streetAddress"])
		out.Locality = str(addr["addressLocality"])
		out.Country = str(addr["addressCountry"])
	}
	var agg map[string]any
	switch t := m["aggregateRating"].(type) {
	case map[string]any:
		agg = t
	case []any:
		if len(t) > 0 {
			agg, _ = t[0].(map[string]any)
		}
	}
	if agg != nil {
		if v, ok := num(agg["ratingValue"]); ok {
			best := 10.0
			if b, ok := num(agg["bestRating"]); ok && b > 0 {
				best = b
			}
			r := v * 10 / best
			out.Rating = &r
		}
		if c, ok := num(agg["reviewCount"]); ok {
			n := int(c)
			out.RatingCount = &n
		}
	}
	return out
}

// parseDetails detay sayfasi parcalari (pageHTML) ve /odalar HTML'inden HotelDetails uretir.
func parseDetails(pageHTML, roomsHTML string) (*provider.HotelDetails, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(pageHTML))
	if err != nil {
		return nil, fmt.Errorf("detay HTML ayristirilamadi: %w", err)
	}
	d := &provider.HotelDetails{
		Images: []provider.Image{}, Facilities: []provider.FacilityGroup{}, Rooms: []provider.RoomInfo{},
		FAQ: []provider.QA{}, Sections: []provider.Section{},
	}

	// Baslik: ad, konum metni ("Kıbrıs, Girne | Haritada Göster"), ust ozellik etiketleri
	d.Name = cleanText(doc.Find("h1").First().Text())
	loc := doc.Find(".hotel-location").First().Clone()
	loc.Find("a").Remove()
	locText := strings.Trim(cleanText(loc.Text()), "| ")
	var locParts []string
	for _, p := range strings.Split(locText, ",") {
		if p = strings.TrimSpace(p); p != "" {
			locParts = append(locParts, p)
		}
	}
	var headerAmenities []string
	doc.Find("ul.amenities li").Each(func(_ int, s *goquery.Selection) {
		if title, ok := s.Attr("title"); ok && s.HasClass("icon-add") {
			headerAmenities = append(headerAmenities, htmlToLines(title)...)
			return
		}
		if t := cleanText(s.Text()); t != "" {
			headerAmenities = append(headerAmenities, t)
		}
	})

	// ld+json Hotel
	var ld ldHotel
	doc.Find(`script[type="application/ld+json"]`).Each(func(_ int, s *goquery.Selection) {
		if ld.Name == "" && reHotelType.MatchString(s.Text()) {
			ld = parseLDHotel(s.Text())
		}
	})
	if d.Name == "" {
		d.Name = ld.Name
	}
	d.Address, d.City, d.Country = ld.Street, ld.Locality, ld.Country
	if d.City == "" && len(locParts) > 0 {
		d.City = locParts[len(locParts)-1]
	}
	if len(locParts) > 1 && locParts[0] != d.City && locParts[0] != d.Country {
		d.Region = locParts[0]
	}
	d.Rating, d.RatingCount = ld.Rating, ld.RatingCount

	// Konum (harita)
	if mc := doc.Find(".map-canvas[data-lat]").First(); mc.Length() > 0 {
		lat, ok1 := parseTRFloat(mc.AttrOr("data-lat", ""))
		lng, ok2 := parseTRFloat(mc.AttrOr("data-lng", ""))
		if ok1 && ok2 && lat != 0 && lng != 0 {
			d.Location = &provider.GeoPoint{Lat: lat, Lng: lng}
		}
	}

	// Galeri: gizli slider (tum resimler, kategori adiyla); yoksa ust galeri; yoksa ld gorseli
	seenImg := map[string]bool{}
	addImg := func(u, caption string) {
		u = normalizeImageURL(u)
		if u == "" || seenImg[u] {
			return
		}
		seenImg[u] = true
		d.Images = append(d.Images, provider.Image{URL: u, Caption: caption})
	}
	doc.Find(".detail-slider-wrapper .slider-img-wrapper[data-picture-id]").Each(func(_ int, s *goquery.Selection) {
		cap := cleanText(s.AttrOr("data-category", ""))
		if c := cleanText(s.AttrOr("data-caption", "")); c != "" {
			if cap != "" {
				cap += " - "
			}
			cap += c
		}
		addImg(s.AttrOr("data-src", ""), cap)
	})
	if len(d.Images) == 0 {
		doc.Find(".gallery-v5 img[data-src]").Each(func(_ int, s *goquery.Selection) { addImg(s.AttrOr("data-src", ""), "") })
	}
	if len(d.Images) == 0 && ld.Image != "" {
		addImg(ld.Image, "")
	}

	// "Otel Detaylari" sekmesi: acilir basliklar (metin/liste) + ozellik gruplari
	var genelUyari []string
	doc.Find("#check-list section.check-list-box").Each(func(_ int, s *goquery.Selection) {
		title := cleanText(s.Find("h3").First().Text())
		body, _ := s.Find(".check-list-description").First().Html()
		lines := htmlToLines(body)
		if title == "" || len(lines) == 0 {
			return
		}
		sec := linesToSection(title, lines)
		d.Sections = append(d.Sections, sec)
		lt := strings.ToLower(title)
		switch {
		case d.Description == "" && strings.Contains(lt, "tanıtım"):
			d.Description = strings.Join(lines, "\n")
		case strings.Contains(lt, "uyarı"):
			genelUyari = lines
		}
		if reChildPol.MatchString(lt) && d.Policies == nil {
			d.Policies = &provider.Policies{ChildPolicy: strings.Join(lines, "\n")}
		}
	})
	if d.Description == "" {
		d.Description = ld.Description
	}
	facIdx := map[string]int{}
	doc.Find(".other-check-list").Each(func(_ int, s *goquery.Selection) {
		title := cleanText(s.Find("h4").First().Text())
		var items []string
		s.Find("li").Each(func(_ int, li *goquery.Selection) {
			if t := cleanText(li.Text()); t != "" {
				items = append(items, t)
			}
		})
		items = uniqueStrings(items)
		if title == "" || len(items) == 0 {
			return
		}
		facIdx[title] = len(d.Facilities)
		d.Facilities = append(d.Facilities, provider.FacilityGroup{Title: title, Items: items})
		d.Sections = append(d.Sections, provider.Section{Title: title, Kind: "list", Items: items})
	})
	if len(headerAmenities) > 0 {
		const hotelFeat = "Otel Özellikleri"
		if i, ok := facIdx[hotelFeat]; ok {
			d.Facilities[i].Items = uniqueStrings(append(d.Facilities[i].Items, headerAmenities...))
		} else {
			d.Facilities = append(d.Facilities, provider.FacilityGroup{Title: hotelFeat, Items: uniqueStrings(headerAmenities)})
		}
	}

	// Kampanyalar (baslik -> metin)
	var cancellation string
	var campaigns []provider.KV
	doc.Find(".hotel-discount-detail-box").Each(func(_ int, s *goquery.Selection) {
		title := cleanText(s.Find("h4").First().Text())
		c := s.Clone()
		c.Find("h4").Remove()
		body, _ := c.Html()
		text := strings.Join(htmlToLines(body), "\n")
		if title == "" || text == "" {
			return
		}
		campaigns = append(campaigns, provider.KV{Key: title, Value: text})
		if cancellation == "" && reCancel.MatchString(title) {
			cancellation = text
		}
	})
	if len(campaigns) > 0 {
		d.Sections = append(d.Sections, provider.Section{Title: "Kampanyalar", Kind: "pairs", Pairs: campaigns})
	}

	// Etkinlik takvimi (tarih/sanatci -> aciklama)
	var events []provider.KV
	doc.Find("#activities-tab .hotel-activity-box").Each(func(_ int, s *goquery.Selection) {
		key := cleanText(s.Find("h4").First().Text())
		desc, _ := s.Find(".hotel-activity-description").First().Html()
		if key == "" {
			return
		}
		events = append(events, provider.KV{Key: key, Value: strings.Join(htmlToLines(desc), "\n")})
	})
	if len(events) > 0 {
		title := cleanText(doc.Find("#activities-tab h3").First().Text())
		if title == "" {
			title = "Etkinlik Takvimi"
		}
		d.Sections = append(d.Sections, provider.Section{Title: title, Kind: "pairs", Pairs: events})
	}

	// Yorum puanlari (genel puan ld+json'da yoksa buradan; kategori puanlari pairs)
	if rw := doc.Find(".review-wrapper").First(); rw.Length() > 0 {
		if d.Rating == nil {
			if f, ok := parseTRFloat(rw.Find(".score").First().Text()); ok {
				d.Rating = &f
			}
		}
		if d.RatingCount == nil {
			if n, err := strconv.Atoi(cleanText(rw.Find(".reviewcount").First().Text())); err == nil {
				c := n
				d.RatingCount = &c
			}
		}
		var scores []provider.KV
		rw.Find(".question").Each(func(_ int, q *goquery.Selection) {
			spans := q.Find("span")
			if spans.Length() < 2 {
				return
			}
			k := cleanText(spans.Eq(0).Text())
			v := cleanText(spans.Eq(1).Text())
			if k != "" && v != "" {
				scores = append(scores, provider.KV{Key: k, Value: v})
			}
		})
		if len(scores) > 0 {
			d.Sections = append(d.Sections, provider.Section{Title: "Yorumlar", Kind: "pairs", Pairs: scores})
		}
	}

	// Odalar ve pansiyon tipleri
	if roomsHTML != "" {
		rooms, boards, err := parseRooms(roomsHTML)
		if err != nil {
			return nil, err
		}
		d.Rooms, d.Boards = rooms, boards
	}

	// Politikalar: giris/cikis saati "Genel Uyarilar" (yoksa tum bolum metinleri) icinden
	polText := strings.Join(genelUyari, "\n")
	if polText == "" {
		var all []string
		for _, s := range d.Sections {
			if s.Kind == "text" {
				all = append(all, s.Text)
			} else {
				all = append(all, s.Items...)
			}
		}
		polText = strings.Join(all, "\n")
	}
	pol := d.Policies
	if pol == nil {
		pol = &provider.Policies{}
	}
	if m := reCheckIn.FindStringSubmatch(polText); m != nil {
		pol.CheckIn = normTime(m[1])
	}
	if m := reCheckOut.FindStringSubmatch(polText); m != nil {
		pol.CheckOut = normTime(m[1])
	}
	pol.Cancellation = cancellation
	if len(genelUyari) > 0 {
		pol.Other = make([]string, 0, len(genelUyari))
		for _, ln := range genelUyari {
			pol.Other = append(pol.Other, strings.TrimSpace(strings.TrimPrefix(ln, "• ")))
		}
	}
	if pol.CheckIn != "" || pol.CheckOut != "" || pol.ChildPolicy != "" || pol.Cancellation != "" || len(pol.Other) > 0 {
		d.Policies = pol
	}
	return d, nil
}

// parseRooms /odalar HTML'inden oda bilgisi (ad bazinda tekil) ve pansiyon tiplerini (gorunum sirasi) cikarir.
func parseRooms(roomsHTML string) ([]provider.RoomInfo, []string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(roomsHTML))
	if err != nil {
		return nil, nil, fmt.Errorf("oda HTML ayristirilamadi: %w", err)
	}
	rooms := []provider.RoomInfo{}
	idx := map[string]int{}
	var boards []string
	doc.Find(".availability-item").Each(func(_ int, it *goquery.Selection) {
		name := cleanText(it.Find("h4.accommodation-type").First().Text())
		if name == "" {
			return
		}
		if b := cleanText(it.Find("h5.hotel-pension-type").First().Text()); b != "" {
			boards = append(boards, b)
		}
		if _, ok := idx[name]; ok {
			return
		}
		r := provider.RoomInfo{Name: name, Features: []string{}, Images: []string{}}
		r.Capacity = cleanText(it.Find(".room-capacity-text").First().Text())
		if href := it.Find("a.detail-link[href^='#']").First().AttrOr("href", ""); len(href) > 1 {
			pop := doc.Find("[id='" + href[1:] + "']").First()
			if pop.Length() > 0 {
				r.Description = strings.Join(htmlToLines(firstHTML(pop.Find(".room-check-list-box .check-list-description").First())), "\n")
				pop.Find(".room-check-list-box ul.check-list li").Each(func(_ int, li *goquery.Selection) {
					if t := cleanText(li.Text()); t != "" {
						r.Features = append(r.Features, t)
					}
				})
				pop.Find(".big-image-slider img[data-src]").Each(func(_ int, img *goquery.Selection) {
					if u := normalizeImageURL(img.AttrOr("data-src", "")); u != "" {
						r.Images = append(r.Images, u)
					}
				})
			}
		}
		if len(r.Images) == 0 {
			if u := normalizeImageURL(it.Find(".room-img-wrapper img[data-src]").First().AttrOr("data-src", "")); u != "" {
				r.Images = append(r.Images, u)
			}
		}
		r.Features = uniqueStrings(r.Features)
		r.Images = uniqueStrings(r.Images)
		if m := reRoomSize.FindStringSubmatch(r.Description); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil {
				r.Size = &n
			}
		}
		idx[name] = len(rooms)
		rooms = append(rooms, r)
	})
	return rooms, uniqueStrings(boards), nil
}

func firstHTML(s *goquery.Selection) string {
	if s.Length() == 0 {
		return ""
	}
	h, _ := s.Html()
	return h
}

// parseAutocomplete otomatik tamamlama HTML'inden yalnizca otel sonuclarini doner
// (<h3>KATEGORİLER</h3> altindaki "... Otelleri" bolge baglantilari atlanir).
func parseAutocomplete(html string) []provider.HotelCandidate {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil
	}
	out := []provider.HotelCandidate{}
	seen := map[string]bool{}
	section := ""
	doc.Find(".scroll-wrapper").First().Children().Each(func(_ int, s *goquery.Selection) {
		if goquery.NodeName(s) == "h3" {
			section = strings.ToUpper(cleanText(s.Text()))
			return
		}
		if strings.Contains(section, "KATEGOR") || !strings.Contains(section, "OTEL") {
			return
		}
		s.Find("a[data-link]").Each(func(_ int, a *goquery.Selection) {
			slug := strings.Trim(strings.TrimSpace(a.AttrOr("data-link", "")), "/")
			name := cleanText(a.Text())
			if slug == "" || name == "" || seen[slug] || strings.HasSuffix(slug, "-otelleri") {
				return
			}
			seen[slug] = true
			out = append(out, provider.HotelCandidate{Name: name, URL: siteRoot + "/" + slug, ProviderHotelID: slug})
		})
	})
	return out
}

var trFold = strings.NewReplacer("ç", "c", "Ç", "C", "ğ", "g", "Ğ", "G", "ı", "i", "İ", "I", "ö", "o", "Ö", "O", "ş", "s", "Ş", "S", "ü", "u", "Ü", "U")

// searchAttempts: tam sorgu, ASCII'ye indirgenmisi, sonra son kelime atilarak kisaltilmis halleri (en az 1 kelime, 3 harf).
func searchAttempts(q string) []string {
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if len([]rune(s)) < 3 {
			return
		}
		for _, o := range out {
			if strings.EqualFold(o, s) {
				return
			}
		}
		out = append(out, s)
	}
	add(q)
	add(trFold.Replace(q))
	words := strings.Fields(q)
	for n := len(words) - 1; n >= 1; n-- {
		add(strings.Join(words[:n], " "))
	}
	return out
}

func queryEscape(s string) string { return url.QueryEscape(s) }

// timeOf "15.00"/"15:00" metnini dogrular (test yardimcisi).
func timeOf(s string) string {
	if m := reTime.FindStringSubmatch(s); m != nil {
		return m[1] + ":" + m[2]
	}
	return ""
}
