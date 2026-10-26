package touristica

// Statik otel detayi (DetailProvider) ve isimle arama (HotelSearcher).
//
// Site Cloudflare arkasinda oldugu icin her sey kalici sekme icinde calisir:
//   - Detay: sekmedeki otel sayfasindan ilgili HTML parcalari (baslik, galeri, "Otel Detaylari" sekmesi,
//     kampanyalar, etkinlikler, yorum puanlari, ld+json) okunur; oda listesi sayfanin kendi
//     `/<slug>/odalar` ucundan sayfa ici fetch ile alinir (fiyat aramasi TETIKLENMEZ, DOM degismez).
//     Ayristirma Go tarafinda goquery ile yapilir (details_parse.go, fixture testli).
//   - Arama: ana sayfanin arama kutusu `GET /anasayfa/otomatik-tamamlama?SearchType=hotel&SearchText=<q>`
//     HTML'ini sayfa ici fetch ile ceker (herhangi bir touristica sayfasindan calisir; sekme navigasyonu gerekmez).

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"pricebot/internal/browser"
	"pricebot/internal/provider"
)

const siteRoot = "https://www.touristica.com.tr"

// evalAwait Promise donduren JS'i calistirir ve sonucu bekler (browser.Eval promise beklemez).
func evalAwait(ctx context.Context, js string, out any) error {
	return chromedp.Run(ctx, chromedp.Evaluate(js, out, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	}))
}

// rawDetailPage sayfadan okunan ham parcalar (ayristirma Go tarafinda).
type rawDetailPage struct {
	URL      string `json:"url"`
	HotelID  string `json:"hotelId"`
	Slug     string `json:"slug"`
	PageHTML string `json:"pageHtml"`
	RoomsURL string `json:"roomsUrl"`
}

// Detay sayfasindan yalnizca ihtiyac duyulan kapsayicilarin HTML'ini toplar (DOM degistirilmez).
const jsReadDetailPage = `(function(){
  var pick = function(sel){ var e = document.querySelector(sel); return e ? e.outerHTML : ""; };
  var h1 = document.querySelector("h1");
  var header = "";
  if (h1) { var row = h1.closest(".row") || h1.parentElement; header = row ? row.outerHTML : h1.outerHTML; }
  var ld = Array.from(document.querySelectorAll('script[type="application/ld+json"]')).filter(function(s){
    return /"@type"\s*:\s*"Hotel"/.test(s.textContent || "");
  }).map(function(s){ return s.outerHTML; }).join("");
  var html = '<div id="pb-detail">' + header + ld + pick("#hotel-map") + pick(".detail-slider-wrapper") + pick(".gallery-v5") +
    pick("#hotel-details-tab") + pick("#HotelDetailActionTabPlaceHolder") + pick("#activities-tab") +
    '<div class="review-wrapper">' + pick("#review-tab .review-wrapper > .row") + '</div></div>'; // yalnizca puan satiri (yorum metinleri degil)
  var ph = document.querySelector("#HotelRoomListPlaceHolder");
  return {
    url: location.href,
    hotelId: window.HotelId != null ? String(window.HotelId) : "",
    slug: window.PageURL || "",
    pageHtml: html,
    roomsUrl: ph ? (ph.getAttribute("data-place-holder-target") || "") : ""
  };
})()`

type rawFetch struct {
	Status int    `json:"status"`
	HTML   string `json:"html"`
}

// jsFetchHTML sayfa icinden (cerezlerle) GET yapar; Promise doner.
func jsFetchHTML(u string) string {
	return `(async function(u){
  var r = await fetch(u, { headers: { "X-Requested-With": "XMLHttpRequest", "Accept": "*/*" }, credentials: "include" });
  var t = await r.text();
  return { status: r.status, html: t };
})(` + jsArg(u) + `)`
}

