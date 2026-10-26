package tatilsitesi

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"

	"pricebot/internal/provider"
)

// Statik detay: detay sayfasinin JS'i (HotelDetail.js) ile ayni WebMethod'lar cagrilir, hepsi
// cerezsiz JSON:
//   - GetHotelInfo {HotelCode}          -> Info (ad, sehir, yildiz, tum metin alanlari HTML, ImageList)
//   - getHDDetailContent {hotelCode}    -> one cikan ozellik ikonlari
//   - getHDChildAges {hotelCode}        -> cocuk yas politikasi
//   - getHDAccomodation {hotelCode}     -> pansiyon tipleri
//   - Get_HotelCoordinat {hotelCode}    -> koordinat (inGoogleX = enlem, inGoogleY = boylam)
// Odalar yalnizca tarihli hotelDetailSearch ile geliyor (musaitlige bagli) -> statik detayda yok.

const detailTimeout = 30 * time.Second

// HotelInfo GetHotelInfo.d.Info (metin alanlari HTML).
type HotelInfo struct {
	HotelName       string    `json:"HotelName"`
	Star            int       `json:"Star"`
	ImageList       []tsImage `json:"ImageList"`
	CityName        string    `json:"CityName"`
	CountryName     string    `json:"CountryName"`
	TownName        string    `json:"TownName"`
	Url             string    `json:"Url"`
	MainImage       string    `json:"MainImage"`
	PageDescription string    `json:"PageDescription"`

	GeneralInfo         string `json:"GeneralInfo"`
	BriefInfo           string `json:"BriefInfo"`
	HotelFeatures       string `json:"HotelFeatures"`
	HotelLocation       string `json:"HotelLocation"`
	HotelTransportation string `json:"HotelTransportation"`
	HotelFoodConcept    string `json:"HotelFoodConcept"`
	ChildConcept        string `json:"ChildConcept"`
	BeachInfo           string `json:"BeachInfo"`
	RoomInfo            string `json:"RoomInfo"`
	HoneymoonInfo       string `json:"HoneymoonInfo"`
	Facilities          string `json:"Facilities"`
	FreeServices        string `json:"FreeServices"`
	PaidServices        string `json:"PaidServices"`
	ConceptDates        string `json:"ConceptDates"`
	WinterConcept       string `json:"WinterConcept"`
	WinterFreeServices  string `json:"WinterFreeServices"`
	WinterPaidServices  string `json:"WinterPaidServices"`
	WinterChildConcept  string `json:"WinterChildConcept"`
	GeneralNotice       string `json:"GeneralNotice"`
	Notes               string `json:"Notes"`
	AdditionalInfo      string `json:"AdditionalInfo"`
}

type tsImage struct {
	InID        int     `json:"inID"`
	StFilePath  string  `json:"stFilePath"`
	InActive    bool    `json:"inActive"`
	InOrder     int     `json:"inOrder"`
	StImageType *string `json:"stImageType"`
	StTitle     *string `json:"stTitle"`
}

type hotelInfoResponse struct {
	D struct {
		Info *HotelInfo `json:"Info"`
	} `json:"d"`
}

// Widget getHDDetailContent / getHDChildAges / getHDAccomodation ortak satiri.
type Widget struct {
	StHeader  string `json:"stHeader"`
	StContent string `json:"stContent"`
	StCode    string `json:"stCode"`
}

type widgetResponse struct {
	D []Widget `json:"d"`
}

type coordResponse struct {
	D struct {
		InGoogleX *float64 `json:"inGoogleX"`
		InGoogleY *float64 `json:"inGoogleY"`
	} `json:"d"`
}

// detailData statik detay icin toplanan ham veri (testlerde fixture'dan doldurulur).
type detailData struct {
	Info      *HotelInfo
	Content   []Widget
	ChildAges []Widget
	Boards    []Widget
	Lat, Lng  float64
}

