package touristica

import (
	"context"
	"encoding/json"
	"fmt"
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

const jsClickPackageTab = `(function(){
  var tab = Array.from(document.querySelectorAll("a,button,li")).find(function(e){
    var t = (e.textContent || "").trim();
    return /otel\s*\+\s*uçak/i.test(t) && t.length < 60;
  });
  if (tab) tab.click();
  return true;
})()`

// Oda/paket bloklarini, otel adini ve sehri okur.
const jsReadBlocks = `(function(){
  var roomBlocks = [], packageBlocks = [];
  document.querySelectorAll('[class*="room"],[class*="package"]').forEach(function(el){
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

// openAndSearch sayfayi acar, cerezleri kabul eder, tarih/kisi doldurup "Oda Ara" tetikler ve fiyatlari bekler.
func openAndSearch(ctx context.Context, detailURL, checkIn, checkOut string, adults, children int, maxWait int) error {
	if err := browser.Navigate(ctx, detailURL, 0); err != nil {
		return err
	}
	_ = browser.Eval(ctx, jsAcceptCookies, nil)
	if err := browser.Sleep(ctx, 1500*time.Millisecond); err != nil {
		return err
	}
	if err := browser.Eval(ctx, jsFillAndSearch(toTouristicaDate(checkIn), toTouristicaDate(checkOut), adults, children), nil); err != nil {
		return err
	}
	for i := 0; i < maxWait; i++ {
		if err := browser.Sleep(ctx, time.Second); err != nil {
			return err
		}
		var ready bool
		if err := browser.Eval(ctx, jsPricesReady, &ready); err == nil && ready {
			break
		}
	}
	return nil
}

// waitForPackages paket bloklari gorunene kadar bekler; 3. ve 10. saniyede "Otel + Ucak" sekmesini tetikler.
func waitForPackages(ctx context.Context, maxSeconds int) bool {
	for i := 0; i < maxSeconds; i++ {
		var has bool
		if err := browser.Eval(ctx, jsHasPackages, &has); err == nil && has {
			return true
		}
		if i == 3 || i == 10 {
			_ = browser.Eval(ctx, jsClickPackageTab, nil)
		}
		if err := browser.Sleep(ctx, time.Second); err != nil {
			return false
		}
	}
	return false
}

func (p *Provider) fetchRoomPrices(ctx context.Context, detailURL, checkIn, checkOut string, adults, children int) (*RawPrice, error) {
	page, cancel, err := p.browser.NewPage(ctx, 150*time.Second)
	if err != nil {
		return nil, err
	}
	defer cancel()

	if err := openAndSearch(page, detailURL, checkIn, checkOut, adults, children, 15); err != nil {
		return nil, fmt.Errorf("Touristica sayfasi acilamadi: %w", err)
	}
	var raw RawPrice
	if err := browser.Eval(page, jsReadBlocks, &raw); err != nil {
		return nil, fmt.Errorf("Touristica fiyat bloklari okunamadi: %w", err)
	}
	return &raw, nil
}

func (p *Provider) fetchPackagePrices(ctx context.Context, detailURL, checkIn, checkOut string, adults, children int) ([]PackageCard, *string, error) {
	page, cancel, err := p.browser.NewPage(ctx, 180*time.Second)
	if err != nil {
		return nil, nil, err
	}
	defer cancel()

	if err := openAndSearch(page, detailURL, checkIn, checkOut, adults, children, 20); err != nil {
		return nil, nil, fmt.Errorf("Touristica sayfasi acilamadi: %w", err)
	}
	waitForPackages(page, 25)

	var data struct {
		Cards []struct {
			RoomName  *string `json:"roomName"`
			BoardType *string `json:"boardType"`
			PriceText string  `json:"priceText"`
		} `json:"cards"`
		HotelName *string `json:"hotelName"`
	}
	if err := browser.Eval(page, jsReadPackageCards, &data); err != nil {
		return nil, nil, fmt.Errorf("Touristica paket kartlari okunamadi: %w", err)
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
