package tatilcikus

// Statik otel detayi (provider.DetailProvider).
//
// Kaynak: Hotels/GetByUrlWithDetail (otel metinleri, gorseller, tag id'leri, odalar, pansiyonlar) +
// Tags/ ve TagGroups/ (tag adlari ve grup basliklari; 1 saat cache) + Cities/Districts/Countries (ad cozumu).
// Cikti deterministiktir: tum listeler sabit anahtarlara (grup id, kayit id) gore siralanir; tarih/sayac yok.

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"pricebot/internal/httpx"
	"pricebot/internal/provider"
)

const lookupTTL = time.Hour

// imageGroupNames detay sayfasindaki galeri sekmeleri (bundle: "1"==groupId -> Genel ...).
var imageGroupNames = map[int]string{1: "Genel", 2: "Odalar", 3: "Yeme İçme", 4: "Çocuk", 5: "Aktivite"}

type Tag struct {
	ID         int    `json:"id"`
	TagGroupID int    `json:"tagGroupId"`
	Name       string `json:"name"`
	IsShown    bool   `json:"isShown"`
	Status     bool   `json:"status"`
}

type TagGroup struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	IsShown bool   `json:"isShown"`
}

// lookups detay normalizasyonu icin ad sozlukleri.
type lookups struct {
	Tags      map[int]Tag
	Groups    []TagGroup // id'ye gore sirali
	Cities    map[int]string
	Districts map[int]string
	Countries map[int]string
}

type cachedJSON struct {
	body []byte
	exp  time.Time
}

var (
	lookupMu    sync.Mutex
	lookupCache = map[string]cachedJSON{}
)

// getListCached GET <apiBase><path> govdesini TTL boyunca cache'ler (Tags/, TagGroups/, Districts/ ...).
func getListCached(ctx context.Context, path string, ttl time.Duration) ([]byte, error) {
	lookupMu.Lock()
	if c, ok := lookupCache[path]; ok && time.Now().Before(c.exp) {
		lookupMu.Unlock()
		return c.body, nil
	}
	lookupMu.Unlock()
	resp, err := httpx.Get(ctx, apiBase+path, apiHeaders, 20*time.Second)
	if err != nil {
		return nil, err
	}
	lookupMu.Lock()
	lookupCache[path] = cachedJSON{body: resp.Body, exp: time.Now().Add(ttl)}
	lookupMu.Unlock()
	return resp.Body, nil
}

