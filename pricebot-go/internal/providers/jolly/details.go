package jolly

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"pricebot/internal/httpx"
	"pricebot/internal/provider"
)

// Statik otel detayi: detay sayfasi HTML'i (sezon sekmeleri #genel-bilgiler, galeri
// #detailSliderGallery, ld+json Hotel). Odalar GetReservationCompletePartial'dan sabit
// bir tarih penceresiyle alinir (bkz. staticStayWindow).

const siteBase = "https://www.jollytur.com"

var reLDHotel = regexp.MustCompile(`(?s)\{"@context":"https?://schema\.org","@type":"Hotel".*?\}\s*;?\s*</script>`)

type ldHotel struct {
	Name    string `json:"name"`
	Address *struct {
		StreetAddress   string `json:"streetAddress"`
		AddressLocality string `json:"addressLocality"`
		AddressRegion   string `json:"addressRegion"`
		AddressCountry  string `json:"addressCountry"`
	} `json:"address"`
	CheckinTime  string `json:"checkinTime"`
	CheckoutTime string `json:"checkoutTime"`
	Geo          *struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"geo"`
}

func parseLDHotel(html string) *ldHotel {
	m := reLDHotel.FindString(html)
	if m == "" {
		return nil
	}
	m = strings.TrimSpace(strings.TrimSuffix(m, "</script>"))
	m = strings.TrimSpace(strings.TrimSuffix(m, ";"))
	var out ldHotel
	if err := json.Unmarshal([]byte(m), &out); err != nil {
		return nil
	}
	return &out
}

func fetchDetailPage(ctx context.Context, u string) (string, error) {
	resp, err := httpx.Do(ctx, http.MethodGet, u, httpx.Options{Headers: pageHeaders, Timeout: 25 * time.Second})
	if err != nil {
		return "", err
	}
	return resp.Text(), nil
}

// GetDetails Jolly otel detayini ortak semaya cevirir.
func (p *Provider) GetDetails(ctx context.Context, m *provider.Mapping) (*provider.HotelDetails, error) {
	src := m.MetaString("sourceUrl")
	if src == "" {
		return nil, fmt.Errorf("Mapping metadata'da sourceUrl eksik (otel detay URL'i)")
	}
	html, err := fetchDetailPage(ctx, src)
	if err != nil {
		return nil, err
	}
	d, err := parseDetails(html, m.ProviderHotelID, src)
	if err != nil {
		return nil, err
	}
	if hotelType := m.MetaString("hotelType"); hotelType != "" && m.ProviderHotelID != "" {
		ci, co := staticStayWindow(time.Now())
		if data, err := fetchHotelReservation(ctx, m.ProviderHotelID, hotelType, ci, co, 2, nil, nil); err == nil && data != nil {
			d.Rooms = parseRooms(data.HTML)
		}
	}
	return d, nil
}

// parseDetails detay HTML'ini (odalar haric) normalize eder.
func parseDetails(html, hotelID, sourceURL string) (*provider.HotelDetails, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("Jolly sayfasi parse edilemedi: %w", err)
	}
	out := &provider.HotelDetails{
		Provider: "jolly", ProviderHotelID: hotelID, SourceURL: sourceURL, FetchedAt: time.Now().UTC(),
		Images:     []provider.Image{},
		Facilities: []provider.FacilityGroup{},
		Rooms:      []provider.RoomInfo{},
		FAQ:        []provider.QA{},
		Sections:   []provider.Section{},
	}
	out.Name = text(doc.Find("h1.title").First())
	if out.Name == "" {
		out.Name = provider.DecodeEntities(provider.FirstMatch(reSPName, html))
	}
	if out.Name == "" {
		return nil, fmt.Errorf("Jolly sayfasinda otel adi bulunamadi (otel detay URL'i olmali)")
	}
	if hotelID == "" {
		out.ProviderHotelID = provider.FirstMatch(reSPID, html)
	}

	pol := &provider.Policies{Other: []string{}}
	if ld := parseLDHotel(html); ld != nil {
		if ld.Address != nil {
			out.Address = cleanHTML(ld.Address.StreetAddress)
		}
		if ld.Geo != nil && ld.Geo.Latitude != 0 {
			out.Location = &provider.GeoPoint{Lat: ld.Geo.Latitude, Lng: ld.Geo.Longitude}
		}
		pol.CheckIn, pol.CheckOut = strings.TrimSpace(ld.CheckinTime), strings.TrimSpace(ld.CheckoutTime)
	}
	// Breadcrumb: Jolly > Kibris Otelleri > Girne Otelleri > Catalkoy Otelleri > Otel
	var crumbs []string
	doc.Find("#breadCrumbBar li a").Each(func(i int, a *goquery.Selection) {
		if i == 0 {
			return
		}
		if t := strings.TrimSpace(strings.TrimSuffix(text(a), " Otelleri")); t != "" {
			crumbs = append(crumbs, t)
		}
	})
	switch len(crumbs) {
	case 0:
	case 1:
		out.City = crumbs[0]
	case 2:
		out.Country, out.City = crumbs[0], crumbs[1]
	default:
		out.Country, out.City, out.Region = crumbs[0], crumbs[1], crumbs[len(crumbs)-1]
	}

	// Galeri (kampanya bannerlari haric)
	doc.Find("#detailSliderGallery a.item[href]").Each(func(_ int, a *goquery.Selection) {
		cat := provider.DecodeEntities(a.Find("img").AttrOr("data-category", ""))
		if strings.EqualFold(cat, "Kampanyalar") {
			return
		}
		u := stripQuery(absURL(a.AttrOr("href", "")))
		if u == "" || !strings.Contains(u, "/media/hotel/") {
			return
		}
		out.Images = append(out.Images, provider.Image{URL: u, Caption: cat})
	})
	out.Images = dedupeImages(out.Images)

	// Sezon sekmeleri
	seasons := map[string]string{}
	var seasonPairs []provider.KV
	doc.Find("#genel-bilgiler .tabs-item").Each(func(_ int, t *goquery.Selection) {
		idx := t.AttrOr("data-index", "")
		name := text(t.Find(".tabs-title"))
		dates := text(t.Find(".tabs-description"))
		if name != "" {
			seasons[idx] = name
			seasonPairs = append(seasonPairs, provider.KV{Key: name, Value: dates})
		}
	})
	if len(seasonPairs) > 0 {
		out.Sections = append(out.Sections, provider.Section{Title: "Sezonlar", Kind: "pairs", Pairs: seasonPairs})
	}
	tabs := doc.Find("#genel-bilgiler .tab-content.info-content-area")
	multi := tabs.Length() > 1
	tabs.Each(func(ti int, tab *goquery.Selection) {
		prefix := ""
		if multi {
			if s := seasons[tab.AttrOr("data-index", "")]; s != "" {
				prefix = s + " · "
			}
		}
		first := ti == 0
		parseSeasonTab(tab, prefix, first, out, pol)
	})

	// Etkinlikler (tum modal basliklari, sirali)
	var events []string
	doc.Find(".addon-modal .modal-title, .addon-slider-modal .modal-title").Each(func(_ int, h *goquery.Selection) {
		if t := text(h); t != "" {
			events = append(events, t)
		}
	})
	if events = uniq(events); len(events) > 0 {
		sortStrings(events)
		out.Sections = append(out.Sections, provider.Section{Title: "Etkinlikler", Kind: "list", Items: events})
	}

	if pol.CheckIn != "" || pol.CheckOut != "" || pol.PetPolicy != "" || pol.ChildPolicy != "" || len(pol.Other) > 0 {
		out.Policies = pol
	}
	return out, nil
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func dedupeImages(in []provider.Image) []provider.Image {
	seen := map[string]bool{}
	out := make([]provider.Image, 0, len(in))
	for _, im := range in {
		if im.URL == "" || seen[im.URL] {
			continue
		}
		seen[im.URL] = true
		out = append(out, im)
	}
	return out
}

// detailListItems ul.detail-list li -> "Ad (Ucretli)" listesi.
func detailListItems(ul *goquery.Selection) []string {
	var items []string
	ul.Find("li").Each(func(_ int, li *goquery.Selection) {
		label := text(li.Find("span").First())
		if label == "" {
			label = text(li)
		}
		if label == "" {
			return
		}
		if paid := strings.TrimSpace(li.Find("[data-content]").First().AttrOr("data-content", "")); paid != "" {
			label += " (" + paid + ")"
		}
		items = append(items, label)
	})
	return uniq(items)
}

// tagPairs .tag-list .tag-item -> (title, text); basliksiz olanlar (orn. "Evcil Hayvan Kabul Edilmez") list'e.
func tagPairs(sel *goquery.Selection) ([]provider.KV, []string) {
	var pairs []provider.KV
	var items []string
	sel.Find(".tag-item").Each(func(_ int, it *goquery.Selection) {
		k := text(it.Find(".title").First())
		if paid := strings.TrimSpace(it.Find(".title [data-content]").First().AttrOr("data-content", "")); paid != "" {
			k += " (" + paid + ")"
		}
		v := text(it.Find(".text").First())
		switch {
		case k != "" && v != "":
			pairs = append(pairs, provider.KV{Key: k, Value: v})
		case k != "":
			items = append(items, k)
		case v != "":
			items = append(items, v)
		}
	})
	return pairs, items
}

// articleGroups <strong>Baslik</strong><ul><li>..</li></ul> yapisini gruplara ayirir.
func articleGroups(article *goquery.Selection) []provider.FacilityGroup {
	var groups []provider.FacilityGroup
	cur := -1
	article.Find("strong, li").Each(func(_ int, n *goquery.Selection) {
		t := text(n)
		if t == "" {
			return
		}
		if goquery.NodeName(n) == "strong" {
			if n.ParentsFiltered("li").Length() > 0 {
				return
			}
			groups = append(groups, provider.FacilityGroup{Title: t, Items: []string{}})
			cur = len(groups) - 1
			return
		}
		if cur >= 0 {
			groups[cur].Items = append(groups[cur].Items, t)
		}
	})
	out := groups[:0]
	for _, g := range groups {
		g.Items = uniq(g.Items)
		if len(g.Items) > 0 {
			out = append(out, g)
		}
	}
	return out
}

// parseSeasonTab bir sezon sekmesindeki kutulari (hotel-deatil-box) ve yan paneli isler.
// first=true ise yapisal alanlar (aciklama, ozellikler, pansiyon, politikalar) da doldurulur.
func parseSeasonTab(tab *goquery.Selection, prefix string, first bool, out *provider.HotelDetails, pol *provider.Policies) {
	boards := map[string]bool{}
	tab.Find(".hotel-deatil-box").Each(func(_ int, box *goquery.Selection) {
		title := text(box.Find("header .title").First())
		if title == "" {
			return
		}
		content := box.Find(".content").First()
		switch {
		case content.Find("ul.detail-list").Length() > 0:
			items := detailListItems(content.Find("ul.detail-list").First())
			if len(items) == 0 {
				return
			}
			out.Sections = append(out.Sections, provider.Section{Title: prefix + title, Kind: "list", Items: items})
			if first {
				out.Facilities = append(out.Facilities, provider.FacilityGroup{Title: title, Items: items})
			}
		case content.Find(".content-item[data-concept]").Length() > 0:
			var pairs []provider.KV
			content.Find(".content-item[data-concept]").Each(func(_ int, ci *goquery.Selection) {
				board := text(ci.Find(".badge-title").First())
				if board != "" {
					boards[board] = true
				}
				ci.Find(".head-title").Each(func(_ int, ht *goquery.Selection) {
					place := text(ht)
					tl := ht.NextFiltered(".tag-list-content")
					ps, _ := tagPairs(tl)
					for _, kv := range ps {
						key := strings.Join(nonEmpty(board, place, kv.Key), " – ")
						pairs = append(pairs, provider.KV{Key: key, Value: kv.Value})
					}
				})
				if note := blockText(ci.Find("article").First()); note != "" {
					pairs = append(pairs, provider.KV{Key: board + " – Not", Value: note})
				}
			})
			if len(pairs) > 0 {
				out.Sections = append(out.Sections, provider.Section{Title: prefix + title, Kind: "pairs", Pairs: pairs})
			}
		default:
			pairs, items := tagPairs(content.ChildrenFiltered(".tag-list"))
			var texts []string
			content.ChildrenFiltered("article").Each(func(_ int, a *goquery.Selection) {
				if t := blockText(a); t != "" {
					texts = append(texts, t)
				}
			})
			// alt basliklar (orn. Toplanti Salonlari): .tag-list-content .head-title + article
			var subs []provider.Section
			content.Find(".tag-list-content .head-title").Each(func(_ int, ht *goquery.Selection) {
				st := text(ht)
				body := blockText(ht.Closest(".tag-list-content").NextFiltered("article"))
				if st != "" && body != "" {
					subs = append(subs, provider.Section{Title: prefix + title + " · " + st, Kind: "text", Text: body})
				}
			})
			groups := []provider.FacilityGroup{}
			if content.Find("article strong").Length() > 0 && content.Find("article li").Length() > 0 {
				groups = articleGroups(content.Find("article").First())
			}
			body := strings.Join(texts, "\n")
			if len(pairs) > 0 {
				out.Sections = append(out.Sections, provider.Section{Title: prefix + title, Kind: "pairs", Pairs: pairs, Items: items, Text: body})
			} else if body != "" || len(items) > 0 {
				kind := "text"
				if body == "" {
					kind = "list"
				}
				out.Sections = append(out.Sections, provider.Section{Title: prefix + title, Kind: kind, Text: body, Items: items})
			}
			out.Sections = append(out.Sections, subs...)
			if first {
				for _, g := range groups {
					out.Facilities = append(out.Facilities, provider.FacilityGroup{Title: title + " / " + g.Title, Items: g.Items})
				}
				if box.Is("#generalInfoBox") || strings.EqualFold(title, "Genel Bilgiler") {
					if out.Description == "" && len(texts) > 0 {
						out.Description = texts[0]
					}
				}
				if strings.Contains(strings.ToLower(title), "önemli") {
					for _, it := range items {
						if strings.Contains(strings.ToLower(it), "evcil") && pol.PetPolicy == "" {
							pol.PetPolicy = it
						}
					}
					for _, para := range strings.Split(body, "\n") {
						pol.Other = append(pol.Other, para)
					}
				}
			}
		}
	})

	// Yan panel
	tab.Find(".hotel-sidebar-detail").Each(func(_ int, sb *goquery.Selection) {
		if cc := sb.Find(".checkin-checkout"); cc.Length() > 0 {
			var pairs []provider.KV
			cc.Find(".item").Each(func(_ int, it *goquery.Selection) {
				k, v := text(it.Find(".title")), text(it.Find(".text"))
				if k != "" && v != "" {
					pairs = append(pairs, provider.KV{Key: k, Value: v})
					if first {
						switch strings.ToLower(k) {
						case "check-in":
							pol.CheckIn = v
						case "check-out":
							pol.CheckOut = v
						}
					}
				}
			})
			if len(pairs) > 0 {
				out.Sections = append(out.Sections, provider.Section{Title: prefix + "Giriş / Çıkış", Kind: "pairs", Pairs: pairs})
			}
			return
		}
		title := text(sb.Find(".title").First())
		if title == "" {
			return
		}
		if sb.Find(".addons-slider").Length() > 0 {
			return // etkinlikler ayrica toplaniyor
		}
		var texts, items []string
		sb.ChildrenFiltered("p").Each(func(_ int, pp *goquery.Selection) {
			if t := blockText(pp); t != "" {
				texts = append(texts, t)
			}
		})
		sb.ChildrenFiltered("ul").Each(func(_ int, ul *goquery.Selection) {
			sub := text(ul.Find(".subtitle").First())
			ul.Find("li").Each(func(_ int, li *goquery.Selection) {
				if li.Find(".subtitle").Length() > 0 {
					return
				}
				t := text(li)
				if t == "" {
					return
				}
				if sub != "" {
					t = sub + ": " + t
				}
				items = append(items, t)
			})
		})
		body := strings.Join(texts, "\n")
		if body == "" && len(items) == 0 {
			return
		}
		kind := "text"
		if body == "" {
			kind = "list"
		}
		out.Sections = append(out.Sections, provider.Section{Title: prefix + title, Kind: kind, Text: body, Items: uniq(items)})
	})

	if first && len(boards) > 0 {
		out.Boards = sortedKeys(boards)
	}
}

func nonEmpty(parts ...string) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

var reSize = regexp.MustCompile(`(\d+)\s*m[2²]`)

// parseRooms GetReservationCompletePartial HTML'indeki oda kartlarini (farkli pansiyonlarda
// tekrar eden odalar birlestirilir) ada gore sirali listeye cevirir.
func parseRooms(html string) []provider.RoomInfo {
	out := []provider.RoomInfo{}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return out
	}
	byName := map[string]*provider.RoomInfo{}
	doc.Find("[data-roomtype]").Each(func(_ int, card *goquery.Selection) {
		name := text(card.Find(".room-title").First())
		if name == "" {
			name = provider.DecodeEntities(card.AttrOr("data-roomtype", ""))
		}
		if name == "" {
			return
		}
		ri, ok := byName[name]
		if !ok {
			ri = &provider.RoomInfo{Name: name, Features: []string{}, Images: []string{}}
			byName[name] = ri
		}
		if ri.Description == "" {
			ri.Description = blockText(card.Find(".option-text").First())
		}
		card.Find(".room-badges .badge").Each(func(_ int, b *goquery.Selection) {
			t := text(b)
			if t == "" {
				return
			}
			if m := reSize.FindStringSubmatch(t); m != nil && ri.Size == nil {
				if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
					ri.Size = &n
				}
				return
			}
			ri.Features = append(ri.Features, t)
		})
		card.Find(".option-tag span").Each(func(_ int, s *goquery.Selection) {
			t := text(s)
			if t == "" {
				return
			}
			if paid := strings.TrimSpace(s.Find("[data-content]").AttrOr("data-content", "")); paid != "" {
				t += " (" + paid + ")"
			}
			ri.Features = append(ri.Features, t)
		})
		card.Find(".image-gallery a[href]").Each(func(_ int, a *goquery.Selection) {
			if u := stripQuery(absURL(a.AttrOr("href", ""))); u != "" {
				ri.Images = append(ri.Images, u)
			}
		})
		if len(ri.Images) == 0 {
			if u := stripQuery(absURL(card.Find(".info-image img").AttrOr("src", ""))); u != "" && !strings.Contains(u, "/theme/") {
				ri.Images = append(ri.Images, u)
			}
		}
	})
	for _, name := range sortedKeys(byName) {
		ri := byName[name]
		ri.Features = uniq(ri.Features)
		ri.Images = uniq(ri.Images)
		out = append(out, *ri)
	}
	return out
}

// ---- Isimle arama ----

type searchHit struct {
	AdjustName            string  `json:"adjustName"`
	CategoryName          string  `json:"categoryName"`
	TypeName              string  `json:"typeName"`
	ID                    float64 `json:"id"`
	Value                 string  `json:"value"`
	DestinationBreadCrumb string  `json:"destinationBreadCrumb"`
}

// Shared/Search planner tipleri: Kibris otelleri ayri bir indekste (CybrusHotelPlanner).
var searchPlanners = []string{"CybrusHotelPlanner", "HotelPlanner"}

func searchPlanner(ctx context.Context, key, planner string) ([]searchHit, error) {
	u := searchURL + "?" + url.Values{"key": {key}, "type": {planner}}.Encode()
	resp, err := httpx.Get(ctx, u, map[string]string{"user-agent": ua, "accept": "application/json", "x-requested-with": "XMLHttpRequest"}, 20*time.Second)
	if err != nil {
		return nil, err
	}
	var list []searchHit
	if err := json.Unmarshal(resp.Body, &list); err != nil {
		return nil, fmt.Errorf("Jolly arama yaniti parse edilemedi: %w", err)
	}
	return list, nil
}

func hotelCandidates(hits []searchHit, seen map[string]bool) []provider.HotelCandidate {
	var out []provider.HotelCandidate
	for _, h := range hits {
		if h.ID <= 0 || h.AdjustName == "" || !strings.EqualFold(h.TypeName, "Otel") {
			continue
		}
		id := strconv.FormatFloat(h.ID, 'f', 0, 64)
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, provider.HotelCandidate{
			Name:            provider.DecodeEntities(h.Value),
			URL:             absURL(h.AdjustName),
			City:            provider.DecodeEntities(h.DestinationBreadCrumb),
			ProviderHotelID: id,
		})
	}
	return out
}

// SearchHotels /Shared/Search?key=&type= ile (once Kibris sonra yurt ici otel indeksi)
// otel adaylarini doner; sonuc yoksa sorguyu kelime kelime kisaltarak tekrar dener.
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
		out := []provider.HotelCandidate{}
		seen := map[string]bool{}
		for _, planner := range searchPlanners {
			hits, err := searchPlanner(ctx, q, planner)
			if err != nil {
				lastErr = err
				continue
			}
			out = append(out, hotelCandidates(hits, seen)...)
		}
		if len(out) > 0 {
			return out, nil
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
