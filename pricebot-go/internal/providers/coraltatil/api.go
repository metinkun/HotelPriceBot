// Package coraltatil Coral Tatil (coraltatil.com) saglayicisi.
//
// Site Imperva/Incapsula bot korumasi arkasindadir: duz HTTP ile hicbir sayfa acilmaz (JS challenge,
// TLS parmak izi). Bu yuzden tum istekler headless Chromium icinde, sitenin kendi origin'inden
// `fetch` ile yapilir (tek kalici sekme, ana sayfa acik tutulur; cerezler sekmede yasar).
//
// Akis (sitenin Next.js uygulamasi ile ayni):
//  1. POST /endpoints/OnlyHotelProduct/PriceSearchEncrypt {kriter} -> result.queryParams.sidp (arama kimligi)
//  2. GET  /_next/data/<buildId>/<slug>.json?sidp=<sidp>&availability=2&p=2&pagePath=<slug>
//     -> pageProps.layout ... widget(widgetType=12 "Hotel Detail Widget").widgetData.hotelDetailData
//     .onlyHotelPriceSearchResponse {hotelDetail, rooms{roomKey:{name}}, meals{mealKey:{name}}, products[]}
//     Her product bir oda+pansiyon teklifidir: rooms[0].roomKey/mealKey/stopSale, price{oldAmount, amount,
//     discountPercent, currency}, availability (1 = satista).
//  3. Isimle arama: POST /endpoints/OnlyHotelProduct/ListArrivalLocations {text} -> locations (type 7 = otel).
//     buildId sayfadaki __NEXT_DATA__'dan okunur; site deploy edince degisir (veri rotasi 404 -> sayfa yenilenir).
package coraltatil

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"pricebot/internal/browser"
)

const (
	siteBase = "https://www.coraltatil.com"
	// challengeWait Imperva dogrulamasinin tamamlanmasi icin ust sinir.
	challengeWait = 30 * time.Second
	opTimeout     = 60 * time.Second
)

// criteria sitenin PriceSearch kriter govdesi (yalnizca zorunlu alanlar).
type passenger struct {
	PassengerType   int     `json:"passengerType"`
	BirthDate       *string `json:"birthDate"`
	Age             int     `json:"age"`
	IsChildOrInfant bool    `json:"isChildOrInfant"`
}

type roomCriteria struct {
	Passengers              []passenger `json:"passengers"`
	AdultPassengerCount     int         `json:"adultPassengerCount"`
	ChildOrInfantCount      int         `json:"childOrInfantCount"`
	ChildOrInfantPassengers []passenger `json:"childOrInfantPassengers"`
	AdultPassengers         []passenger `json:"adultPassengers"`
}

type arrivalLocation struct {
	ID               string   `json:"id"`
	LocationID       string   `json:"locationId"`
	LocationUniqueID string   `json:"locationUniqueId"`
	Type             int      `json:"type"`
	Name             string   `json:"name"`
	FriendlyURL      string   `json:"friendlyUrl"`
	CountryIDs       []string `json:"countryIds,omitempty"`
}

type criteria struct {
	ReservationType   int               `json:"reservationType"`
	BeginDates        []string          `json:"beginDates"`
	RoomCriterias     []roomCriteria    `json:"roomCriterias"`
	Nights            []map[string]int  `json:"nights"`
	ArrivalLocations  []arrivalLocation `json:"arrivalLocations"`
	Paging            map[string]int    `json:"paging"`
	ImageSizes        []int             `json:"imageSizes"`
	AdditionalFilters []any             `json:"additionalFilters"`
}

func buildCriteria(hotelID, slug, name, checkIn string, nights, adults int, childAges []int) criteria {
	adultsP := make([]passenger, 0, adults)
	for i := 0; i < adults; i++ {
		adultsP = append(adultsP, passenger{PassengerType: 0, Age: 20})
	}
	kids := make([]passenger, 0, len(childAges))
	for _, a := range childAges {
		kids = append(kids, passenger{PassengerType: 1, Age: a, IsChildOrInfant: true})
	}
	all := append(append([]passenger{}, adultsP...), kids...)
	return criteria{
		ReservationType: 2, BeginDates: []string{checkIn, checkIn},
		RoomCriterias:    []roomCriteria{{Passengers: all, AdultPassengerCount: adults, ChildOrInfantCount: len(kids), ChildOrInfantPassengers: kids, AdultPassengers: adultsP}},
		Nights:           []map[string]int{{"value": nights}},
		ArrivalLocations: []arrivalLocation{{ID: hotelID + "-7", LocationID: hotelID, LocationUniqueID: hotelID + "-7", Type: 7, Name: name, FriendlyURL: slug}},
		Paging:           map[string]int{"pageNumber": 0, "pageSize": 20, "sortType": 3},
		ImageSizes:       []int{4}, AdditionalFilters: []any{},
	}
}

