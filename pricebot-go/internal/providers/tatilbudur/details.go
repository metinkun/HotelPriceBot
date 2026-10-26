package tatilbudur

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"pricebot/internal/httpx"
	"pricebot/internal/provider"
)

// Statik otel detayi: detay sayfasi HTML'i (Akamai'yi gecen pageHeaders seti ile).
// Yapisal veri ld+json (WebPage.mainEntity Hotel), galeri #main-carousel, ozellikler
// .hotel-amenities-wrapper, bilgi kartlari .hd-info-card. Odalar calculate-room-price
// gorunumunden sabit bir tarih penceresiyle alinir (bkz. staticStayWindow).

const hotelSearchURL = base + "/hotel/search"

// XHR cagrilari icin Akamai'yi gecen header seti (sayfa header'larinin cors karsiligi).
var xhrHeaders = map[string]string{
	"accept":             "application/json, text/javascript, */*; q=0.01",
	"accept-language":    "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
	"user-agent":         ua,
	"sec-ch-ua":          `"Chromium";v="150", "Google Chrome";v="150", "Not;A=Brand";v="8"`,
	"sec-ch-ua-mobile":   "?0",
	"sec-ch-ua-platform": `"macOS"`,
	"sec-fetch-dest":     "empty",
	"sec-fetch-mode":     "cors",
	"sec-fetch-site":     "same-origin",
	"x-requested-with":   "XMLHttpRequest",
	"referer":            base + "/",
}

var reLDJSON = regexp.MustCompile(`(?s)<script type="application/ld\+json">\s*(\{.*?\})\s*</script>`)

