package tatilsepeti

// Statik otel detayi (provider.DetailProvider).
//
// Kaynak: otel detay HTML sayfasi (tarayici-gezinme basliklariyla GET; engelde tarayici fallback'i, bkz.
// resolveHotelFromURL). Sayfa sunucu tarafinda render edilir: JSON-LD (LodgingBusiness: giris/cikis saati,
// evcil hayvan, adres, puan), galeri (`.photo-gallery` img), oda kartlari (`.hotel-detail-cards`,
// data-roomtypeid) + oda modallari (`.room-properties`: m2, ozellikler), "detail-title" basliklari
// (Genel Bilgiler, Konum Bilgileri, Otel Ozellikleri, Ucretsiz/Ucretli Aktiviteler, Cocuklar Icin, Diger Notlar),
// misafir puanlari. Oda gorselleri ayri XHR: GET /hotel/gethotelroomimages/?hotelId&roomTypeId&ImageSizeType=3
// (oda basina bir istek; hata olursa kart gorseliyle devam edilir).
// Cikti deterministiktir: sayfa sirasi korunur, gorsel URL'lerinde sorgu parametresi (cache-buster) atilir.

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

	"pricebot/internal/browser"
	"pricebot/internal/httpx"
	"pricebot/internal/provider"
)

const roomImagesURL = "https://www.tatilsepeti.com/hotel/gethotelroomimages/"

var (
	reH1Name       = regexp.MustCompile(`(?s)<h1 class="hotel__name[^"]*"[^>]*>(.*?)</h1>`)
	reRegion       = regexp.MustCompile(`(?s)<div class="hotel__region">(.*?)<span`)
	reJSONLD       = regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)
	reGallery      = regexp.MustCompile(`(?s)<div class="photo-gallery[^"]*"[^>]*>(.*?)<div class="hotelDetailPhotoThumbs`)
	reGalleryImg   = regexp.MustCompile(`(?s)<img([^>]*)>`)
	reImgSrc       = regexp.MustCompile(`(?:data-src|src)="(https?://[^"]+)"`)
	reImgTitle     = regexp.MustCompile(`title="([^"]*)"`)
	reDetailTitle  = regexp.MustCompile(`(?s)<h[23] class="detail-title">(.*?)</h[23]>`)
	reDescCard     = regexp.MustCompile(`(?s)<div class="ts-card descriptions">(.*?)</div>\s*</div>`)
	reTableRow     = regexp.MustCompile(`(?s)<th[^>]*>(.*?)</th>\s*<td[^>]*>(.*?)</td>`)
	reSpectRow     = regexp.MustCompile(`(?s)room-spect__title">(.*?)</div>\s*<div[^>]*>(.*?)</div>`)
	reChildRow     = regexp.MustCompile(`(?s)activities-for-children__title">(.*?)</td>\s*<td>(.*?)</td>`)
	reBold         = regexp.MustCompile(`(?s)<div><b>(.*?)</b></div>`)
	reRoomCard     = regexp.MustCompile(`(?s)<div class="hotel-detail-cards[^"]*"[^>]*data-roomtypeid="(\d+)"`)
	reRoomName     = regexp.MustCompile(`(?s)<h3>(.*?)</h3>`)
	reRoomImg      = regexp.MustCompile(`(?:data-src|src)="(https?://[^"]*/TesisOda/[^"]+)"`)
	reRoomCapacity = regexp.MustCompile(`(?s)hotel-detail-cards__content-div__description[^>]*>.*?<span>(.*?)</span>`)
	reRoomFeatures = regexp.MustCompile(`(?s)hotel-detail-cards__content-div__hotel-features[^>]*>\s*<ul>(.*?)</ul>`)
	reSpan         = regexp.MustCompile(`(?s)<span>(.*?)</span>`)
	reRoomProps    = regexp.MustCompile(`(?s)<div class="room-properties">(.*?)</ul>`)
	reRoomM2       = regexp.MustCompile(`(?s)Oda metrakaresi:\s*<strong>(.*?)</strong>`)
	reSizeNum      = regexp.MustCompile(`(\d{2,3})`)
	rePension      = regexp.MustCompile(`var ttl_pension_type = "([^"]*)"`)
	reReviewScore  = regexp.MustCompile(`(?s)hotel-detail-guest-review-upper-leftside-point-second">\s*<p>(.*?)</p>\s*<span>(.*?)</span>`)
	reReviewTexts  = regexp.MustCompile(`(?s)hotel-detail-guest-review-upper-leftside-comment-texts[^>]*>(.*?)</div>`)
	reReviewSub    = regexp.MustCompile(`(?s)hotel-detail-guest-review-upper-rightside-text">(.*?)</div>.*?hotel-detail-guest-revoew-rightside-value">(.*?)</div>`)
	reReviewBlock  = regexp.MustCompile(`(?s)hotel-detail-guest-review-upper-rightside-second[^>]*>.*?rightside-value">[^<]*</div>`)
	reTimeHHMM     = regexp.MustCompile(`^(\d{1,2}:\d{2})`)
	reQueryTail    = regexp.MustCompile(`[?#].*$`)
)