// ---- Sayfa icinde calisan JS ----

const jsHeaders = `{ "Accept": "application/json, text/plain, */*", "Content-Type": "application/json", "CurrencyCode": "TRY", "DepartureId": "63-5", "LanguageCode": "tr-TR", "MobileType": "Web", "accept-language": "tr-TR" }`

// jsReady challenge gecildi mi (gercek sayfa govdesi var mi).
const jsReady = `(function(){ var t = (document.body && document.body.innerText) || ""; return t.length > 200 && !/_Incapsula_Resource/.test(document.documentElement.outerHTML.slice(0, 3000)) && !!document.getElementById("__NEXT_DATA__"); })()`

// jsPrice: kriteri sifreler, veri rotasini ceker, otel detay widget'indan teklifleri doner.
func jsPrice(slug string, crit criteria) string {
	cb, _ := json.Marshal(crit)
	return `(async () => {
  const H = ` + jsHeaders + `;
  const slug = ` + jsStr(slug) + `;
  let build = ""; try { build = JSON.parse(document.getElementById("__NEXT_DATA__").textContent).buildId; } catch (e) { return { error: "buildId okunamadi" }; }
  const er = await fetch("/endpoints/OnlyHotelProduct/PriceSearchEncrypt", { method: "POST", headers: { ...H, "X-Correlation-ID": crypto.randomUUID() }, body: ` + jsStr(string(cb)) + ` });
  const et = await er.text();
  if (!er.ok) return { error: "PriceSearchEncrypt " + er.status + ": " + et.slice(0, 200) };
  let sidp = ""; try { sidp = JSON.parse(et).result.queryParams.sidp; } catch (e) {}
  if (!sidp) return { error: "arama kimligi (sidp) alinamadi: " + et.slice(0, 200) };
  const dr = await fetch("/_next/data/" + build + "/" + slug + ".json?sidp=" + sidp + "&availability=2&p=2&pagePath=" + slug, { headers: { "x-nextjs-data": "1" } });
  const dt = await dr.text();
  if (dr.status === 404 && dt.length < 2000) return { error: "data route 404", stale: true };
  let d; try { d = JSON.parse(dt); } catch (e) { return { error: "veri rotasi JSON degil: " + dt.slice(0, 120) }; }
  let w = null; const walk = (o, depth) => { if (!o || depth > 14 || w) return; if (Array.isArray(o)) { o.forEach((x) => walk(x, depth + 1)); return; } if (typeof o === "object") { if (o.widgetType === 12) { w = o; return; } for (const k of Object.keys(o)) walk(o[k], depth + 1); } };
  walk(d.pageProps && d.pageProps.layout, 0);
  const hd = w && w.widgetData && w.widgetData.hotelDetailData;
  const R = hd && hd.onlyHotelPriceSearchResponse;
  if (!R) return { error: "otel detay widget'i bulunamadi (sayfa tipi otel degil?)", notFound: true, title: d.pageProps && d.pageProps.meta && d.pageProps.meta.title };
  const rooms = {}; for (const k of Object.keys(R.rooms || {})) rooms[k] = (R.rooms[k] && R.rooms[k].name) || "";
  const meals = {}; for (const k of Object.keys(R.meals || {})) meals[k] = (R.meals[k] && R.meals[k].name) || "";
  const h = R.hotelDetail || {};
  return {
    hotel: { id: String(h.id || ""), name: h.name || "", friendlyUrl: h.friendlyUrl || "", locationSummary: h.locationSummary || "", place: h.place && h.place.name || "", star: h.star || null },
    rooms, meals,
    products: (R.products || []).map((p) => ({ roomKey: p.rooms && p.rooms[0] && p.rooms[0].roomKey || "", mealKey: p.rooms && p.rooms[0] && p.rooms[0].mealKey || "", stopSale: !!(p.rooms && p.rooms[0] && p.rooms[0].stopSale), availability: p.availability, checkIn: p.checkInDate, checkOut: p.checkOutDate, nights: p.stayNights, oldAmount: p.price && p.price.oldAmount, amount: p.price && p.price.amount, discountPercent: p.price && p.price.discountPercent, currency: p.price && p.price.currency || "TRY" })),
  };
})()`
}