func fetchDetailData(ctx context.Context, hotelCode, referer string) (*detailData, error) {
	var info hotelInfoResponse
	if err := postMethod(ctx, "GetHotelInfo", map[string]any{"HotelCode": hotelCode}, referer, detailTimeout, &info); err != nil {
		return nil, err
	}
	if info.D.Info == nil || strings.TrimSpace(info.D.Info.HotelName) == "" {
		return nil, fmt.Errorf("Tatilsitesi GetHotelInfo otel bilgisi dondurmedi (kod %s)", hotelCode)
	}
	d := &detailData{Info: info.D.Info}
	// Yardimci uclar: biri hata verirse detay yine uretilir (alan bos kalir).
	widgets := func(method string) []Widget {
		var out widgetResponse
		if err := postMethod(ctx, method, map[string]any{"hotelCode": hotelCode}, referer, detailTimeout, &out); err != nil {
			return nil
		}
		return out.D
	}
	d.Content = widgets("getHDDetailContent")
	d.ChildAges = widgets("getHDChildAges")
	d.Boards = widgets("getHDAccomodation")
	var c coordResponse
	if err := postMethod(ctx, "Get_HotelCoordinat", map[string]any{"hotelCode": hotelCode}, referer, detailTimeout, &c); err == nil && c.D.InGoogleX != nil && c.D.InGoogleY != nil {
		d.Lat, d.Lng = *c.D.InGoogleX, *c.D.InGoogleY
	}
	return d, nil
}

// GetDetails otel statik detayini ortak semaya cevirir.
func (p *Provider) GetDetails(ctx context.Context, m *provider.Mapping) (*provider.HotelDetails, error) {
	if m.ProviderHotelID == "" {
		return nil, fmt.Errorf("providerHotelId (hotelCODE) eksik")
	}
	d, err := fetchDetailData(ctx, m.ProviderHotelID, refererOf(m))
	if err != nil {
		return nil, err
	}
	out := normalizeDetails(d)
	out.ProviderHotelID = m.ProviderHotelID
	out.SourceURL = m.MetaString("sourceUrl")
	if out.SourceURL == "" && d.Info.Url != "" {
		out.SourceURL = base + "/" + strings.TrimPrefix(d.Info.Url, "/")
	}
	return out, nil
}

// infoSection Info alani -> sitedeki sekme basligi.
type infoSection struct {
	title string
	get   func(*HotelInfo) string
}

var infoSections = []infoSection{
	{"Öne Çıkanlar", func(i *HotelInfo) string { return i.BriefInfo }},
	{"Tesis Özellikleri", func(i *HotelInfo) string { return i.HotelFeatures }},
	{"Genel Özellikler", func(i *HotelInfo) string { return i.Facilities }},
	{"Konum", func(i *HotelInfo) string { return i.HotelLocation }},
	{"Ulaşım", func(i *HotelInfo) string { return i.HotelTransportation }},
	{"Yeme & İçme", func(i *HotelInfo) string { return i.HotelFoodConcept }},
	{"Çocuk", func(i *HotelInfo) string { return i.ChildConcept }},
	{"Plaj & Havuz", func(i *HotelInfo) string { return i.BeachInfo }},
	{"Odalar", func(i *HotelInfo) string { return i.RoomInfo }},
	{"Balayı", func(i *HotelInfo) string { return i.HoneymoonInfo }},
	{"Ücretsiz Hizmetler", func(i *HotelInfo) string { return i.FreeServices }},
	{"Ücretli Hizmetler", func(i *HotelInfo) string { return i.PaidServices }},
	{"Konsept Tarihleri", func(i *HotelInfo) string { return i.ConceptDates }},
	{"Kış Konsepti", func(i *HotelInfo) string { return i.WinterConcept }},
	{"Kış Ücretsiz Hizmetler", func(i *HotelInfo) string { return i.WinterFreeServices }},
	{"Kış Ücretli Hizmetler", func(i *HotelInfo) string { return i.WinterPaidServices }},
	{"Kış Çocuk", func(i *HotelInfo) string { return i.WinterChildConcept }},
	{"Önemli Bilgiler", func(i *HotelInfo) string { return i.GeneralNotice }},
	{"Notlar", func(i *HotelInfo) string { return i.Notes }},
	{"Ek Bilgiler", func(i *HotelInfo) string { return i.AdditionalInfo }},
}