// GetDetails otel detay sayfasini ortak semaya normalize eder. Fiyat aramasi tetiklenmez; sekmedeki
// sayfa durumu degismediginden sonraki fiyat sorgulari ayni sekmeyi navigasyonsuz kullanabilir.
func (p *Provider) GetDetails(ctx context.Context, m *provider.Mapping) (*provider.HotelDetails, error) {
	src := m.MetaString("sourceUrl")
	if src == "" {
		if slug := m.MetaString("slug"); slug != "" {
			src = siteRoot + "/" + slug
		}
	}
	if src == "" {
		return nil, fmt.Errorf("Mapping metadata'da sourceUrl eksik (otel detay URL'i)")
	}
	var raw rawDetailPage
	var rooms rawFetch
	err := p.withTab(ctx, src, func(page context.Context, t *tab) error {
		if err := browser.Eval(page, jsReadDetailPage, &raw); err != nil {
			return fmt.Errorf("Touristica detay sayfasi okunamadi: %w", err)
		}
		if raw.PageHTML == "" || raw.Slug == "" {
			return fmt.Errorf("Touristica otel detay sayfasi degil (URL kontrol edilmeli)")
		}
		roomsURL := raw.RoomsURL
		if roomsURL == "" {
			roomsURL = siteRoot + "/" + raw.Slug + "/odalar?rpd=True&cc=&det=False&y=1c"
		}
		fctx, cancel := context.WithTimeout(page, 40*time.Second)
		defer cancel()
		if err := evalAwait(fctx, jsFetchHTML(roomsURL), &rooms); err != nil {
			return fmt.Errorf("Touristica oda listesi alinamadi: %w", err)
		}
		if rooms.Status != 200 {
			return fmt.Errorf("Touristica oda listesi HTTP %d", rooms.Status)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	d, err := parseDetails(raw.PageHTML, rooms.HTML)
	if err != nil {
		return nil, err
	}
	d.Provider = p.Name()
	d.SourceURL = src
	d.ProviderHotelID = m.ProviderHotelID
	if d.ProviderHotelID == "" {
		d.ProviderHotelID = raw.HotelID
	}
	if d.ProviderHotelID == "" {
		d.ProviderHotelID = raw.Slug
	}
	if d.Name == "" && m.HotelName != nil {
		d.Name = *m.HotelName
	}
	d.FetchedAt = time.Now().UTC().Truncate(time.Second)
	return d, nil
}

// withAnyTab havuzdan sekme alir; sekmede bir touristica sayfasi acik degilse ana sayfayi acar.
// Acik olan otel sayfasi korunur (sonraki fiyat sorgusu navigasyonsuz devam eder).
func (p *Provider) withAnyTab(ctx context.Context, fn func(page context.Context) error) error {
	t, err := p.pool.acquire(ctx)
	if err != nil {
		return err
	}
	broken := false
	defer func() { p.pool.release(t, broken) }()

	target := t.url
	if !t.alive() || target == "" {
		target = siteRoot + "/"
	}
	if err := p.pool.ensure(t, target); err != nil {
		broken = true
		return err
	}
	opCtx, cancel := context.WithTimeout(t.ctx, 60*time.Second)
	defer cancel()
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	if err := fn(opCtx); err != nil {
		broken = true
		return err
	}
	return nil
}

// SearchHotels sitenin arama kutusu otomatik tamamlamasiyla otel adaylarini doner (yalnizca OTEL sonuclari;
// "... Otelleri" kategorileri atlanir). Tam sorgu sonuc vermezse ASCII'ye indirgenmis ve kisaltilmis
// (son kelime atilmis) sorgular denenir.
func (p *Provider) SearchHotels(ctx context.Context, query string) ([]provider.HotelCandidate, error) {
	query = strings.Join(strings.Fields(query), " ")
	if query == "" {
		return []provider.HotelCandidate{}, nil
	}
	attempts := searchAttempts(query)
	var out []provider.HotelCandidate
	err := p.withAnyTab(ctx, func(page context.Context) error {
		for _, q := range attempts {
			var res rawFetch
			u := siteRoot + "/anasayfa/otomatik-tamamlama?SearchType=hotel&SearchText=" + queryEscape(q)
			fctx, cancel := context.WithTimeout(page, 20*time.Second)
			err := evalAwait(fctx, jsFetchHTML(u), &res)
			cancel()
			if err != nil {
				return fmt.Errorf("Touristica otomatik tamamlama alinamadi: %w", err)
			}
			if res.Status != 200 {
				return fmt.Errorf("Touristica otomatik tamamlama HTTP %d", res.Status)
			}
			if cands := parseAutocomplete(res.HTML); len(cands) > 0 {
				out = cands
				return nil
			}
		}
		out = []provider.HotelCandidate{}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