// jsResolve: slug -> otel kimligi/adi (sidp'siz veri rotasi).
func jsResolve(slug string) string {
	return `(async () => {
  const slug = ` + jsStr(slug) + `;
  let build = ""; try { build = JSON.parse(document.getElementById("__NEXT_DATA__").textContent).buildId; } catch (e) { return { error: "buildId okunamadi" }; }
  const dr = await fetch("/_next/data/" + build + "/" + slug + ".json?pagePath=" + slug, { headers: { "x-nextjs-data": "1" } });
  const dt = await dr.text();
  if (dr.status === 404 && dt.length < 2000) return { error: "data route 404", stale: true };
  let d; try { d = JSON.parse(dt); } catch (e) { return { error: "veri rotasi JSON degil" }; }
  let w = null; const walk = (o, depth) => { if (!o || depth > 14 || w) return; if (Array.isArray(o)) { o.forEach((x) => walk(x, depth + 1)); return; } if (typeof o === "object") { if (o.widgetType === 12) { w = o; return; } for (const k of Object.keys(o)) walk(o[k], depth + 1); } };
  walk(d.pageProps && d.pageProps.layout, 0);
  const R = w && w.widgetData && w.widgetData.hotelDetailData && w.widgetData.hotelDetailData.onlyHotelPriceSearchResponse;
  const h = (R && R.hotelDetail) || null;
  if (!h) return { error: "otel bulunamadi (URL otel detay sayfasi olmali)", notFound: true, title: d.pageProps && d.pageProps.meta && d.pageProps.meta.title };
  return { hotel: { id: String(h.id || ""), name: h.name || "", friendlyUrl: h.friendlyUrl || "", locationSummary: h.locationSummary || "", place: h.place && h.place.name || "", star: h.star || null } };
})()`
}

// jsSearch: isimle otel arama (type 7 = otel).
func jsSearch(text string) string {
	return `(async () => {
  const H = ` + jsHeaders + `;
  const r = await fetch("/endpoints/OnlyHotelProduct/ListArrivalLocations", { method: "POST", headers: { ...H, "X-Correlation-ID": crypto.randomUUID() }, body: JSON.stringify({ text: ` + jsStr(text) + ` }) });
  const t = await r.text();
  if (!r.ok) return { error: "ListArrivalLocations " + r.status };
  let d; try { d = JSON.parse(t); } catch (e) { return { error: "JSON degil" }; }
  const locs = (d.result && d.result.locations) || [];
  return { locations: locs.filter((l) => l.type === 7).map((l) => ({ id: String(l.id || ""), name: l.name || "", friendlyUrl: l.friendlyUrl || "", parent: l.parent && l.parent.name || "" })) };
})()`
}

func jsStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// ---- Yanit tipleri ----

type hotelInfo struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	FriendlyURL     string `json:"friendlyUrl"`
	LocationSummary string `json:"locationSummary"`
	Place           string `json:"place"`
}

type product struct {
	RoomKey         string   `json:"roomKey"`
	MealKey         string   `json:"mealKey"`
	StopSale        bool     `json:"stopSale"`
	Availability    int      `json:"availability"`
	CheckIn         string   `json:"checkIn"`
	CheckOut        string   `json:"checkOut"`
	Nights          int      `json:"nights"`
	OldAmount       *float64 `json:"oldAmount"`
	Amount          *float64 `json:"amount"`
	DiscountPercent float64  `json:"discountPercent"`
	Currency        string   `json:"currency"`
}