func normalizeDetails(d *detailData) *provider.HotelDetails {
	info := d.Info
	out := &provider.HotelDetails{
		Provider:   "tatilsitesi",
		Name:       provider.DecodeEntities(strings.TrimSpace(info.HotelName)),
		City:       strings.TrimSpace(info.TownName),
		Region:     strings.TrimSpace(info.CityName),
		Country:    strings.TrimSpace(info.CountryName),
		Images:     []provider.Image{},
		Facilities: []provider.FacilityGroup{},
		Rooms:      []provider.RoomInfo{},
		FAQ:        []provider.QA{},
		Sections:   []provider.Section{},
	}
	if info.Star > 0 {
		s := info.Star
		out.Stars = &s
	}
	if d.Lat != 0 && d.Lng != 0 {
		out.Location = &provider.GeoPoint{Lat: d.Lat, Lng: d.Lng}
	}

	// Gorseller: aktif olanlar, (inOrder, inID) sirali; kategori adi aciklama olur.
	imgs := make([]tsImage, 0, len(info.ImageList))
	for _, im := range info.ImageList {
		if im.InActive && strings.TrimSpace(im.StFilePath) != "" {
			imgs = append(imgs, im)
		}
	}
	sort.SliceStable(imgs, func(i, j int) bool {
		if imgs[i].InOrder != imgs[j].InOrder {
			return imgs[i].InOrder < imgs[j].InOrder
		}
		return imgs[i].InID < imgs[j].InID
	})
	seen := map[string]bool{}
	for _, im := range imgs {
		u := cleanImageURL(im.StFilePath)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		img := provider.Image{URL: u}
		if im.StImageType != nil {
			img.Caption = strings.TrimSpace(*im.StImageType)
		}
		out.Images = append(out.Images, img)
	}
	if len(out.Images) == 0 && strings.TrimSpace(info.MainImage) != "" {
		out.Images = append(out.Images, provider.Image{URL: cleanImageURL(info.MainImage)})
	}

	// Hakkinda (GeneralInfo): tanitim metni + SSS (<details>).
	about, faq := splitGeneralInfo(info.GeneralInfo)
	out.FAQ = faq
	if about != "" {
		out.Sections = append(out.Sections, provider.Section{Title: "Hakkında", Kind: "text", Text: about})
		out.Description = firstParagraph(about)
	}
	if out.Description == "" {
		out.Description = htmlToText(info.PageDescription)
	}

	// One cikan ozellikler (ikon listesi).
	items := []string{}
	for _, w := range d.Content {
		if t := htmlToText(w.StContent); t != "" {
			items = append(items, t)
		}
	}
	if len(items) > 0 {
		out.Facilities = append(out.Facilities, provider.FacilityGroup{Title: "Öne Çıkanlar", Items: items})
	}

	// Metin alanlari -> bolumler; liste iceren alanlar ozellik grubu olarak da eklenir.
	for _, s := range infoSections {
		raw := s.get(info)
		txt := htmlToText(raw)
		if txt == "" {
			continue
		}
		if li := listItems(raw); len(li) > 0 && (s.title == "Ücretsiz Hizmetler" || s.title == "Ücretli Hizmetler" || s.title == "Genel Özellikler") {
			out.Facilities = append(out.Facilities, provider.FacilityGroup{Title: s.title, Items: li})
		}
		out.Sections = append(out.Sections, provider.Section{Title: s.title, Kind: "text", Text: txt})
	}

	// Pansiyon tipleri
	boards := []string{}
	for _, w := range d.Boards {
		if t := htmlToText(w.StHeader); t != "" {
			boards = append(boards, t)
		}
	}
	if len(boards) > 0 {
		out.Boards = boards
		out.Sections = append(out.Sections, provider.Section{Title: "Pansiyon Tipleri", Kind: "list", Items: boards})
	}

	// Politikalar
	pol := &provider.Policies{}
	if len(d.ChildAges) > 0 {
		pairs := []provider.KV{}
		parts := []string{}
		for _, w := range d.ChildAges {
			h, c := htmlToText(w.StHeader), htmlToText(w.StContent)
			if h == "" && c == "" {
				continue
			}
			pairs = append(pairs, provider.KV{Key: h, Value: c})
			parts = append(parts, strings.TrimSpace(h+": "+c))
		}
		if len(pairs) > 0 {
			pol.ChildPolicy = strings.Join(parts, "; ")
			out.Sections = append(out.Sections, provider.Section{Title: "Çocuk Yaş Politikası", Kind: "pairs", Pairs: pairs})
		}
	}
	allText := strings.Join([]string{htmlToText(info.Notes), htmlToText(info.GeneralNotice), htmlToText(info.HotelFeatures), about}, "\n")
	if s := sentenceWith(allText, "evcil"); s != "" {
		pol.PetPolicy = s
	}
	if m := reCheckIn.FindStringSubmatch(allText); m != nil {
		pol.CheckIn = m[1]
	}
	if m := reCheckOut.FindStringSubmatch(allText); m != nil {
		pol.CheckOut = m[1]
	}
	if pol.CheckIn != "" || pol.CheckOut != "" || pol.ChildPolicy != "" || pol.PetPolicy != "" || pol.Cancellation != "" {
		out.Policies = pol
	}
	return out
}