func parseNameMap(body []byte) map[int]string {
	var env struct {
		Data []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	out := map[int]string{}
	if json.Unmarshal(body, &env) == nil {
		for _, c := range env.Data {
			out[c.ID] = c.Name
		}
	}
	return out
}

func parseTags(body []byte) (map[int]Tag, error) {
	var env struct {
		Data []Tag `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("Tatilcikus Tags yaniti parse edilemedi: %w", err)
	}
	out := make(map[int]Tag, len(env.Data))
	for _, t := range env.Data {
		out[t.ID] = t
	}
	return out, nil
}

func parseTagGroups(body []byte) ([]TagGroup, error) {
	var env struct {
		Data []TagGroup `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("Tatilcikus TagGroups yaniti parse edilemedi: %w", err)
	}
	sort.Slice(env.Data, func(i, j int) bool { return env.Data[i].ID < env.Data[j].ID })
	return env.Data, nil
}

func fetchLookups(ctx context.Context) (*lookups, error) {
	tb, err := getListCached(ctx, "Tags/", lookupTTL)
	if err != nil {
		return nil, err
	}
	tags, err := parseTags(tb)
	if err != nil {
		return nil, err
	}
	gb, err := getListCached(ctx, "TagGroups/", lookupTTL)
	if err != nil {
		return nil, err
	}
	groups, err := parseTagGroups(gb)
	if err != nil {
		return nil, err
	}
	l := &lookups{Tags: tags, Groups: groups, Cities: map[int]string{}, Districts: map[int]string{}, Countries: map[int]string{}}
	// Ad sozlukleri yardimci; alinamazsa bos kalir (hata degil).
	for path, dst := range map[string]*map[int]string{"Cities/": &l.Cities, "Districts/": &l.Districts, "Countries/": &l.Countries} {
		if b, err := getListCached(ctx, path, lookupTTL); err == nil {
			*dst = parseNameMap(b)
		}
	}
	return l, nil
}

// GetDetails otelin statik detayini ortak semaya normalize eder.
func (p *Provider) GetDetails(ctx context.Context, m *provider.Mapping) (*provider.HotelDetails, error) {
	slug, err := slugOf(m)
	if err != nil {
		return nil, err
	}
	d, err := fetchHotelDetail(ctx, slug)
	if err != nil {
		return nil, err
	}
	l, err := fetchLookups(ctx)
	if err != nil {
		return nil, err
	}
	src := m.MetaString("sourceUrl")
	if src == "" {
		src = siteURL + "/" + d.Hotel.URL
	}
	out := buildDetails(d, l, src)
	out.FetchedAt = time.Now()
	return out, nil
}

var (
	reMapsLatLng = regexp.MustCompile(`@(-?\d+(?:\.\d+)?),(-?\d+(?:\.\d+)?)`)
	reRoomSize   = regexp.MustCompile(`(\d{2,3})(?:\s*-\s*(\d{2,3}))?\s*m(?:²|2|\^2)`)
	reCapacity   = regexp.MustCompile(`(?i)(?:maksimum|en fazla|max\.?)\s+([^.]*?)\s+konaklaya`)
)

func imageURL(folder, name string) string {
	return siteURL + "/images/hotels/" + folder + "/gallery/optimized/" + name + ".jpg"
}

// tagLabel tag adi; ucretli ise "(Ücretli)" eki.
func tagLabel(t Tag, paid bool) string {
	n := provider.DecodeEntities(t.Name)
	if paid {
		n += " (Ücretli)"
	}
	return n
}

// buildDetails saf donusum (test edilebilir): ham detay + sozlukler -> HotelDetails.
func buildDetails(d *HotelDetail, l *lookups, sourceURL string) *provider.HotelDetails {
	h := &d.Hotel
	name := provider.DecodeEntities(h.Name)
	out := &provider.HotelDetails{
		Provider:        "tatilcikus",
		ProviderHotelID: strconv.Itoa(h.ID),
		SourceURL:       sourceURL,
		Name:            name,
		Description:     htmlText(h.GeneralText),
		City:            l.Cities[h.CityID],
		Region:          l.Districts[h.DistrictID],
		Country:         l.Countries[h.CountryID],
		Images:          []provider.Image{},
		Facilities:      []provider.FacilityGroup{},
		Rooms:           []provider.RoomInfo{},
		Boards:          []string{},
		FAQ:             []provider.QA{},
		Sections:        []provider.Section{},
	}
	if h.Score > 0 {
		s := h.Score
		out.Rating = &s
	}
	if m := reMapsLatLng.FindStringSubmatch(h.GoogleMapsURL); m != nil {
		lat, e1 := strconv.ParseFloat(m[1], 64)
		lng, e2 := strconv.ParseFloat(m[2], 64)
		if e1 == nil && e2 == nil {
			out.Location = &provider.GeoPoint{Lat: lat, Lng: lng}
		}
	}

	// Gorseller: gosterimdekiler, galeri grubu + id sirasiyla.
	imgs := make([]HotelImage, 0, len(d.HotelImages))
	for _, im := range d.HotelImages {
		if im.IsShown && im.URL != "" {
			imgs = append(imgs, im)
		}
	}
	sort.Slice(imgs, func(i, j int) bool {
		if imgs[i].GroupID != imgs[j].GroupID {
			return imgs[i].GroupID < imgs[j].GroupID
		}
		return imgs[i].ID < imgs[j].ID
	})
	roomImages := map[int][]string{}
	for _, im := range imgs {
		u := imageURL(h.FolderName, im.URL)
		cap := imageGroupNames[im.GroupID]
		if im.Description != nil && strings.TrimSpace(*im.Description) != "" {
			cap = strings.TrimSpace(provider.DecodeEntities(*im.Description))
		}
		out.Images = append(out.Images, provider.Image{URL: u, Caption: cap})
		if im.RoomID > 0 {
			roomImages[im.RoomID] = append(roomImages[im.RoomID], u)
		}
	}

	// Ozellik gruplari: TagGroups sirasiyla; grupta gosterimdeki tag'ler tag id sirasiyla.
	tagsByGroup := map[int][]HotelTag{}
	for _, ht := range d.HotelTags {
		t, ok := l.Tags[ht.TagID]
		if !ok || !t.IsShown || !t.Status {
			continue
		}
		gid := ht.TagGroupID
		if gid == 0 {
			gid = t.TagGroupID
		}
		tagsByGroup[gid] = append(tagsByGroup[gid], ht)
	}
	groupName := map[int]string{}
	for _, g := range l.Groups {
		groupName[g.ID] = provider.DecodeEntities(g.Name)
		hts := tagsByGroup[g.ID]
		if len(hts) == 0 {
			continue
		}
		sort.Slice(hts, func(i, j int) bool { return hts[i].TagID < hts[j].TagID })
		items := make([]string, 0, len(hts))
		for _, ht := range hts {
			items = append(items, tagLabel(l.Tags[ht.TagID], ht.IsPaid))
		}
		out.Facilities = append(out.Facilities, provider.FacilityGroup{Title: groupName[g.ID], Items: items})
	}

	// Odalar (gosterimdekiler, API sirasi = site sirasi; id ile sabitlenir).
	rooms := make([]Item, 0, len(d.Rooms))
	for _, r := range d.Rooms {
		if r.IsShown {
			rooms = append(rooms, r)
		}
	}
	sort.SliceStable(rooms, func(i, j int) bool { return rooms[i].ID < rooms[j].ID })
	roomTags := map[int][]RoomTag{}
	for _, rt := range d.RoomTags {
		if t, ok := l.Tags[rt.TagID]; ok && t.IsShown && t.Status {
			roomTags[rt.RoomID] = append(roomTags[rt.RoomID], rt)
		}
	}
	for _, r := range rooms {
		desc := htmlText(r.Description)
		ri := provider.RoomInfo{Name: provider.DecodeEntities(r.Name), Description: desc, Features: []string{}, Images: []string{}}
		if m := reRoomSize.FindStringSubmatch(desc); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil {
				ri.Size = &n
			}
		}
		if m := reCapacity.FindStringSubmatch(desc); m != nil {
			ri.Capacity = strings.TrimSpace(m[1])
		}
		rts := roomTags[r.ID]
		sort.Slice(rts, func(i, j int) bool { return rts[i].TagID < rts[j].TagID })
		for _, rt := range rts {
			ri.Features = append(ri.Features, tagLabel(l.Tags[rt.TagID], rt.IsPaid))
		}
		if imgs := roomImages[r.ID]; len(imgs) > 0 {
			ri.Images = imgs
		}
		out.Rooms = append(out.Rooms, ri)
	}

	// Pansiyonlar.
	concepts := make([]Item, 0, len(d.Concepts))
	for _, c := range d.Concepts {
		if c.IsShown {
			concepts = append(concepts, c)
		}
	}
	sort.SliceStable(concepts, func(i, j int) bool { return concepts[i].ID < concepts[j].ID })
	for _, c := range concepts {
		out.Boards = append(out.Boards, provider.DecodeEntities(c.Name))
	}

	// Politikalar.
	pol := &provider.Policies{Other: []string{}}
	if t := htmlText(h.ChildText); t != "" {
		pol.ChildPolicy = t
	}
	if h.FirstChildAge > 0 {
		pol.Other = append(pol.Other, fmt.Sprintf("Tedarikci cocuk yas siniri: %d (ustu yetiskin sayilir)", h.FirstChildAge))
	}
	if h.CancelOption {
		pol.Other = append(pol.Other, "İptal seçeneği satın alınabilir")
	}
	if h.MoneyOption {
		pol.Other = append(pol.Other, "Para iade seçeneği satın alınabilir")
	}
	if pol.ChildPolicy != "" || len(pol.Other) > 0 {
		out.Policies = pol
	}

	// Bolumler (sitenin kendi basliklari, sayfa sirasina yakin).
	addSection := func(title, raw string) {
		if s, ok := sectionFromHTML(title, raw); ok {
			out.Sections = append(out.Sections, s)
		}
	}
	if len(d.Highlights) > 0 {
		items := make([]string, 0, len(d.Highlights))
		for _, hl := range d.Highlights {
			if n := provider.DecodeEntities(hl.Name); n != "" {
				items = append(items, n)
			}
		}
		if len(items) > 0 {
			out.Sections = append(out.Sections, provider.Section{Title: "Öne Çıkanlar", Kind: "list", Items: items})
		}
	}
	addSection(name+" Hakkında", h.GeneralText)
	texts := append([]TagGroupText(nil), h.TagGroupHotelTexts...)
	sort.SliceStable(texts, func(i, j int) bool { return texts[i].TagGroupID < texts[j].TagGroupID })
	for _, t := range texts {
		title := groupName[t.TagGroupID]
		if title == "" {
			title = "Grup " + strconv.Itoa(t.TagGroupID)
		}
		addSection(title, t.Text)
	}
	addSection("Çocuk Bilgileri", h.ChildText) // "Çocuk" tag grubu metniyle karismasin
	addSection("Balayı Tesisi", h.MarriedText)
	addSection("Çağrı Merkezi Bilgilendirmesi", h.CallCenterText)
	for _, c := range concepts {
		addSection("Pansiyon: "+provider.DecodeEntities(c.Name), c.Description)
	}
	if h.ShowTripadvisor && h.TripadvisorRating > 0 {
		pairs := []provider.KV{
			{Key: "Puan", Value: strconv.FormatFloat(h.TripadvisorRating, 'f', -1, 64)},
			{Key: "Değerlendirme Sayısı", Value: strconv.Itoa(h.TripadvisorRatingCount)},
		}
		if h.TripadvisorName != "" {
			pairs = append(pairs, provider.KV{Key: "Tripadvisor Adı", Value: h.TripadvisorName})
		}
		subs := append([]SubRating(nil), d.SubRatings...)
		sort.SliceStable(subs, func(i, j int) bool { return subs[i].Name < subs[j].Name })
		for _, s := range subs {
			pairs = append(pairs, provider.KV{Key: s.Name, Value: strconv.FormatFloat(s.Value, 'f', -1, 64)})
		}
		out.Sections = append(out.Sections, provider.Section{Title: "Tripadvisor", Kind: "pairs", Pairs: pairs})
	}
	if len(d.HotelVideos) > 0 {
		items := []string{}
		for _, v := range d.HotelVideos {
			if v.YoutubeVideoID != "" {
				items = append(items, "https://www.youtube.com/watch?v="+v.YoutubeVideoID)
			}
		}
		sort.Strings(items)
		if len(items) > 0 {
			out.Sections = append(out.Sections, provider.Section{Title: "Video", Kind: "list", Items: items})
		}
	}
	return out
}
