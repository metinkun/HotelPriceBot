package touristica

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"pricebot/internal/browser"
	"pricebot/internal/provider"
)

// Touristica Cloudflare challenge (cf-mitigated: challenge) arkasinda.
// cf_clearance cookie'si TLS parmak izine bagli oldugu icin HTTP istemcisine
// tasinamaz -> tum istekler headless tarayici icinden yapilir.

// toTouristicaDate yyyy-MM-dd -> dd.MM.yyyy
func toTouristicaDate(iso string) string {
	p := strings.Split(iso[:min(10, len(iso))], "-")
	if len(p) != 3 {
		return iso
	}
	return p[2] + "." + p[1] + "." + p[0]
}

type RawPrice struct {
	RoomBlocks    []string `json:"roomBlocks"`
	PackageBlocks []string `json:"packageBlocks"`
	HotelName     *string  `json:"hotelName"`
	City          *string  `json:"city"`
}

type Resolved struct {
	ProviderHotelID string
	HotelName       *string
	City            *string
	Slug            string
}

type PackageCard struct {
	RoomName  *string
	BoardType *string
	Price     *float64
}

func jsArg(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// Cerez onay banner'ini kapatir (fiyat panelini kapatiyor).
const jsAcceptCookies = `(function(){
  var b = Array.from(document.querySelectorAll("button,a")).find(function(x){ return /tümünü kabul/i.test(x.textContent || ""); });
  if (b) b.click();
  return true;
})()`

// Tarih/kisi doldurup "Oda Ara" tetikler.
func jsFillAndSearch(inDate, outDate string, adults, children int) string {
	return `(function(inDate, outDate, ad, ch){
  var set = function(sel, val){
    var el = document.querySelector(sel);
    if (el) { el.value = val; el.setAttribute("data-value", val); el.dispatchEvent(new Event("change", {bubbles:true})); }
  };
  set("#txtCheckInDate", inDate);
  set("#txtCheckOutDate", outDate);
  var adultEl = document.querySelector('[data-model-prop="AdultCount"], #ddlAdultCount');
  if (adultEl) { adultEl.value = String(ad); adultEl.dispatchEvent(new Event("change", {bubbles:true})); }
  var childEl = document.querySelector('[data-model-prop="ChildCount"], #ddlChildCount');
  if (childEl) { childEl.value = String(ch); childEl.dispatchEvent(new Event("change", {bubbles:true})); }
  var btn = Array.from(document.querySelectorAll("button,a,input")).find(function(b){ return /oda ara/i.test(b.textContent || b.value || ""); });
  if (btn) btn.click();
  return true;
})(` + jsArg(inDate) + `,` + jsArg(outDate) + `,` + jsArg(adults) + `,` + jsArg(children) + `)`
}

const jsPricesReady = `/Gecelik Toplam Fiyat|Otel \+ Uçak|Fırsat Paketi/i.test((document.body && document.body.innerText) || "")`

const jsHasPackages = `/Otel \+ Uçak \+ Transfer Dahil\s*[\d.,]+\s*TL/i.test((document.body && document.body.innerText) || "")`

// Paket sekmesi: once sayfanin kendi sekme linki (aramadan sonra belirir). Gercek href'li linklere
// (orn. ust menudeki "ÖZEL DÖNEM(OTEL+UÇAK)" -> /kibris-turlari) ASLA tiklanmaz; yoksa sayfa degisir.
const jsClickPackageTab = `(function(){
  var tab = document.querySelector('a.nav-hotel-detail-room-and-flight-tab, a[href="#hotel-detail-room-and-flight-tab"], [data-target="#hotel-detail-room-and-flight-tab"]');
  if (!tab) {
    tab = Array.from(document.querySelectorAll("a,button,li")).find(function(e){
      var t = (e.textContent || "").trim();
      var href = e.getAttribute && (e.getAttribute("href") || "");
      return /otel\s*\+\s*uçak/i.test(t) && t.length < 60 && (href === "" || href.charAt(0) === "#");
    });
  }
  if (tab) { tab.click(); return tab.tagName + "." + tab.className; }
  return "";
})()`

// Oda/paket bloklarini, otel adini ve sehri okur.
const jsReadBlocks = `(function(){
  var roomBlocks = [], packageBlocks = [];
  document.querySelectorAll('[class*="room"],[class*="package"]').forEach(function(el){
    if (el.hasAttribute("data-pb-stale")) return; // onceki aramanin kalintisi
    var t = (el.innerText || "").replace(/\s+/g, " ").trim();
    if (!/TL|₺/.test(t) || t.length < 25 || t.length > 300) return;
    if (/Otel \+ Uçak/i.test(t)) packageBlocks.push(t);
    else if (/Gecelik Toplam Fiyat/i.test(t)) roomBlocks.push(t);
  });
  var h1 = document.querySelector("h1");
  var crumbBox = document.querySelector('[class*="breadcrumb"], [class*="bread-crumb"]');
  var crumbLinks = crumbBox ? Array.from(crumbBox.querySelectorAll("a")).filter(function(a){ return /Otelleri$/.test((a.textContent || "").trim()); }) : [];
  var crumb = crumbLinks.length ? crumbLinks[crumbLinks.length - 1] : null;
  return {
    roomBlocks: Array.from(new Set(roomBlocks)).slice(0, 20),
    packageBlocks: Array.from(new Set(packageBlocks)).slice(0, 10),
    hotelName: h1 ? (h1.textContent || "").trim() : null,
    city: crumb ? (crumb.textContent || "").trim().replace(/\s*Otelleri$/i, "") : null
  };
})()`

// Paket (Otel + Ucak + Transfer) kartlarini okur.
const jsReadPackageCards = `(function(){
  var BOARDS = ["Ultra Her Şey Dahil","Her Şey Dahil","Tam Pansiyon Plus","Tam Pansiyon","Yarım Pansiyon","Oda Kahvaltı","Sadece Oda"];
  var seen = {}; var cards = [];
  document.querySelectorAll("*").forEach(function(el){
    if (el.hasAttribute("data-pb-stale")) return; // onceki aramanin karti
    var t = (el.innerText || "").replace(/\s+/g, " ").trim();
    if (!/Otel \+ Uçak \+ Transfer Dahil/i.test(t)) return;
    if (t.length > 400) return;
    var priceM = t.match(/Otel \+ Uçak \+ Transfer Dahil\s*([\d.,]+)\s*TL/i);
    if (!priceM) return;
    var roomName = null; var cardText = t; var p = el;
    for (var i = 0; i < 10 && p; i++) {
      p = p.parentElement;
      if (!p) break;
      var head = p.querySelector('h4.accommodation-type, h2,h3,h4,[class*="room-name"],[class*="room-title"]');
      if (head) {
        var ht = (head.textContent || "").replace(/\s+/g, " ").trim();
        if (ht && ht.length < 70) { roomName = ht; cardText = (p.innerText || "").replace(/\s+/g, " ").trim(); break; }
      }
    }
    var boardType = null;
    for (var j = 0; j < BOARDS.length; j++) { if (cardText.indexOf(BOARDS[j]) !== -1) { boardType = BOARDS[j]; break; } }
    if (roomName && boardType && roomName.indexOf(boardType) !== -1) { roomName = roomName.split(boardType)[0].trim(); }
    var key = String(roomName) + "|" + String(boardType) + "|" + priceM[1];
    if (seen[key]) return;
    seen[key] = true;
    cards.push({ roomName: roomName, boardType: boardType, priceText: priceM[1] });
  });
  var h1 = document.querySelector("h1");
  return { cards: cards, hotelName: h1 ? (h1.textContent || "").trim() : null };
})()`

// Otel kimligi/slug/ad/sehir (resolve icin).
const jsResolveInfo = `(function(){
  var w = window; var detail = w.PageURLDetail || {};
  var h1 = document.querySelector("h1");
  var id = null; var dl = w.dataLayer;
  if (Array.isArray(dl)) {
    for (var i = 0; i < dl.length; i++) {
      var entry = dl[i];
      var items = (entry && entry.ecommerce && (entry.ecommerce.items || (entry.ecommerce.detail && entry.ecommerce.detail.products))) || null;
      if (Array.isArray(items) && items[0]) { id = String(items[0].item_id != null ? items[0].item_id : (items[0].id != null ? items[0].id : "")); if (id) break; }
    }
  }
  if (!id) { var el = document.querySelector("[data-hotel-id]"); if (el) id = el.getAttribute("data-hotel-id"); }
  var crumbBox = document.querySelector('[class*="breadcrumb"], [class*="bread-crumb"]');
  var crumbLinks = crumbBox ? Array.from(crumbBox.querySelectorAll("a")).filter(function(a){ return /Otelleri$/.test((a.textContent || "").trim()); }) : [];
  var crumb = crumbLinks.length ? crumbLinks[crumbLinks.length - 1] : null;
  return {
    id: id || null,
    slug: detail.URL || null,
    name: detail.URLNAME || (h1 ? (h1.textContent || "").trim() : null),
    isHotelDetail: !!detail.IsHotelDetailPage,
    city: crumb ? (crumb.textContent || "").trim().replace(/\s*Otelleri$/i, "") : null
  };
})()`

// searchOnTab acik sekmede tarih/kisi doldurup "Oda Ara" tetikler; /odalar ajax yaniti gelince doner.
// Kanca varsa yalnizca ajax sayaci beklenir (eski fiyat metni yaniltmasin); yoksa metin tabanli bekleme.
func searchOnTab(ctx context.Context, hooked bool, checkIn, checkOut string, adults, children int) error {
	_ = browser.Eval(ctx, jsMarkStale, nil)
	before := counter(ctx, "odalar")
	if err := browser.Eval(ctx, jsFillAndSearch(toTouristicaDate(checkIn), toTouristicaDate(checkOut), adults, children), nil); err != nil {
		return err
	}
	fallback := ""
	if !hooked {
		fallback = jsFreshPricesReady
	}
	if !waitCounter(ctx, "odalar", before, fallback, 25*time.Second) {
		return fmt.Errorf("Touristica oda listesi zamaninda gelmedi")
	}
	// Ajax handler'inin DOM'u yazmasi icin kisa pay
	if err := browser.Sleep(ctx, 400*time.Millisecond); err != nil {
		return err
	}
	// Tarihlerin gercekten uygulandigini dogrula (sayfa kendi formatinda tutar)
	var applied struct {
		In  string `json:"in"`
		Out string `json:"out"`
	}
	_ = browser.Eval(ctx, `({in: (document.querySelector("#txtCheckInDate")||{}).value || "", out: (document.querySelector("#txtCheckOutDate")||{}).value || ""})`, &applied)
	if applied.In != "" && applied.In != toTouristicaDate(checkIn) {
		slog.Warn("touristica: sayfadaki giris tarihi istenenle eslesmiyor", "istenen", toTouristicaDate(checkIn), "sayfa", applied.In)
	}
	return nil
}

// Isaretlenmemis (taze) fiyat blogu var mi? (jQuery kancasi olmayan sayfalar icin)
const jsFreshPricesReady = `(function(){
  var ok = false;
  document.querySelectorAll('[class*="room"],[class*="package"]').forEach(function(el){
    if (ok || el.hasAttribute("data-pb-stale")) return;
    var t = (el.innerText || "").replace(/\s+/g, " ").trim();
    if (/Gecelik Toplam Fiyat/i.test(t) && t.length < 300) ok = true;
  });
  return ok;
})()`

// waitForPackages paket kartlarini bekler. Gozlem (debug akisi): taze yuklenmis sayfada arama,
// paketleri de OTOMATIK yukler (`/ucak-ve-odalar?ojr=true`); sekmeye tiklamak paneli gizleyip sonraki
// aramada otomatik yuklemeyi bozar. Bu yuzden once tiklamadan beklenir; gelmezse bir kez sekme denenir.
// before: arama ONCESI pkg sayaci. Donus: (bulundu, sayfaDegisti).
func waitForPackages(ctx context.Context, hooked bool, before int, max time.Duration) (bool, bool) {
	navigated := false
	clicked := false
	deadline := time.Now().Add(max)
	for i := 0; time.Now().Before(deadline); i++ {
		var state struct {
			Hooked bool   `json:"hooked"`
			Pkg    int    `json:"pkg"`
			Ready  bool   `json:"ready"`
			Has    bool   `json:"has"`
			Fresh  bool   `json:"fresh"`
			Title  string `json:"title"`
		}
		err := browser.Eval(ctx, `({
  hooked: !!(window.__pb && window.__pb.hooked),
  pkg: (window.__pb && window.__pb.pkg) || 0,
  ready: document.readyState === "complete",
  has: `+jsHasPackages+`,
  fresh: `+jsFreshPackagesReady+`,
  title: document.title || ""
})`, &state)
		if err != nil {
			// Navigasyon sirasinda evaluate basarisiz olur: bekle ve tekrar dene
			navigated = true
			if e := browser.Sleep(ctx, 400*time.Millisecond); e != nil {
				return false, navigated
			}
			continue
		}
		if !state.Hooked {
			navigated = true // kanca gitti -> yeni sayfa
		}
		switch {
		case navigated:
			// Yeni sayfada bayat isaret yok; hazir olup paket metni gelince bitir (Cloudflare sayfasi degilse)
			if state.Ready && state.Has && !strings.Contains(strings.ToLower(state.Title), "just a moment") {
				_ = browser.Sleep(ctx, 400*time.Millisecond)
				return true, true
			}
		// Paket istegi jQuery disinda da gelebilir: kanca sayaci VEYA taze (isaretsiz) paket metni yeterli
		case hooked && state.Pkg > before, state.Fresh:
			_ = browser.Sleep(ctx, 400*time.Millisecond)
			return true, false
		}
		if i == 30 && !clicked { // ~12 sn icinde otomatik gelmediyse bir kez sekmeyi dene
			clicked = true
			_ = browser.Eval(ctx, jsClickPackageTab, nil)
		}
		if e := browser.Sleep(ctx, 400*time.Millisecond); e != nil {
			return false, navigated
		}
	}
	return false, navigated
}

const jsFreshPackagesReady = `(function(){
  var ok = false;
  document.querySelectorAll("*").forEach(function(el){
    if (ok || el.hasAttribute("data-pb-stale")) return;
    var t = (el.innerText || "").replace(/\s+/g, " ").trim();
    if (t.length < 400 && /Otel \+ Uçak \+ Transfer Dahil\s*[\d.,]+\s*TL/i.test(t)) ok = true;
  });
  return ok;
})()`

// withTab havuzdan sekme alir, sayfayi hazirlar ve fn'i calistirir; hata halinde sekme yenilenir.
func (p *Provider) withTab(ctx context.Context, detailURL string, fn func(page context.Context, t *tab) error) error {
	return p.withTabNav(ctx, detailURL, false, fn)
}

// withTabNav forceNav true ise sayfa her durumda yeniden yuklenir (oturum/cerez sekmede kalir).
func (p *Provider) withTabNav(ctx context.Context, detailURL string, forceNav bool, fn func(page context.Context, t *tab) error) error {
	t, err := p.pool.acquire(ctx)
	if err != nil {
		return err
	}
	broken := false
	defer func() { p.pool.release(t, broken) }()

	if forceNav {
		t.url = ""
	}
	if err := p.pool.ensure(t, detailURL); err != nil {
		broken = true
		return err
	}
	opCtx, cancel := context.WithTimeout(t.ctx, 120*time.Second)
	defer cancel()
	// Istek iptal olursa islemi de iptal et (sekme kalir)
	stop := context.AfterFunc(ctx, cancel)
	defer stop()

	if err := fn(opCtx, t); err != nil {
		// Sekme olmus, zaman asimi veya arama sonucu gelmedi: bir sonraki kullanim icin sayfayi yenile
		broken = true
		return err
	}
	// Sayfa beklenmedik sekilde baska bir URL'e gittiyse sonraki sorguda otel sayfasina geri don
	var href string
	if browser.Eval(opCtx, `location.href`, &href) == nil && href != "" && !strings.HasPrefix(href, detailURL) {
		t.url = ""
	}
	return nil
}

func (p *Provider) fetchRoomPrices(ctx context.Context, detailURL, checkIn, checkOut string, adults, children int) (*RawPrice, error) {
	var raw RawPrice
	err := p.withTab(ctx, detailURL, func(page context.Context, t *tab) error {
		if err := searchOnTab(page, t.hooked, checkIn, checkOut, adults, children); err != nil {
			return err
		}
		if err := browser.Eval(page, jsReadBlocks, &raw); err != nil {
			return fmt.Errorf("Touristica fiyat bloklari okunamadi: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &raw, nil
}

func (p *Provider) fetchPackagePrices(ctx context.Context, detailURL, checkIn, checkOut string, adults, children int) ([]PackageCard, *string, error) {
	var data struct {
		Cards []struct {
			RoomName  *string `json:"roomName"`
			BoardType *string `json:"boardType"`
			PriceText string  `json:"priceText"`
		} `json:"cards"`
		HotelName *string `json:"hotelName"`
	}
	// Paket sorgusu temiz sayfa durumu ister (bkz. waitForPackages): sekmeye yeniden navigasyon.
	err := p.withTabNav(ctx, detailURL, true, func(page context.Context, t *tab) error {
		before := counter(page, "pkg")
		if err := searchOnTab(page, t.hooked, checkIn, checkOut, adults, children); err != nil {
			return err
		}
		waitForPackages(page, t.hooked, before, 30*time.Second)
		t.url = "" // paket sonrasi sayfa durumu degisti: sonraki sorgu otel sayfasini yeniden yuklesin
		if err := browser.Eval(page, jsReadPackageCards, &data); err != nil {
			return fmt.Errorf("Touristica paket kartlari okunamadi: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	cards := make([]PackageCard, 0, len(data.Cards))
	for _, c := range data.Cards {
		cards = append(cards, PackageCard{RoomName: c.RoomName, BoardType: c.BoardType, Price: parseTL(c.PriceText)})
	}
	return cards, data.HotelName, nil
}

var reLastSeg = regexp.MustCompile(`[^/]+/?$`)

func (p *Provider) resolveHotelFromURL(ctx context.Context, u string) (*Resolved, error) {
	page, cancel, err := p.browser.NewPage(ctx, 120*time.Second)
	if err != nil {
		return nil, err
	}
	defer cancel()

	if err := browser.Navigate(page, u, 2*time.Second); err != nil {
		return nil, fmt.Errorf("Touristica sayfasi acilamadi: %w", err)
	}
	var info struct {
		ID            *string `json:"id"`
		Slug          *string `json:"slug"`
		Name          *string `json:"name"`
		IsHotelDetail bool    `json:"isHotelDetail"`
		City          *string `json:"city"`
	}
	if err := browser.Eval(page, jsResolveInfo, &info); err != nil {
		return nil, fmt.Errorf("Touristica sayfa bilgisi okunamadi: %w", err)
	}
	slug := ""
	if info.Slug != nil {
		slug = *info.Slug
	}
	if !info.IsHotelDetail && slug == "" {
		return nil, fmt.Errorf("Touristica otel detay sayfasi degil (URL kontrol edilmeli)")
	}
	id := ""
	if info.ID != nil {
		id = *info.ID
	}
	if id == "" {
		id = slug
	}
	if id == "" {
		return nil, fmt.Errorf("Touristica otel kimligi cozulemedi")
	}
	if slug == "" {
		slug = strings.TrimSuffix(reLastSeg.FindString(strings.SplitN(u, "?", 2)[0]), "/")
	}
	return &Resolved{ProviderHotelID: id, HotelName: info.Name, City: info.City, Slug: slug}, nil
}

// parseTL "51.330" -> 51330
func parseTL(s string) *float64 {
	return typesF64Pos(provider.ParseTRPrice(s))
}
