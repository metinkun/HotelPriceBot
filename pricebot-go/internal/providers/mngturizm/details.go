package mngturizm

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

// Statik detay: otel sayfasi (sunucu tarafli HTML) + fotograf galerisi (index.php?s=fotograflar).
//   - Sayfa: h1.title, .bread (Kibris > Girne), data-koords, .otelpuan .point (5 uzerinden puan),
//     .one-cikan-ozellik (Tesis Ozellikleri), #otel-hakkinda (strong basliklar + p/ul), .hncText (SSS),
//     #otel-ekstra .kampanyadetay (kampanyalar), #otel-etkinlik (etkinlikler), .cyas (cocuk).
//   - Galeri: POST index.php?s=fotograflar  data=<#resim_value>&islem=liste (kategoriler) / tumu (tum resimler).
//     Referer kurali XHR icin gecerli (Cloudflare); sayfa GET icin gerekmez.
// Odalar yalnizca tarihli otelprice yanitinda (musaitlige bagli) -> statik detayda yok.

const (
	fotograflarURL = indexURL + "?s=fotograflar"
	searchJSONURL  = baseURL + "/searchjson.php"
	hotelSearchURL = indexURL + "?s=hotelsearch"
)

type galleryCategory struct {
	Kategori string `json:"kategori"`
	Toplam   int    `json:"toplam"`
	ID       any    `json:"id"` // sayi veya "video"
}

type galleryPhoto struct {
	Img      string `json:"img"`
	Kategori any    `json:"kategori"`
}

type galleryResponse[T any] struct {
	Success  bool   `json:"success"`
	Hata     string `json:"hata"`
	Sonuclar []T    `json:"sonuclar"`
}

// detailData statik detay icin toplanan ham veri (testlerde fixture'dan doldurulur).
type detailData struct {
	PageHTML   string
	Categories []galleryCategory
	Photos     []galleryPhoto
}

func fetchGallery[T any](ctx context.Context, resimValue, islem, referer string) ([]T, error) {
	form := httpx.FormValues{{"data", resimValue}, {"islem", islem}}
	resp, err := httpx.Post(ctx, fotograflarURL, []byte(form.Encode()), xhrHeaders(referer), 25*time.Second)
	if err != nil {
		return nil, err
	}
	var out galleryResponse[T]
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("MNG Turizm galeri yaniti parse edilemedi: %w", err)
	}
	if !out.Success {
		return nil, fmt.Errorf("MNG Turizm galeri hatasi: %s", out.Hata)
	}
	return out.Sonuclar, nil
}

var reResimValue = regexp.MustCompile(`id="resim_value"\s+value="([^"]*)"`)

// GetDetails otel sayfasini ve galeriyi cekip ortak semaya cevirir.
func (p *Provider) GetDetails(ctx context.Context, m *provider.Mapping) (*provider.HotelDetails, error) {
	u := m.MetaString("sourceUrl")
	if u == "" {
		return nil, fmt.Errorf("Mapping metadata'da sourceUrl eksik (MNG Turizm otel sayfasi URL'i)")
	}
	resp, err := httpx.Do(ctx, http.MethodGet, u, httpx.Options{Headers: pageHeaders, Timeout: 25 * time.Second})
	if err != nil {
		return nil, err
	}
	html := resp.Text()
	if provider.FirstMatch(reRezOtelID, html) == "" && provider.FirstMatch(reDataID, html) == "" {
		return nil, fmt.Errorf("MNG Turizm sayfasinda otel bulunamadi (otel detay URL'i olmali)")
	}
	d := &detailData{PageHTML: html}
	// Galeri: hata olursa sayfadaki gorsellerle devam edilir.
	if rv := provider.FirstMatch(reResimValue, html); rv != "" {
		if cats, err := fetchGallery[galleryCategory](ctx, rv, "liste", u); err == nil {
			d.Categories = cats
		}
		if photos, err := fetchGallery[galleryPhoto](ctx, rv, "tumu", u); err == nil {
			d.Photos = photos
		}
	}
	out, err := normalizeDetails(d)
	if err != nil {
		return nil, err
	}
	out.ProviderHotelID = m.ProviderHotelID
	if out.ProviderHotelID == "" {
		out.ProviderHotelID = provider.FirstMatch(reRezOtelID, html)
	}
	out.SourceURL = u
	return out, nil
}

var (
	reKoords   = regexp.MustCompile(`^\s*(-?\d+(?:\.\d+)?)\s*,\s*(-?\d+(?:\.\d+)?)\s*$`)
	reCheckIn  = regexp.MustCompile(`(?i)giri[sş] saati[^0-9]{0,40}?(\d{1,2}[:.]\d{2})`)
	reCheckOut = regexp.MustCompile(`(?i)[cç][iı]k[iı][sş] saati[^0-9]{0,40}?(\d{1,2}[:.]\d{2})`)
)