// GetDetails detay sayfasini ceker, normalize eder; oda gorsellerini ayri XHR ile tamamlar.
func (p *Provider) GetDetails(ctx context.Context, m *provider.Mapping) (*provider.HotelDetails, error) {
	src := m.MetaString("sourceUrl")
	if src == "" {
		return nil, fmt.Errorf("Mapping metadata'da sourceUrl eksik (otel detay sayfasi URL'i gerekir)")
	}
	html, err := p.fetchDetailHTML(ctx, src)
	if err != nil {
		return nil, err
	}
	out, roomIDs, err := parseDetails(html, src)
	if err != nil {
		return nil, err
	}
	if m.ProviderHotelID != "" && out.ProviderHotelID != m.ProviderHotelID {
		return nil, fmt.Errorf("TatilSepeti sayfasi otel id %s verdi, mapping %s bekliyor", out.ProviderHotelID, m.ProviderHotelID)
	}
	// Oda gorselleri (oda basina bir XHR; basarisizsa kart gorseli kalir).
	for i := range out.Rooms {
		if roomIDs[i] == "" {
			continue
		}
		if imgs, err := p.fetchRoomImages(ctx, out.ProviderHotelID, roomIDs[i], src); err == nil && len(imgs) > 0 {
			out.Rooms[i].Images = imgs
		}
	}
	out.FetchedAt = time.Now()
	return out, nil
}

func (p *Provider) fetchDetailHTML(ctx context.Context, url string) (string, error) {
	resp, err := httpx.Do(ctx, http.MethodGet, url, httpx.Options{Headers: pageHeaders, Timeout: 30 * time.Second})
	if err != nil {
		if looksBlocked(resp, err) && p.sess.bm != nil {
			page, cancel, perr := p.sess.bm.NewPage(ctx, 90*time.Second)
			if perr != nil {
				return "", fmt.Errorf("tatilsepeti: sekme acilamadi: %w", perr)
			}
			defer cancel()
			if nerr := browser.Navigate(page, url, 1500*time.Millisecond); nerr != nil {
				return "", fmt.Errorf("tatilsepeti: sayfa acilamadi: %w", nerr)
			}
			var html string
			if eerr := browser.Eval(page, `document.documentElement.outerHTML`, &html); eerr != nil {
				return "", eerr
			}
			return html, nil
		}
		return "", err
	}
	if token := reForgeryTokenMatch(resp.Text()); token != "" || resp.SetCookies() != "" {
		p.sess.store(&session{Cookie: resp.SetCookies(), Token: token, At: time.Now(), Source: sourceHTTP})
	}
	return resp.Text(), nil
}

// fetchRoomImages GET /hotel/gethotelroomimages/ (parca HTML) -> TesisOda gorsel URL'leri (1050X700).
func (p *Provider) fetchRoomImages(ctx context.Context, hotelID, roomTypeID, referer string) ([]string, error) {
	headers := httpx.Merge(defaultHeaders, map[string]string{"accept": "text/html, */*; q=0.01", "referer": referer})
	delete(headers, "content-type")
	if s, err := p.sess.get(ctx, referer); err == nil && s.Cookie != "" {
		headers["cookie"] = s.Cookie
	}
	u := roomImagesURL + "?hotelId=" + hotelID + "&roomTypeId=" + roomTypeID + "&ImageSizeType=3"
	resp, err := httpx.Get(ctx, u, headers, 20*time.Second)
	if err != nil {
		return nil, err
	}
	body := resp.Text()
	// Uc, XHR basliklariyla parca HTML'i JSON string olarak sarip doner ("\r\n<div class=\"room-pictures ...").
	if strings.HasPrefix(strings.TrimSpace(body), `"`) {
		var s string
		if json.Unmarshal(resp.Body, &s) == nil {
			body = s
		}
	}
	return roomImagesFromHTML(body), nil
}