type ldPage struct {
	MainEntity *struct {
		Name        string `json:"name"`
		Identifier  string `json:"identifier"`
		Description string `json:"description"`
		Telephone   string `json:"telephone"`
		CheckIn     string `json:"checkInTime"`
		CheckOut    string `json:"checkOutTime"`
		Address     *struct {
			StreetAddress   string `json:"streetAddress"`
			AddressLocality string `json:"addressLocality"`
			AddressRegion   string `json:"addressRegion"`
			AddressCountry  *struct {
				Name string `json:"name"`
			} `json:"addressCountry"`
		} `json:"address"`
		Geo *struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"geo"`
		StarRating *struct {
			RatingValue any `json:"ratingValue"`
		} `json:"starRating"`
	} `json:"mainEntity"`
}

func parseLD(html string) *ldPage {
	for _, m := range reLDJSON.FindAllStringSubmatch(html, -1) {
		var out ldPage
		if err := json.Unmarshal([]byte(m[1]), &out); err == nil && out.MainEntity != nil {
			return &out
		}
	}
	return nil
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

// GetDetails Tatilbudur otel detayini ortak semaya cevirir.
func (p *Provider) GetDetails(ctx context.Context, m *provider.Mapping) (*provider.HotelDetails, error) {
	src := m.MetaString("sourceUrl")
	if src == "" {
		return nil, fmt.Errorf("Mapping metadata'da sourceUrl eksik (otel detay URL'i)")
	}
	s, err := fetchTokenAndCookies(ctx, src)
	if err != nil {
		return nil, err
	}
	d, err := parseDetails(s.HTML, m.ProviderHotelID, src)
	if err != nil {
		return nil, err
	}
	if d.ProviderHotelID != "" {
		ci, co := staticStayWindow(time.Now())
		body := buildBaseForm(s.Token, d.ProviderHotelID, ci, co, 2, 0)
		if resp, err := httpx.Post(ctx, calcURL, []byte(body), priceHeaders(s.Token, src, s.Cookie), 30*time.Second); err == nil {
			if view, err := parseView(resp.Body); err == nil {
				rooms, boards, ids := parseRooms(view.View)
				// Oda detay modali ancak ayni oturumda fiyat hesaplandiktan sonra acilir.
				cookie := mergeCookies(s.Cookie, resp.SetCookies())
				for i := range rooms {
					enrichRoom(ctx, &rooms[i], ids[rooms[i].Name], d.ProviderHotelID, src, s.Token, cookie)
				}
				d.Rooms = rooms
				if len(d.Boards) == 0 {
					d.Boards = boards
				}
			}
		}
	}
	return d, nil
}

// mergeCookies "a=1; b=2" listelerini birlestirir; sonraki ayni adli cerezi ezer.
func mergeCookies(lists ...string) string {
	order := []string{}
	vals := map[string]string{}
	for _, l := range lists {
		for _, part := range strings.Split(l, ";") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			name := strings.SplitN(part, "=", 2)[0]
			if _, ok := vals[name]; !ok {
				order = append(order, name)
			}
			vals[name] = part
		}
	}
	out := make([]string, 0, len(order))
	for _, n := range order {
		out = append(out, vals[n])
	}
	return strings.Join(out, "; ")
}

// enrichRoom GET /room-detail-modal/<hotelId>/<roomId>: oda ozellikleri (" *" = ucretli) ve galerisi.
func enrichRoom(ctx context.Context, ri *provider.RoomInfo, rid, hotelID, detailURL, token, cookie string) {
	if rid == "" {
		return
	}
	headers := priceHeaders(token, detailURL, cookie)
	headers["accept"] = "text/html, */*; q=0.01"
	delete(headers, "content-type")
	resp, err := httpx.Get(ctx, base+"/room-detail-modal/"+hotelID+"/"+rid, headers, 20*time.Second)
	if err != nil {
		return
	}
	applyRoomModal(ri, resp.Text())
}

// applyRoomModal oda modali HTML'inden ozellik ve gorselleri odaya yazar (JSON hata yaniti yok sayilir).
func applyRoomModal(ri *provider.RoomInfo, html string) {
	if !strings.Contains(html, "room-title") {
		return
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return
	}
	var feats []string
	doc.Find(".room-property li").Each(func(_ int, li *goquery.Selection) {
		t := text(li)
		if t == "" {
			return
		}
		if strings.HasSuffix(t, "*") {
			t = strings.TrimSpace(strings.TrimSuffix(t, "*")) + " (Ücretli)"
		}
		feats = append(feats, t)
	})
	if len(feats) > 0 {
		ri.Features = uniq(feats)
	}
	var imgs []string
	doc.Find(".hotel-images-2 img").Each(func(_ int, img *goquery.Selection) {
		if u := stripQuery(absURL(firstNonEmpty(img.AttrOr("data-src", ""), img.AttrOr("src", "")))); u != "" && !strings.Contains(u, "/themes/") {
			imgs = append(imgs, u)
		}
	})
	if len(imgs) > 0 {
		ri.Images = uniq(imgs)
	}
}

var (
	reImageID = regexp.MustCompile(`_(\d+)\.(?:jpg|jpeg|png|webp)$`)
	reRating  = regexp.MustCompile(`(\d+(?:[.,]\d+)?)\s*/\s*10`)
	reLeadInt = regexp.MustCompile(`^\s*(\d+)`)
)

// parseDetails detay HTML'ini (odalar haric) normalize eder.
func parseDetails(html, hotelID, sourceURL string) (*provider.HotelDetails, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("Tatilbudur sayfasi parse edilemedi: %w", err)
	}
	out := &provider.HotelDetails{
		Provider: "tatilbudur", ProviderHotelID: hotelID, SourceURL: sourceURL, FetchedAt: time.Now().UTC(),
		Images:     []provider.Image{},
		Facilities: []provider.FacilityGroup{},
		Rooms:      []provider.RoomInfo{},
		FAQ:        []provider.QA{},
		Sections:   []provider.Section{},
	}
	if out.ProviderHotelID == "" {
		out.ProviderHotelID = provider.FirstMatch(reHotelID, html)
	}
	out.Name = text(doc.Find("h1.hotel-name").First())
	pol := &provider.Policies{Other: []string{}}
	if ld := parseLD(html); ld != nil {
		e := ld.MainEntity
		if out.Name == "" {
			out.Name = provider.DecodeEntities(e.Name)
		}
		if e.Address != nil {
			out.Address = provider.DecodeEntities(e.Address.StreetAddress)
			out.City = provider.DecodeEntities(e.Address.AddressRegion)
			out.Region = provider.DecodeEntities(e.Address.AddressLocality)
			if e.Address.AddressCountry != nil {
				out.Country = provider.DecodeEntities(e.Address.AddressCountry.Name)
			}
		}
		if e.Geo != nil && e.Geo.Latitude != 0 {
			out.Location = &provider.GeoPoint{Lat: e.Geo.Latitude, Lng: e.Geo.Longitude}
		}
		if e.StarRating != nil {
			out.Stars = anyToInt(e.StarRating.RatingValue)
		}
		if tel := strings.TrimSpace(e.Telephone); tel != "" {
			out.Contact = &provider.Contact{Phone: tel}
		}
		pol.CheckIn, pol.CheckOut = strings.TrimSpace(e.CheckIn), strings.TrimSpace(e.CheckOut)
	}
	if out.Name == "" {
		return nil, fmt.Errorf("Tatilbudur sayfasinda otel adi bulunamadi (otel detay URL'i olmali)")
	}
	if addr := text(doc.Find("#location-information .address").First()); addr != "" && out.Address == "" {
		out.Address = addr
	}

	// Misafir puani: "7.7/10", "407 Onayli Misafir Yorumu", kategori puanlari
	if m := reRating.FindStringSubmatch(text(doc.Find(".comments-number").First())); m != nil {
		if f, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64); err == nil && f > 0 {
			out.Rating = &f
		}
	}
	if m := reLeadInt.FindStringSubmatch(text(doc.Find(".comment-detail-title li").First())); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			out.RatingCount = &n
		}
	}
	if out.Rating != nil {
		pairs := []provider.KV{{Key: "Genel", Value: strconv.FormatFloat(*out.Rating, 'f', 1, 64)}}
		doc.Find(".comments-detail-number").Each(func(_ int, n *goquery.Selection) {
			v := text(n.Find("span").First())
			k := strings.TrimSpace(strings.TrimSuffix(text(n), v))
			if k != "" && v != "" {
				pairs = append(pairs, provider.KV{Key: k, Value: v})
			}
		})
		if out.RatingCount != nil {
			pairs = append(pairs, provider.KV{Key: "Yorum Sayısı", Value: strconv.Itoa(*out.RatingCount)})
		}
		out.Sections = append(out.Sections, provider.Section{Title: "Misafir Değerlendirmesi", Kind: "pairs", Pairs: pairs})
	}

	// Galeri: kategori adlari filtre listesinden, gorseller ana carousel + (ek olarak) modal carousel
	catNames := map[string]string{}
	doc.Find(".hotel__modal__filter li[data-filter]").Each(func(_ int, li *goquery.Selection) {
		k := li.AttrOr("data-filter", "")
		name := text(li.Find("a").First())
		if i := strings.LastIndex(name, " "); i > 0 && strings.Trim(name[i+1:], "0123456789") == "" {
			name = name[:i]
		}
		if k != "" && name != "" {
			catNames[k] = strings.TrimSpace(name)
		}
	})
	seenImg := map[string]bool{}
	addImg := func(a *goquery.Selection, src string) {
		u := stripQuery(absURL(src))
		if u == "" || strings.Contains(u, "/themes/") {
			return
		}
		key := u
		if m := reImageID.FindStringSubmatch(u); m != nil {
			key = m[1]
		}
		if seenImg[key] {
			return
		}
		seenImg[key] = true
		cap := ""
		for _, c := range strings.Fields(a.AttrOr("class", "")) {
			if strings.HasPrefix(c, "galleryCategory_") {
				cap = catNames[c]
			}
		}
		out.Images = append(out.Images, provider.Image{URL: u, Caption: cap})
	}
	doc.Find("#main-carousel a.carousel__slide").Each(func(_ int, a *goquery.Selection) {
		img := a.Find("img").First()
		addImg(a, firstNonEmpty(img.AttrOr("data-src", ""), img.AttrOr("src", "")))
	})
	doc.Find(".hotel_room_modal a.carousel__slide").Each(func(_ int, a *goquery.Selection) {
		img := a.Find("img").First()
		addImg(a, firstNonEmpty(img.AttrOr("data-src", ""), img.AttrOr("src", "")))
	})

	// Imkan ve ozellikler: sekme > alt grup > madde (+ Ucretli/Ucretsiz rozeti)
	var allItems []string
	doc.Find(".hotel-amenities-wrapper .hf-inner").Each(func(_ int, tab *goquery.Selection) {
		tabTitle := text(tab.Find(".hf-tab-title-text").First())
		if tabTitle == "" {
			return
		}
		tab.Find(".hf-subgroup").Each(func(_ int, sg *goquery.Selection) {
			sub := text(sg.Find(".hf-subgroup-title").First())
			var items []string
			sg.Find(".hf-item-row").Each(func(_ int, row *goquery.Selection) {
				it := text(row.Find(".hf-item-text").First())
				if it == "" {
					return
				}
				if badge := text(row.Find(".hf-item-badge .text").First()); badge != "" {
					it += " (" + badge + ")"
				}
				items = append(items, it)
			})
			if items = uniq(items); len(items) == 0 {
				return
			}
			title := tabTitle
			if sub != "" && !strings.EqualFold(sub, tabTitle) {
				title = tabTitle + " / " + sub
			}
			out.Facilities = append(out.Facilities, provider.FacilityGroup{Title: title, Items: items})
			allItems = append(allItems, items...)
		})
		if desc := blockText(tab.Find(".hf-tab-desc-text").First()); desc != "" {
			out.Sections = append(out.Sections, provider.Section{Title: tabTitle, Kind: "text", Text: desc})
			lt := strings.ToLower(tabTitle)
			if strings.Contains(lt, "çocuk") && pol.ChildPolicy == "" {
				pol.ChildPolicy = desc
			}
		}
	})
	if hl := listItems(doc.Find(".highlights .hotel-properties li")); len(hl) > 0 {
		out.Facilities = append([]provider.FacilityGroup{{Title: "Öne Çıkan Özellikler", Items: hl}}, out.Facilities...)
	}
	for _, it := range allItems {
		if strings.Contains(strings.ToLower(it), "evcil") && pol.PetPolicy == "" {
			pol.PetPolicy = it
		}
	}

	// Bilgi kartlari (Konsept Ozellikleri, Onemli Notlar)
	doc.Find(".hd-info-card").Each(func(_ int, card *goquery.Selection) {
		title := text(card.Find(".hd-info-title").First())
		body := blockText(card.Find(".hd-info-desc").First())
		if title == "" || body == "" {
			return
		}
		out.Sections = append(out.Sections, provider.Section{Title: title, Kind: "text", Text: body})
		lt := strings.ToLower(title)
		if strings.Contains(lt, "önemli") {
			for _, para := range strings.Split(body, "\n") {
				pol.Other = append(pol.Other, para)
			}
		}
		if strings.Contains(lt, "konsept") && out.Description == "" {
			out.Description = body
		}
	})

	// Konum bilgileri
	var loc []provider.KV
	doc.Find("#distance-items .location-item").Each(func(_ int, it *goquery.Selection) {
		k := strings.TrimSuffix(text(it.Find("b").First()), ":")
		v := strings.TrimSpace(strings.TrimPrefix(text(it), text(it.Find("b").First())))
		if k != "" && v != "" {
			loc = append(loc, provider.KV{Key: k, Value: v})
		}
	})
	if len(loc) > 0 {
		out.Sections = append(out.Sections, provider.Section{Title: "Konum Bilgileri", Kind: "pairs", Pairs: loc})
	}

	// Etkinlikler
	var events []string
	doc.Find(".activity-card").Each(func(_ int, c *goquery.Selection) {
		t := text(c.Find(".activity-card-title").First())
		if t == "" {
			return
		}
		if d := text(c.Find(".activity-card-date").First()); d != "" {
			t += " (" + d + ")"
		}
		events = append(events, t)
	})
	if events = uniq(events); len(events) > 0 {
		out.Sections = append(out.Sections, provider.Section{Title: "Etkinlikler", Kind: "list", Items: events})
	}

	// KKTC giris kurallari (Kibris otellerinde modal)
	if body := blockText(doc.Find("#cyprusInfoModal .modal-dialog .modal-dialog").First()); body != "" {
		out.Sections = append(out.Sections, provider.Section{Title: "KKTC Giriş Kuralları", Kind: "text", Text: body})
	}

	if pol.CheckIn != "" || pol.CheckOut != "" || pol.PetPolicy != "" || pol.ChildPolicy != "" || len(pol.Other) > 0 {
		out.Policies = pol
	}
	return out, nil
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func listItems(sel *goquery.Selection) []string {
	var items []string
	sel.Each(func(_ int, li *goquery.Selection) {
		if t := text(li); t != "" {
			items = append(items, t)
		}
	})
	return uniq(items)
}

// parseRooms calculate-room-price gorunumundeki oda kartlarini ada gore sirali listeye cevirir;
// pansiyon tipleri (meal-type-desc) ve oda adi -> roomTypeId eslemesi ayrica doner.
func parseRooms(view string) ([]provider.RoomInfo, []string, map[string]string) {
	out := []provider.RoomInfo{}
	ids := map[string]string{}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(view))
	if err != nil {
		return out, nil, ids
	}
	boards := map[string]bool{}
	byName := map[string]*provider.RoomInfo{}
	doc.Find(".room-type-new").Each(func(_ int, card *goquery.Selection) {
		name := text(card.Find("h3.room-type-title").First())
		if name == "" {
			return
		}
		if id := strings.TrimSpace(card.AttrOr("id", "")); id != "" && ids[name] == "" {
			ids[name] = id
		}
		ri, ok := byName[name]
		if !ok {
			ri = &provider.RoomInfo{Name: name, Features: []string{}, Images: []string{}}
			byName[name] = ri
		}
		card.Find(".room-gallery img").Each(func(_ int, img *goquery.Selection) {
			if u := stripQuery(absURL(img.AttrOr("data-src", ""))); u != "" && !strings.Contains(u, "/themes/") {
				ri.Images = append(ri.Images, u)
			}
		})
		card.Find(".features-check-list li span").Each(func(_ int, s *goquery.Selection) {
			if t := text(s); t != "" {
				ri.Features = append(ri.Features, t)
			}
		})
		card.Find(".meal-type-desc").Each(func(_ int, s *goquery.Selection) {
			if t := text(s); t != "" {
				boards[t] = true
			}
		})
	})
	for _, name := range sortedKeys(byName) {
		ri := byName[name]
		ri.Features = uniq(ri.Features)
		ri.Images = uniq(ri.Images)
		if len(ri.Images) > 1 {
			ri.Images = ri.Images[:1] // ayni gorselin farkli boyutlari
		}
		out = append(out, *ri)
	}
	return out, sortedKeys(boards), ids
}

// ---- Isimle arama ----

type searchHit struct {
	Value   string  `json:"value"`
	ID      float64 `json:"id"`
	Type    string  `json:"type"`
	URL     string  `json:"url"`
	Country string  `json:"country"`
	City    string  `json:"city"`
	County  string  `json:"county"`
}

func searchTerm(ctx context.Context, term string) ([]searchHit, error) {
	u := hotelSearchURL + "?term=" + strings.ReplaceAll(strings.TrimSpace(term), " ", "%20")
	resp, err := httpx.Get(ctx, u, xhrHeaders, 20*time.Second)
	if err != nil {
		return nil, err
	}
	var list []searchHit
	if err := json.Unmarshal(resp.Body, &list); err != nil {
		return nil, fmt.Errorf("Tatilbudur arama yaniti parse edilemedi (Akamai engeli olabilir): %w", err)
	}
	return list, nil
}

func hotelCandidates(hits []searchHit) []provider.HotelCandidate {
	out := []provider.HotelCandidate{}
	seen := map[string]bool{}
	for _, h := range hits {
		if !strings.EqualFold(h.Type, "Hotel") || h.URL == "" || seen[h.URL] {
			continue
		}
		seen[h.URL] = true
		city := provider.DecodeEntities(h.City)
		if c := provider.DecodeEntities(h.County); c != "" && !strings.EqualFold(c, city) {
			city = strings.TrimSpace(strings.Trim(c+", "+city, ", "))
		}
		id := ""
		if h.ID > 0 {
			id = strconv.FormatFloat(h.ID, 'f', 0, 64)
		}
		out = append(out, provider.HotelCandidate{
			Name: provider.DecodeEntities(h.Value), URL: absURL(h.URL), City: city, ProviderHotelID: id,
		})
	}
	return out
}

// SearchHotels GET /hotel/search?term= ile otel adaylarini doner; sonuc yoksa sorguyu
// kelime kelime kisaltarak tekrar dener.
func (p *Provider) SearchHotels(ctx context.Context, query string) ([]provider.HotelCandidate, error) {
	words := strings.Fields(strings.TrimSpace(query))
	if len(words) == 0 {
		return nil, fmt.Errorf("Arama metni bos")
	}
	var lastErr error
	for n := len(words); n >= 1; n-- {
		q := strings.Join(words[:n], " ")
		if len([]rune(q)) < 2 {
			break
		}
		hits, err := searchTerm(ctx, q)
		if err != nil {
			lastErr = err
			continue
		}
		if c := hotelCandidates(hits); len(c) > 0 {
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