func normalizeDetails(d *detailData) (*provider.HotelDetails, error) {
	// Sayfa HTML'i satir sonlariyla bicimlendirilmis; metin dugumlerindeki satir sonlari bosluga cevrilir.
	flat := strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(d.PageHTML)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(flat))
	if err != nil {
		return nil, err
	}
	out := &provider.HotelDetails{
		Provider:   "mngturizm",
		Images:     []provider.Image{},
		Facilities: []provider.FacilityGroup{},
		Rooms:      []provider.RoomInfo{},
		FAQ:        []provider.QA{},
		Sections:   []provider.Section{},
	}
	out.Name = cleanText(doc.Find("h1.title").First().Text())
	if out.Name == "" {
		out.Name = cleanText(provider.FirstMatch(reProdName, d.PageHTML))
	}

	// Konum: breadcrumb (Kibris > Girne) ve .yer metni.
	crumbs := []string{}
	doc.Find(".bread a").Each(func(_ int, s *goquery.Selection) {
		if t := cleanText(s.Text()); t != "" {
			crumbs = append(crumbs, t)
		}
	})
	switch {
	case len(crumbs) >= 2:
		out.Region, out.City = crumbs[0], crumbs[len(crumbs)-1]
	case len(crumbs) == 1:
		out.City = crumbs[0]
	}
	if out.City == "" {
		out.City = cleanText(doc.Find(".tour-infos .yer > span").First().Text())
	}
	if k, ok := doc.Find(".haritabtn a[data-koords]").First().Attr("data-koords"); ok {
		if m := reKoords.FindStringSubmatch(k); m != nil {
			lat, _ := strconv.ParseFloat(m[1], 64)
			lng, _ := strconv.ParseFloat(m[2], 64)
			if lat != 0 && lng != 0 {
				out.Location = &provider.GeoPoint{Lat: lat, Lng: lng}
			}
		}
	}

	// Puan (5 uzerinden) -> 10 uzerinden.
	if pt := cleanText(doc.Find(".otelpuan .point").First().Text()); pt != "" {
		if v, err := strconv.ParseFloat(strings.ReplaceAll(pt, ",", "."), 64); err == nil && v > 0 {
			r := v * 2
			out.Rating = &r
		}
	}

	// Kisa aciklama: baslik altindaki ilk paragraf.
	out.Description = cleanText(doc.Find(".tour-infos .col-md-9 > div > p").First().Text())

	// Tesis Ozellikleri (one cikan).
	items := []string{}
	doc.Find(".one-cikan-ozellik span").Each(func(_ int, s *goquery.Selection) {
		if t := cleanText(s.Text()); t != "" {
			items = append(items, t)
		}
	})
	if len(items) > 0 {
		out.Facilities = append(out.Facilities, provider.FacilityGroup{Title: "Tesis Özellikleri", Items: items})
	}

	pol := &provider.Policies{}
	if c := cleanText(doc.Find(".cyas").First().Text()); c != "" {
		pol.ChildPolicy = c
	}

	// Tesis Bilgileri: <p><strong>Baslik</strong></p> + p/ul bloklari; .hncText = SSS.
	var cur *provider.Section
	flush := func() {
		if cur == nil {
			return
		}
		if cur.Text != "" || len(cur.Items) > 0 {
			if cur.Text == "" {
				cur.Kind = "list"
			} else {
				cur.Kind = "text"
				if len(cur.Items) > 0 {
					cur.Text += "\n- " + strings.Join(cur.Items, "\n- ")
					cur.Items = nil
				}
			}
			out.Sections = append(out.Sections, *cur)
			if (cur.Title == "Ücretsiz Hizmetler" || cur.Title == "Ücretli Hizmetler") && cur.Kind == "list" {
				out.Facilities = append(out.Facilities, provider.FacilityGroup{Title: cur.Title, Items: cur.Items})
			}
		}
		cur = nil
	}
	infoText := strings.Builder{}
	doc.Find("#otel-hakkinda .tour-content-box").First().Children().Each(func(_ int, s *goquery.Selection) {
		switch {
		case s.HasClass("title"), s.HasClass("tour-tags"):
			flush()
		case s.HasClass("hncText"):
			flush()
			var q string
			s.Children().Each(func(_ int, c *goquery.Selection) {
				t := cleanText(c.Text())
				if goquery.NodeName(c) == "span" {
					q = t
				} else if goquery.NodeName(c) == "p" && q != "" && t != "" {
					out.FAQ = append(out.FAQ, provider.QA{Question: q, Answer: t})
					q = ""
				}
			})
		case goquery.NodeName(s) == "p":
			strong := s.ChildrenFiltered("strong")
			if strong.Length() == 1 && cleanText(strong.Text()) == cleanText(s.Text()) {
				flush()
				cur = &provider.Section{Title: cleanText(strong.Text())}
				return
			}
			t := cleanText(s.Text())
			if t == "" {
				return
			}
			infoText.WriteString(t + "\n")
			if cur == nil {
				cur = &provider.Section{Title: "Tesis Bilgileri"}
			}
			if cur.Text != "" {
				cur.Text += "\n"
			}
			cur.Text += t
		case goquery.NodeName(s) == "ul":
			if cur == nil {
				cur = &provider.Section{Title: "Tesis Bilgileri"}
			}
			s.Find("li").Each(func(_ int, li *goquery.Selection) {
				if t := cleanText(li.Text()); t != "" {
					cur.Items = append(cur.Items, t)
					infoText.WriteString(t + "\n")
				}
			})
		}
	})
	flush()

	all := infoText.String()
	if m := reCheckIn.FindStringSubmatch(all); m != nil {
		pol.CheckIn = m[1]
	}
	if m := reCheckOut.FindStringSubmatch(all); m != nil {
		pol.CheckOut = m[1]
	}
	for _, line := range strings.Split(all, "\n") {
		for _, s := range strings.SplitAfter(line, ".") {
			if strings.Contains(strings.ToLower(s), "evcil hayvan") {
				pol.PetPolicy = strings.TrimSpace(s)
				break
			}
		}
		if pol.PetPolicy != "" {
			break
		}
	}

	// Kampanyalar
	camp := []provider.KV{}
	doc.Find("#otel-ekstra .kampanyadetay").Each(func(_ int, s *goquery.Selection) {
		title := cleanText(s.Find(".baslik").First().Text())
		s.Find(".baslik").Remove()
		body := blockText(s)
		if title == "" && body == "" {
			return
		}
		camp = append(camp, provider.KV{Key: title, Value: body})
		if strings.Contains(strings.ToLower(title), "iptal") && pol.Cancellation == "" {
			pol.Cancellation = body
		}
	})
	if len(camp) > 0 {
		out.Sections = append(out.Sections, provider.Section{Title: "Kampanya ve Fırsatlar", Kind: "pairs", Pairs: camp})
	}

	// Etkinlikler
	events := []string{}
	doc.Find("#otel-etkinlik .oteldetay-etkinlik").Each(func(_ int, s *goquery.Selection) {
		det := s.Find(".detay")
		name := cleanText(det.Find("h4").First().Text())
		date := cleanText(det.ChildrenFiltered("div").First().Text())
		det.Find("h4").Remove()
		det.ChildrenFiltered("div").First().Remove()
		body := blockText(det)
		line := name
		if date != "" {
			line += " — " + date
		}
		if body != "" {
			line += ": " + body
		}
		if strings.TrimSpace(line) != "" {
			events = append(events, line)
		}
	})
	if len(events) > 0 {
		out.Sections = append(out.Sections, provider.Section{Title: "Etkinlikler", Kind: "list", Items: events})
	}

	// Gorseller: galeri (kategori adiyla); yoksa sayfadaki grid.
	catName := map[string]string{}
	for _, c := range d.Categories {
		catName[fmt.Sprint(c.ID)] = cleanText(c.Kategori)
	}
	seen := map[string]bool{}
	for _, ph := range d.Photos {
		u := absImage(ph.Img)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out.Images = append(out.Images, provider.Image{URL: u, Caption: catName[fmt.Sprint(ph.Kategori)]})
	}
	if len(out.Images) == 0 {
		doc.Find(".galleryGrid img").Each(func(_ int, s *goquery.Selection) {
			src, _ := s.Attr("data-original")
			if src == "" {
				src, _ = s.Attr("src")
			}
			u := absImage(src)
			if u == "" || seen[u] || strings.Contains(u, "liste-preview") {
				return
			}
			seen[u] = true
			out.Images = append(out.Images, provider.Image{URL: u})
		})
	}

	if pol.CheckIn != "" || pol.CheckOut != "" || pol.ChildPolicy != "" || pol.PetPolicy != "" || pol.Cancellation != "" {
		out.Policies = pol
	}
	return out, nil
}