// PriceData sayfa icindeki JS'in dondurdugu teklif verisi.
type PriceData struct {
	Error    string            `json:"error"`
	Stale    bool              `json:"stale"`
	NotFound bool              `json:"notFound"`
	Hotel    *hotelInfo        `json:"hotel"`
	Rooms    map[string]string `json:"rooms"`
	Meals    map[string]string `json:"meals"`
	Products []product         `json:"products"`
}

type searchData struct {
	Error     string `json:"error"`
	Locations []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		FriendlyURL string `json:"friendlyUrl"`
		Parent      string `json:"parent"`
	} `json:"locations"`
}

// ---- Kalici sekme ----

type session struct {
	bm       *browser.Manager
	maxAge   time.Duration
	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	openedAt time.Time
	uses     int
}

func newSession(bm *browser.Manager, maxAge time.Duration) *session {
	if maxAge <= 0 {
		maxAge = 20 * time.Minute
	}
	return &session{bm: bm, maxAge: maxAge}
}

func (s *session) close() {
	if s.cancel != nil {
		s.cancel()
	}
	s.ctx, s.cancel, s.uses = nil, nil, 0
}

// ensure sekmeyi acar / ana sayfaya getirir ve Imperva dogrulamasinin gecmesini bekler.
func (s *session) ensure(force bool) error {
	alive := s.ctx != nil && s.ctx.Err() == nil
	if alive && (force || time.Since(s.openedAt) > s.maxAge || s.uses >= 40) {
		s.close()
		alive = false
	}
	if alive {
		return nil
	}
	if s.bm == nil {
		return fmt.Errorf("Coral Tatil icin headless tarayici gerekli (browser manager yok)")
	}
	ctx, cancel, err := s.bm.NewPersistentPage(12 * time.Hour)
	if err != nil {
		return fmt.Errorf("Coral Tatil sekmesi acilamadi: %w", err)
	}
	s.ctx, s.cancel, s.openedAt, s.uses = ctx, cancel, time.Now(), 0
	op, c := context.WithTimeout(ctx, 90*time.Second)
	defer c()
	if err := browser.Navigate(op, siteBase+"/", 1500*time.Millisecond); err != nil {
		s.close()
		return fmt.Errorf("Coral Tatil ana sayfasi acilamadi: %w", err)
	}
	deadline := time.Now().Add(challengeWait)
	for {
		var ok bool
		_ = browser.Eval(op, jsReady, &ok)
		if ok {
			return nil
		}
		if time.Now().After(deadline) {
			s.close()
			return fmt.Errorf("Coral Tatil bot dogrulamasi (Imperva) %s icinde gecilemedi", challengeWait)
		}
		if err := browser.Sleep(op, 500*time.Millisecond); err != nil {
			s.close()
			return err
		}
	}
}

// evalAwait async JS (Promise) calistirir ve JSON sonucu out'a cozer.
func evalAwait(ctx context.Context, js string, out any) error {
	var raw json.RawMessage
	err := chromedp.Run(ctx, chromedp.Evaluate(js, &raw, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true).WithReturnByValue(true)
	}))
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// run sekmede JS calistirir; "stale" (deploy sonrasi eski buildId) veya kopuk sekmede bir kez yenileyip tekrar dener.
func (s *session) run(ctx context.Context, js string, out any, isStale func() bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if err := s.ensure(attempt > 0); err != nil {
			return err
		}
		op, cancel := context.WithTimeout(s.ctx, opTimeout)
		stop := context.AfterFunc(ctx, cancel)
		err := evalAwait(op, js, out)
		stop()
		cancel()
		s.uses++
		if err != nil {
			lastErr = fmt.Errorf("Coral Tatil sayfa betigi calismadi: %w", err)
			continue
		}
		if isStale != nil && isStale() {
			lastErr = fmt.Errorf("Coral Tatil veri rotasi eskidi (yeni deploy); sayfa yenilendi")
			continue
		}
		return nil
	}
	return lastErr
}

var reSlug = regexp.MustCompile(`(?i)^https?://(?:www\.)?coraltatil\.com/([a-z0-9][a-z0-9\-]*)/?(?:[?#].*)?$`)

// slugFromURL https://www.coraltatil.com/<slug>/?sidp=... -> slug
func slugFromURL(u string) string {
	m := reSlug.FindStringSubmatch(strings.TrimSpace(u))
	if m == nil {
		return ""
	}
	return strings.ToLower(m[1])
}
