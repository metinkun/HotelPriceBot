// Package details otel statik verisinin cekilmesi, saklanmasi, gunluk degisiklik kontrolu ve bildirimleri.
package details

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"pricebot/internal/provider"
)

// Change tek bir alan degisikligi.
type Change struct {
	Path string `json:"path"` // orn. "images[3]", "facilities.Plaj", "policies.checkIn", "sections.Genel Bilgiler"
	Kind string `json:"kind"` // added | removed | changed
	Old  any    `json:"old,omitempty"`
	New  any    `json:"new,omitempty"`
}

// normalizeForHash degisiklik tespitinde yoksayilacak alanlari temizler.
func normalizeForHash(d *provider.HotelDetails) map[string]any {
	b, _ := json.Marshal(d)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	delete(m, "fetchedAt")
	// Turetilmis gorunumler bolumlerden hesaplanir; hash'e dahil edilmez (eski snapshot'larla uyum)
	for _, k := range []string{"reviews", "scores", "events", "campaigns"} {
		delete(m, k)
	}
	return m
}

// Hash normalize snapshot'in sha256 hex'i.
func Hash(d *provider.HotelDetails) string {
	m := normalizeForHash(d)
	b, _ := json.Marshal(m) // map anahtarlari json.Marshal'da siralidir
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Diff iki snapshot arasindaki alan bazli farklari uretir (insan okunur path'lerle).
func Diff(oldD, newD *provider.HotelDetails) []Change {
	var out []Change
	Enrich(oldD)
	Enrich(newD)
	skip := classifiedTitles(oldD)
	for k, v := range classifiedTitles(newD) {
		skip[k] = v
	}
	cmpStr := func(path, a, b string) {
		if a != b {
			switch {
			case a == "":
				out = append(out, Change{Path: path, Kind: "added", New: b})
			case b == "":
				out = append(out, Change{Path: path, Kind: "removed", Old: a})
			default:
				out = append(out, Change{Path: path, Kind: "changed", Old: a, New: b})
			}
		}
	}
	cmpStr("name", oldD.Name, newD.Name)
	cmpStr("description", oldD.Description, newD.Description)
	cmpStr("address", oldD.Address, newD.Address)
	cmpStr("city", oldD.City, newD.City)
	cmpStr("region", oldD.Region, newD.Region)
	cmpStr("stars", fmtIntPtr(oldD.Stars), fmtIntPtr(newD.Stars))
	cmpStr("rating", fmtF64Ptr(oldD.Rating), fmtF64Ptr(newD.Rating))
	cmpStr("ratingCount", fmtIntPtr(oldD.RatingCount), fmtIntPtr(newD.RatingCount))
	if oldD.Contact != nil || newD.Contact != nil {
		oc, nc := orContact(oldD.Contact), orContact(newD.Contact)
		cmpStr("contact.phone", oc.Phone, nc.Phone)
		cmpStr("contact.email", oc.Email, nc.Email)
		cmpStr("contact.website", oc.Website, nc.Website)
	}
	if oldD.Policies != nil || newD.Policies != nil {
		op, np := orPolicies(oldD.Policies), orPolicies(newD.Policies)
		cmpStr("policies.checkIn", op.CheckIn, np.CheckIn)
		cmpStr("policies.checkOut", op.CheckOut, np.CheckOut)
		cmpStr("policies.childPolicy", op.ChildPolicy, np.ChildPolicy)
		cmpStr("policies.petPolicy", op.PetPolicy, np.PetPolicy)
		cmpStr("policies.cancellation", op.Cancellation, np.Cancellation)
		out = append(out, diffSet("policies.other", op.Other, np.Other)...)
	}
	// Gorseller: kume farki (URL)
	oi, ni := []string{}, []string{}
	for _, im := range oldD.Images {
		oi = append(oi, im.URL)
	}
	for _, im := range newD.Images {
		ni = append(ni, im.URL)
	}
	out = append(out, diffSet("images", oi, ni)...)
	out = append(out, diffSet("boards", oldD.Boards, newD.Boards)...)
	// Ozellik gruplari: grup adiyla
	og, ng := map[string][]string{}, map[string][]string{}
	for _, g := range oldD.Facilities {
		og[g.Title] = append(og[g.Title], g.Items...)
	}
	for _, g := range newD.Facilities {
		ng[g.Title] = append(ng[g.Title], g.Items...)
	}
	for _, k := range unionKeys(og, ng) {
		out = append(out, diffSet("facilities."+k, og[k], ng[k])...)
	}
	// Odalar: ada gore
	orm, nrm := map[string]provider.RoomInfo{}, map[string]provider.RoomInfo{}
	for _, r := range oldD.Rooms {
		orm[r.Name] = r
	}
	for _, r := range newD.Rooms {
		nrm[r.Name] = r
	}
	for _, k := range unionKeysRoom(orm, nrm) {
		o, ok1 := orm[k]
		n, ok2 := nrm[k]
		switch {
		case !ok1:
			out = append(out, Change{Path: "rooms." + k, Kind: "added", New: n.Name})
		case !ok2:
			out = append(out, Change{Path: "rooms." + k, Kind: "removed", Old: o.Name})
		default:
			cmpStr("rooms."+k+".size", fmtIntPtr(o.Size), fmtIntPtr(n.Size))
			cmpStr("rooms."+k+".capacity", o.Capacity, n.Capacity)
			cmpStr("rooms."+k+".description", o.Description, n.Description)
			out = append(out, diffSet("rooms."+k+".features", o.Features, n.Features)...)
			out = append(out, diffSet("rooms."+k+".images", o.Images, n.Images)...)
		}
	}
	// Soru-cevap: soruya gore
	oq, nq := map[string]string{}, map[string]string{}
	for _, q := range oldD.FAQ {
		oq[q.Question] = q.Answer
	}
	for _, q := range newD.FAQ {
		nq[q.Question] = q.Answer
	}
	for _, k := range unionKeysStr(oq, nq) {
		o, ok1 := oq[k]
		n, ok2 := nq[k]
		switch {
		case !ok1:
			out = append(out, Change{Path: "faq." + k, Kind: "added", New: n})
		case !ok2:
			out = append(out, Change{Path: "faq." + k, Kind: "removed", Old: o})
		case o != n:
			out = append(out, Change{Path: "faq." + k, Kind: "changed", Old: o, New: n})
		}
	}
	// Yorumlar / etkinlikler: kume farki; puanlar / kampanyalar: anahtara gore
	or_, nr_ := []string{}, []string{}
	for _, x := range oldD.Reviews {
		or_ = append(or_, x.Text)
	}
	for _, x := range newD.Reviews {
		nr_ = append(nr_, x.Text)
	}
	out = append(out, diffSet("reviews", or_, nr_)...)
	oe, ne := []string{}, []string{}
	for _, x := range oldD.Events {
		oe = append(oe, strings.TrimSpace(x.Title+" "+x.Text))
	}
	for _, x := range newD.Events {
		ne = append(ne, strings.TrimSpace(x.Title+" "+x.Text))
	}
	out = append(out, diffSet("events", oe, ne)...)
	osc, nsc := map[string]string{}, map[string]string{}
	for _, x := range oldD.Scores {
		osc[x.Key] = x.Value
	}
	for _, x := range newD.Scores {
		nsc[x.Key] = x.Value
	}
	for _, k := range unionKeysStr(osc, nsc) {
		cmpStr("scores."+k, osc[k], nsc[k])
	}
	oc_, nc_ := map[string]string{}, map[string]string{}
	for _, x := range oldD.Campaigns {
		oc_[x.Title] = x.Text
	}
	for _, x := range newD.Campaigns {
		nc_[x.Title] = x.Text
	}
	for _, k := range unionKeysStr(oc_, nc_) {
		cmpStr("campaigns."+k, oc_[k], nc_[k])
	}
	// Bolumler: basliga gore icerik (siniflandirilanlar yukarida raporlandi)
	os_, ns_ := map[string]string{}, map[string]string{}
	for _, s := range oldD.Sections {
		if !skip[s.Title] {
			os_[s.Title] = sectionText(s)
		}
	}
	for _, s := range newD.Sections {
		if !skip[s.Title] {
			ns_[s.Title] = sectionText(s)
		}
	}
	for _, k := range unionKeysStr(os_, ns_) {
		o, ok1 := os_[k]
		n, ok2 := ns_[k]
		switch {
		case !ok1:
			out = append(out, Change{Path: "sections." + k, Kind: "added", New: trunc(n)})
		case !ok2:
			out = append(out, Change{Path: "sections." + k, Kind: "removed", Old: trunc(o)})
		case o != n:
			out = append(out, Change{Path: "sections." + k, Kind: "changed", Old: trunc(o), New: trunc(n)})
		}
	}
	return out
}

// Summary insan okunur ozet: "3 gorsel eklendi, 1 ozellik kaldirildi, aciklama degisti".
func Summary(ch []Change) string {
	if len(ch) == 0 {
		return "degisiklik yok"
	}
	cnt := map[string]int{}
	for _, c := range ch {
		top := strings.SplitN(c.Path, ".", 2)[0]
		top = strings.SplitN(top, "[", 2)[0]
		cnt[top+":"+c.Kind]++
	}
	labels := map[string]string{"images": "gorsel", "facilities": "ozellik", "rooms": "oda", "faq": "soru-cevap", "sections": "bolum", "policies": "politika", "boards": "pansiyon", "description": "aciklama", "name": "ad", "rating": "puan", "ratingCount": "yorum sayisi", "stars": "yildiz", "address": "adres", "contact": "iletisim", "city": "sehir", "region": "bolge", "reviews": "yorum", "scores": "puan", "events": "etkinlik", "campaigns": "kampanya"}
	kinds := map[string]string{"added": "eklendi", "removed": "kaldirildi", "changed": "degisti"}
	keys := make([]string, 0, len(cnt))
	for k := range cnt {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := []string{}
	for _, k := range keys {
		p := strings.SplitN(k, ":", 2)
		l := labels[p[0]]
		if l == "" {
			l = p[0]
		}
		parts = append(parts, fmt.Sprintf("%d %s %s", cnt[k], l, kinds[p[1]]))
	}
	return strings.Join(parts, ", ")
}

// ---- yardimcilar ----

func diffSet(path string, a, b []string) []Change {
	var out []Change
	as, bs := map[string]bool{}, map[string]bool{}
	for _, x := range a {
		as[x] = true
	}
	for _, x := range b {
		bs[x] = true
	}
	for i, x := range b {
		if !as[x] {
			out = append(out, Change{Path: fmt.Sprintf("%s[%d]", path, i), Kind: "added", New: x})
		}
	}
	for i, x := range a {
		if !bs[x] {
			out = append(out, Change{Path: fmt.Sprintf("%s[%d]", path, i), Kind: "removed", Old: x})
		}
	}
	return out
}

func sectionText(s provider.Section) string {
	switch {
	case s.Text != "":
		return s.Text
	case len(s.Items) > 0:
		return strings.Join(s.Items, "\n")
	case len(s.Pairs) > 0:
		parts := make([]string, 0, len(s.Pairs))
		for _, p := range s.Pairs {
			parts = append(parts, p.Key+": "+p.Value)
		}
		return strings.Join(parts, "\n")
	default:
		return s.HTML
	}
}

// trunc eski/yeni gosterimi icin uzun bolum metnini sinirlar (modalda tam karsilastirma icin 6000 karakter).
func trunc(s string) string {
	if len(s) > 6000 {
		return s[:6000] + "…"
	}
	return s
}

func fmtIntPtr(p *int) string {
	if p == nil {
		return ""
	}
	return fmt.Sprint(*p)
}

func fmtF64Ptr(p *float64) string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%.1f", *p)
}

func orContact(c *provider.Contact) provider.Contact {
	if c == nil {
		return provider.Contact{}
	}
	return *c
}

func orPolicies(p *provider.Policies) provider.Policies {
	if p == nil {
		return provider.Policies{}
	}
	return *p
}

func unionKeys(a, b map[string][]string) []string {
	m := map[string]bool{}
	for k := range a {
		m[k] = true
	}
	for k := range b {
		m[k] = true
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func unionKeysStr(a, b map[string]string) []string {
	m := map[string]bool{}
	for k := range a {
		m[k] = true
	}
	for k := range b {
		m[k] = true
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func unionKeysRoom(a, b map[string]provider.RoomInfo) []string {
	m := map[string]bool{}
	for k := range a {
		m[k] = true
	}
	for k := range b {
		m[k] = true
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