var reWS = regexp.MustCompile(`[\s\x{00a0}]+`)

func cleanText(s string) string {
	return strings.TrimSpace(reWS.ReplaceAllString(provider.DecodeEntities(s), " "))
}

// blockText secimin icerigini satir bazli duz metne cevirir (p/li/div/br -> satir).
func blockText(s *goquery.Selection) string {
	s.Find("script,style,img").Remove()
	s.Find("br").Each(func(_ int, b *goquery.Selection) { b.ReplaceWithHtml("\n") })
	s.Find("p,div,li,h1,h2,h3,h4,h5,h6,ul,ol,tr").Each(func(_ int, b *goquery.Selection) {
		b.PrependHtml("\n")
		b.AppendHtml("\n")
	})
	lines := strings.Split(s.Text(), "\n")
	outLines := make([]string, 0, len(lines))
	for _, l := range lines {
		if t := cleanText(l); t != "" {
			outLines = append(outLines, t)
		}
	}
	return strings.Join(outLines, "\n")
}

// absImage "/images/hotel/5729_t.jpg" -> tam URL; cache-buster sorgusu atilir.
func absImage(raw string) string {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, `\/`, "/"))
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	} else if strings.HasPrefix(raw, "/") {
		raw = baseURL + raw
	} else if !strings.HasPrefix(raw, "http") {
		raw = baseURL + "/" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// ---- Isimle arama ----

// searchItem searchjson.php satiri (ana sayfa autocomplete: il/ilce/otel/otelkat/semt).
type searchItem struct {
	Label      string `json:"label"`
	Category   string `json:"category"` // "Otel" | "İl" | ...
	CategoryID string `json:"categoryid"`
}

func searchJSON(ctx context.Context, q string) ([]searchItem, error) {
	v := url.Values{"q": {q}, "il": {"1"}, "ilce": {"1"}, "otel": {"1"}, "otelkat": {"1"}, "semt": {"1"}}
	h := xhrHeaders(baseURL + "/")
	delete(h, "content-type")
	h["accept"] = "application/json, text/javascript, */*; q=0.01"
	resp, err := httpx.Get(ctx, searchJSONURL+"?"+v.Encode(), h, 20*time.Second)
	if err != nil {
		return nil, err
	}
	var items []searchItem
	if err := json.Unmarshal(resp.Body, &items); err != nil {
		return nil, fmt.Errorf("MNG Turizm arama yaniti parse edilemedi: %w", err)
	}
	return items, nil
}

// hotelItems yalnizca "Otel" kategorisindeki sonuclari doner.
func hotelItems(items []searchItem) []searchItem {
	out := []searchItem{}
	seen := map[string]bool{}
	for _, it := range items {
		if !strings.EqualFold(strings.TrimSpace(it.Category), "otel") || it.CategoryID == "" || seen[it.CategoryID] {
			continue
		}
		seen[it.CategoryID] = true
		out = append(out, it)
	}
	return out
}

const maxSearchCandidates = 10

// SearchHotels searchjson.php autocomplete'i ile otel arar; her otel icin s=hotelsearch yonlendirmesi
// takip edilerek detay URL'i ve sayfadaki otel id'si alinir.
func (p *Provider) SearchHotels(ctx context.Context, query string) ([]provider.HotelCandidate, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("arama metni bos")
	}
	items, err := searchJSON(ctx, query)
	if err != nil {
		return nil, err
	}
	hotels := hotelItems(items)
	words := strings.Fields(query)
	for len(hotels) == 0 && len(words) > 1 {
		words = words[:len(words)-1]
		items, err = searchJSON(ctx, strings.Join(words, " "))
		if err != nil {
			return nil, err
		}
		hotels = hotelItems(items)
	}
	out := []provider.HotelCandidate{}
	for i, it := range hotels {
		if i >= maxSearchCandidates {
			break
		}
		v := url.Values{"yasehir": {it.Label}, "aramaKategoriId": {it.CategoryID}}
		resp, err := httpx.Do(ctx, http.MethodGet, hotelSearchURL+"&"+v.Encode(), httpx.Options{Headers: pageHeaders, Timeout: 25 * time.Second})
		if err != nil {
			continue
		}
		fu, perr := url.Parse(resp.FinalURL)
		if perr != nil || !reDomain.MatchString(resp.FinalURL) || strings.Contains(fu.Path, "index.php") {
			continue
		}
		fu.RawQuery, fu.Fragment = "", ""
		r := parseDetailPage(resp.Text())
		if r.ProviderHotelID == "" {
			continue
		}
		name := r.HotelName
		if name == "" {
			name = provider.DecodeEntities(it.Label)
		}
		out = append(out, provider.HotelCandidate{Name: name, URL: fu.String(), City: r.City, ProviderHotelID: r.ProviderHotelID})
	}
	return out, nil
}