func roomImagesFromHTML(html string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, m := range reRoomImg.FindAllStringSubmatch(html, -1) {
		u := reQueryTail.ReplaceAllString(m[1], "")
		if !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}

func clean(s string) string { return provider.DecodeEntities(provider.StripTags(s)) }

// ldLodging JSON-LD LodgingBusiness alanlari (dizi/nesne toleransli).
type ldLodging struct {
	CheckIn, CheckOut, Pets, Address, Rating string
	ReviewCount                              int
}

func parseJSONLD(html string) ldLodging {
	var out ldLodging
	for _, m := range reJSONLD.FindAllStringSubmatch(html, -1) {
		var obj map[string]any
		if json.Unmarshal([]byte(m[1]), &obj) != nil || obj["@type"] != "LodgingBusiness" {
			continue
		}
		str := func(k string) string {
			if v, ok := obj[k].(string); ok {
				return strings.TrimSpace(v)
			}
			return ""
		}
		first := func(v any) map[string]any {
			switch t := v.(type) {
			case map[string]any:
				return t
			case []any:
				if len(t) > 0 {
					if mm, ok := t[0].(map[string]any); ok {
						return mm
					}
				}
			}
			return nil
		}
		out.CheckIn, out.CheckOut, out.Pets = str("checkinTime"), str("checkoutTime"), str("petsAllowed")
		if a := first(obj["address"]); a != nil {
			if s, ok := a["streetAddress"].(string); ok {
				out.Address = provider.DecodeEntities(s)
			}
		}
		if r := first(obj["aggregateRating"]); r != nil {
			if s, ok := r["ratingValue"].(string); ok {
				out.Rating = s
			}
			if s, ok := r["reviewCount"].(string); ok {
				out.ReviewCount, _ = strconv.Atoi(s)
			}
		}
		break
	}
	return out
}

// titleCaseTR "tam pansiyon plus" -> "Tam Pansiyon Plus".
func titleCaseTR(s string) string {
	words := strings.Fields(strings.TrimSpace(s))
	for i, w := range words {
		r := []rune(w)
		if r[0] == 'i' {
			r[0] = 'İ'
		} else {
			r[0] = []rune(strings.ToUpper(string(r[0])))[0]
		}
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

// section detay sayfasindaki bir "detail-title" bolumu (baslik + takip eden HTML).
type section struct{ Title, Body string }

func splitSections(html string) []section {
	ms := reDetailTitle.FindAllStringSubmatchIndex(html, -1)
	out := []section{}
	for i, m := range ms {
		end := len(html)
		if i+1 < len(ms) {
			end = ms[i+1][0]
		}
		body := html[m[1]:end]
		if j := strings.Index(body, "<footer"); j >= 0 {
			body = body[:j]
		}
		out = append(out, section{Title: clean(html[m[2]:m[3]]), Body: body})
	}
	return out
}

// parseDetails saf HTML -> HotelDetails donusumu (test edilebilir; ag erisimi yok).
// Ikinci donus: Rooms ile ayni sirada roomTypeId'ler (oda gorseli XHR'i icin).
func parseDetails(html, sourceURL string) (*provider.HotelDetails, []string, error) {
	base, err := parseDetailPage(html)
	if err != nil {
		return nil, nil, err
	}
	roomIDs := []string{}
	name := clean(provider.FirstMatch(reH1Name, html))
	if name == "" {
		name = base.HotelName
	}
	out := &provider.HotelDetails{
		Provider: "tatilsepeti", ProviderHotelID: base.ProviderHotelID, SourceURL: sourceURL, Name: name,
		Images: []provider.Image{}, Facilities: []provider.FacilityGroup{}, Rooms: []provider.RoomInfo{},
		Boards: []string{}, FAQ: []provider.QA{}, Sections: []provider.Section{},
	}
	ld := parseJSONLD(html)
	out.Address = ld.Address
	if ld.Rating != "" {
		if v, err := strconv.ParseFloat(strings.ReplaceAll(ld.Rating, ",", "."), 64); err == nil && v > 0 {
			out.Rating = &v
		}
	}
	if ld.ReviewCount > 0 {
		c := ld.ReviewCount
		out.RatingCount = &c
	}
	// "Kıbrıs - Girne-Çatalköy" -> ulke / sehir / bolge
	if reg := clean(provider.FirstMatch(reRegion, html)); reg != "" {
		parts := strings.SplitN(reg, " - ", 2)
		if len(parts) == 2 {
			out.Country = strings.TrimSpace(parts[0])
			cp := strings.SplitN(parts[1], "-", 2)
			out.City = strings.TrimSpace(cp[0])
			if len(cp) == 2 {
				out.Region = strings.TrimSpace(cp[1])
			}
		} else {
			out.City = reg
		}
	}
	if pt := provider.FirstMatch(rePension, html); strings.TrimSpace(pt) != "" {
		out.Boards = append(out.Boards, titleCaseTR(pt))
	}

	// Galeri
	seen := map[string]bool{}
	for _, im := range reGalleryImg.FindAllStringSubmatch(provider.FirstMatch(reGallery, html), -1) {
		u := provider.FirstMatch(reImgSrc, im[1])
		if u == "" || !strings.Contains(u, "/Files/Images/") {
			continue
		}
		u = reQueryTail.ReplaceAllString(u, "")
		if seen[u] {
			continue
		}
		seen[u] = true
		out.Images = append(out.Images, provider.Image{URL: u, Caption: provider.DecodeEntities(provider.FirstMatch(reImgTitle, im[1]))})
	}

	// Odalar (kart sirasi; roomtypeid benzersiz)
	cards := reRoomCard.FindAllStringSubmatchIndex(html, -1)
	seenRoom := map[string]bool{}
	for i, c := range cards {
		rid := html[c[2]:c[3]]
		if seenRoom[rid] {
			continue
		}
		seenRoom[rid] = true
		end := len(html)
		if i+1 < len(cards) {
			end = cards[i+1][0]
		}
		card := html[c[0]:end]
		if j := strings.Index(card, `id="otelUp"`); j >= 0 {
			card = card[:j]
		}
		ri := provider.RoomInfo{Name: clean(provider.FirstMatch(reRoomName, card)), Features: []string{}, Images: []string{}}
		if ri.Name == "" {
			continue
		}
		ri.Capacity = clean(provider.FirstMatch(reRoomCapacity, card))
		for _, sp := range reSpan.FindAllStringSubmatch(provider.FirstMatch(reRoomFeatures, card), -1) {
			if t := clean(sp[1]); t != "" {
				ri.Features = append(ri.Features, t)
			}
		}
		if props := provider.FirstMatch(reRoomProps, card); props != "" {
			if sz := provider.FirstMatch(reRoomM2, props); sz != "" {
				if n, err := strconv.Atoi(provider.FirstMatch(reSizeNum, sz)); err == nil {
					ri.Size = &n
				}
				ri.Description = "Oda metrekaresi: " + clean(sz)
			}
			for _, f := range htmlListItems("<ul>" + props[strings.Index(props, "<ul>")+4:] + "</ul>") {
				if f != "" && !containsStr(ri.Features, f) {
					ri.Features = append(ri.Features, f)
				}
			}
		}
		if u := provider.FirstMatch(reRoomImg, card); u != "" {
			ri.Images = append(ri.Images, reQueryTail.ReplaceAllString(u, ""))
		}
		out.Rooms = append(out.Rooms, ri)
		roomIDs = append(roomIDs, rid)
	}

	// Basliklar
	pol := &provider.Policies{Other: []string{}}
	if m := reTimeHHMM.FindStringSubmatch(ld.CheckIn); m != nil {
		pol.CheckIn = m[1]
	}
	if m := reTimeHHMM.FindStringSubmatch(ld.CheckOut); m != nil {
		pol.CheckOut = m[1]
	}
	switch strings.ToLower(ld.Pets) {
	case "no", "false":
		pol.PetPolicy = "Evcil hayvan kabul edilmemektedir."
	case "yes", "true":
		pol.PetPolicy = "Evcil hayvan kabul edilmektedir."
	}
	for _, s := range splitSections(html) {
		switch {
		case strings.HasSuffix(s.Title, "Genel Bilgiler"):
			if t := htmlText(provider.FirstMatch(reDescCard, s.Body)); t != "" {
				out.Description = t
				out.Sections = append(out.Sections, provider.Section{Title: s.Title, Kind: "text", Text: t})
			}
		case s.Title == "Konum Bilgileri":
			pairs := []provider.KV{}
			for _, r := range reTableRow.FindAllStringSubmatch(s.Body, -1) {
				if k, v := strings.TrimSuffix(clean(r[1]), ":"), clean(r[2]); k != "" && v != "" {
					pairs = append(pairs, provider.KV{Key: k, Value: v})
				}
			}
			if len(pairs) > 0 {
				out.Sections = append(out.Sections, provider.Section{Title: s.Title, Kind: "pairs", Pairs: pairs})
			}
		case s.Title == "Otel Özellikleri":
			pairs := []provider.KV{}
			for _, r := range reSpectRow.FindAllStringSubmatch(s.Body, -1) {
				if k, v := clean(r[1]), clean(r[2]); k != "" && v != "" {
					pairs = append(pairs, provider.KV{Key: k, Value: v})
				}
			}
			if len(pairs) > 0 {
				out.Sections = append(out.Sections, provider.Section{Title: s.Title, Kind: "pairs", Pairs: pairs})
			}
			if items := htmlListItems(s.Body); len(items) > 0 {
				out.Facilities = append(out.Facilities, provider.FacilityGroup{Title: s.Title, Items: items})
				out.Sections = append(out.Sections, provider.Section{Title: s.Title + " (Liste)", Kind: "list", Items: items})
			}
		case s.Title == "Ücretsiz Aktiviteler", s.Title == "Ücretli Aktiviteler":
			if items := htmlListItems(s.Body); len(items) > 0 {
				out.Facilities = append(out.Facilities, provider.FacilityGroup{Title: s.Title, Items: items})
				out.Sections = append(out.Sections, provider.Section{Title: s.Title, Kind: "list", Items: items})
			}
		case s.Title == "Çocuklar İçin":
			items, pairs := []string{}, []provider.KV{}
			for _, r := range reChildRow.FindAllStringSubmatch(s.Body, -1) {
				k := clean(r[1])
				vals := htmlListItems(r[2])
				if k == "" || len(vals) == 0 {
					continue
				}
				pairs = append(pairs, provider.KV{Key: k, Value: strings.Join(vals, ", ")})
				for _, v := range vals {
					items = append(items, v+" ("+k+")")
				}
			}
			if len(items) > 0 {
				out.Facilities = append(out.Facilities, provider.FacilityGroup{Title: s.Title, Items: items})
				out.Sections = append(out.Sections, provider.Section{Title: s.Title, Kind: "pairs", Pairs: pairs})
			}
		case s.Title == "Diğer Notlar":
			body := provider.FirstMatch(reDescCard, s.Body)
			if body == "" {
				body = s.Body
			}
			heads := reBold.FindAllStringSubmatchIndex(body, -1)
			if len(heads) == 0 {
				if t := htmlText(body); t != "" {
					out.Sections = append(out.Sections, provider.Section{Title: s.Title, Kind: "text", Text: t})
				}
				break
			}
			for i, h := range heads {
				end := len(body)
				if i+1 < len(heads) {
					end = heads[i+1][0]
				}
				title := clean(body[h[2]:h[3]])
				lines := htmlLines(body[h[1]:end])
				if title == "" || len(lines) == 0 {
					continue
				}
				out.Sections = append(out.Sections, provider.Section{Title: s.Title + ": " + title, Kind: "list", Items: lines})
				low := strings.ToLower(title)
				switch {
				case strings.Contains(low, "çocuk"):
					pol.ChildPolicy = strings.Join(lines, "\n")
				case strings.Contains(low, "giriş") || strings.Contains(low, "check"):
					pol.Other = append(pol.Other, lines...)
				}
				for _, ln := range lines {
					l := strings.ToLower(ln)
					if strings.Contains(l, "evcil hayvan") && pol.PetPolicy == "" {
						pol.PetPolicy = ln
					}
					if (strings.Contains(l, "giriş saati") || strings.Contains(l, "iptal") || strings.Contains(l, "provizyon")) && !containsStr(pol.Other, ln) {
						pol.Other = append(pol.Other, ln)
					}
				}
			}
		case strings.HasSuffix(s.Title, "Yorumları"):
			pairs := []provider.KV{}
			if m := reReviewScore.FindStringSubmatch(s.Body); m != nil {
				pairs = append(pairs, provider.KV{Key: clean(m[1]) + " Puanı", Value: clean(m[2])})
			}
			for _, sp := range reSpan.FindAllStringSubmatch(provider.FirstMatch(reReviewTexts, s.Body), -1) {
				if t := clean(sp[1]); t != "" {
					kv := strings.SplitN(t, " ", 2)
					if len(kv) == 2 {
						pairs = append(pairs, provider.KV{Key: kv[1], Value: kv[0]})
					}
				}
			}
			for _, blk := range reReviewBlock.FindAllString(s.Body, -1) {
				if m := reReviewSub.FindStringSubmatch(blk); m != nil {
					pairs = append(pairs, provider.KV{Key: clean(m[1]), Value: clean(m[2])})
				}
			}
			if len(pairs) > 0 {
				out.Sections = append(out.Sections, provider.Section{Title: s.Title, Kind: "pairs", Pairs: pairs})
			}
		}
	}
	if pol.CheckIn != "" || pol.CheckOut != "" || pol.PetPolicy != "" || pol.ChildPolicy != "" || len(pol.Other) > 0 {
		sort.Strings(pol.Other)
		out.Policies = pol
	}
	return out, roomIDs, nil
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