var (
	reCheckIn  = regexp.MustCompile(`(?i)giri[sş] saati[^0-9]{0,40}?(\d{1,2}[:.]\d{2})`)
	reCheckOut = regexp.MustCompile(`(?i)[cç][iı]k[iı][sş] saati[^0-9]{0,40}?(\d{1,2}[:.]\d{2})`)
)

// sentenceWith metindeki, verilen kelimeyi iceren ilk cumleyi doner.
func sentenceWith(text, word string) string {
	for _, line := range strings.Split(text, "\n") {
		for _, s := range strings.SplitAfter(line, ".") {
			if strings.Contains(strings.ToLower(s), word) {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func firstParagraph(text string) string {
	// Ilk gercek paragraf: yeterince uzun ve cumle ile biten satir (etiket/baslik satirlari atlanir).
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimSpace(l)
		if len([]rune(l)) > 80 && strings.ContainsAny(l[len(l)-1:], ".!?") {
			return l
		}
	}
	return ""
}

// splitGeneralInfo GeneralInfo HTML'ini tanitim metni (SSS harici) ve SSS listesine ayirir.
func splitGeneralInfo(h string) (string, []provider.QA) {
	faq := []provider.QA{}
	h = strings.TrimSpace(strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(h))
	if h == "" {
		return "", faq
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(h))
	if err != nil {
		return htmlToText(h), faq
	}
	doc.Find("details").Each(func(_ int, s *goquery.Selection) {
		q := htmlToText(innerHTML(s.Find("summary").First()))
		s.Find("summary").Remove()
		a := htmlToText(innerHTML(s))
		if q != "" && a != "" {
			faq = append(faq, provider.QA{Question: q, Answer: a})
		}
	})
	doc.Find("section.ts-faq, details, script[type='application/ld+json']").Remove()
	body := doc.Find("body")
	about := htmlToText(innerHTML(body))
	return about, faq
}

func innerHTML(s *goquery.Selection) string {
	h, _ := s.Html()
	return h
}

// listItems HTML icindeki <li> metinlerini doner.
func listItems(h string) []string {
	if !strings.Contains(h, "<li") {
		return nil
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(h))
	if err != nil {
		return nil
	}
	out := []string{}
	doc.Find("li").Each(func(_ int, s *goquery.Selection) {
		if t := htmlToText(innerHTML(s)); t != "" {
			out = append(out, t)
		}
	})
	return out
}

func cleanImageURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "/") {
		raw = "https://cdn.tatilsitesi.com" + raw
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

var (
	reSpaces     = regexp.MustCompile(`[ \t\x{00a0}]+`)
	reBlankLines = regexp.MustCompile(`\n\s*\n+`)
)

// htmlToText HTML parcasini duz metne cevirir (script/style/yorum atilir, blok etiketleri satir sonu).
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
	doc.Find("script,style,noscript,iframe,img").Remove()
	doc.Find("br").Each(func(_ int, s *goquery.Selection) { s.ReplaceWithHtml("\n") })
	doc.Find("p,div,li,tr,h1,h2,h3,h4,h5,h6,ul,ol,table,section,article,details,summary").Each(func(_ int, s *goquery.Selection) {
		s.PrependHtml("\n")
		s.AppendHtml("\n")
	})
	lines := strings.Split(doc.Text(), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(reSpaces.ReplaceAllString(l, " "))
	}
	txt := reBlankLines.ReplaceAllString(strings.Join(lines, "\n"), "\n")
	return strings.TrimSpace(txt)
}

// ---- Isimle arama ----

// GeneralHotel GetGeneralList.d.Hotels elemani.
type GeneralHotel struct {
	Text   string `json:"text"`
	GotoID string `json:"gotoid"` // detay sayfasi slug'i
	Code   string `json:"code"`   // hotelCODE
}

type generalListResponse struct {
	D struct {
		Hotels []GeneralHotel `json:"Hotels"`
	} `json:"d"`
}

// SearchHotels GetGeneralList otel listesini (tum satilan oteller) isme gore filtreler.
func (p *Provider) SearchHotels(ctx context.Context, query string) ([]provider.HotelCandidate, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("arama metni bos")
	}
	var out generalListResponse
	if err := postMethod(ctx, "GetGeneralList", map[string]any{}, "", detailTimeout, &out); err != nil {
		return nil, err
	}
	return matchHotels(out.D.Hotels, query), nil
}

// matchHotels sorgu kelimelerinin tumu otel adindaki bir kelimenin on eki olan otelleri doner;
// sonuc yoksa sorgu sondan kisaltilarak tekrar denenir.
func matchHotels(hotels []GeneralHotel, query string) []provider.HotelCandidate {
	words := foldWords(query)
	for len(words) > 0 {
		out := []provider.HotelCandidate{}
		for _, h := range hotels {
			if strings.TrimSpace(h.GotoID) == "" || !wordsMatch(foldWords(h.Text), words) {
				continue
			}
			out = append(out, provider.HotelCandidate{
				Name: provider.DecodeEntities(strings.TrimSpace(h.Text)), URL: base + "/" + strings.TrimPrefix(strings.TrimSpace(h.GotoID), "/"),
				ProviderHotelID: strings.TrimSpace(h.Code),
			})
		}
		if len(out) > 0 {
			return out
		}
		words = words[:len(words)-1]
	}
	return []provider.HotelCandidate{}
}

func wordsMatch(hotelWords, queryWords []string) bool {
	for _, q := range queryWords {
		ok := false
		for _, w := range hotelWords {
			if strings.HasPrefix(w, q) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

var trFold = strings.NewReplacer("ı", "i", "İ", "i", "ş", "s", "Ş", "s", "ğ", "g", "Ğ", "g", "ü", "u", "Ü", "u", "ö", "o", "Ö", "o", "ç", "c", "Ç", "c", "â", "a", "î", "i", "û", "u")

// foldWords Turkce karakterleri sadelestirip kucuk harfli kelime listesi doner.
func foldWords(s string) []string {
	s = strings.ToLower(trFold.Replace(s))
	return strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}
