package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"pricebot/internal/apperr"
	"pricebot/internal/auth"
	"pricebot/internal/db"
	"pricebot/internal/details"
	"pricebot/internal/portal"
	"pricebot/internal/provider"
)

// ---- Otel ozet ekrani (bizim detay): kunye, siteler, puanlar, yorumlar, etkinlikler, kampanyalar, degisiklikler ----

type overviewProvider struct {
	Provider        string     `json:"provider"`
	Label           string     `json:"label"`
	ProviderHotelID string     `json:"providerHotelId"`
	SourceURL       *string    `json:"sourceUrl"`
	IsActive        bool       `json:"isActive"`
	Supported       bool       `json:"supported"`
	Fetched         bool       `json:"fetched"`
	Version         int        `json:"version,omitempty"`
	FetchedAt       *time.Time `json:"fetchedAt,omitempty"`
	ChangedAt       *time.Time `json:"changedAt,omitempty"`
	Error           *string    `json:"error,omitempty"`
	UnseenChange    bool       `json:"unseenChange"`
	Rating          *float64   `json:"rating,omitempty"`
	RatingCount     *int       `json:"ratingCount,omitempty"`
	Stars           *int       `json:"stars,omitempty"`
	City            string     `json:"city,omitempty"`
	Reviews         int        `json:"reviews"`
	Events          int        `json:"events"`
	Campaigns       int        `json:"campaigns"`
	Images          int        `json:"images"`
	Rooms           int        `json:"rooms"`
}

func (s *Server) providerLabels() map[string]string {
	out := map[string]string{}
	for _, p := range s.portalSvc.ProviderList(nil) {
		out[p.Name] = p.Label
	}
	return out
}

// hotelOverview admin ve portal icin ortak; allowed nil ise tum siteler.
func (s *Server) hotelOverview(w http.ResponseWriter, r *http.Request, hotelID string, allowed map[string]bool, admin bool) {
	hv, err := s.hotels.Get(r.Context(), hotelID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	meta, _ := s.db.GetHotelMeta(r.Context(), hotelID)
	labels := s.providerLabels()
	snaps, _ := s.db.ListDetailSnapshots(r.Context(), hotelID)
	bySnap := map[string]*provider.HotelDetails{}
	byRow := map[string]db.HotelDetailRow{}
	for _, row := range snaps {
		byRow[row.Provider] = row
		var d provider.HotelDetails
		if json.Unmarshal(row.Snapshot, &d) == nil {
			details.Enrich(&d)
			bySnap[row.Provider] = &d
		}
	}
	// hata kaydi olup snapshot'i olmayanlar
	if rows, err := s.db.ListDetailsForHotel(r.Context(), hotelID); err == nil {
		for _, x := range rows {
			if _, ok := byRow[x.Provider]; !ok {
				byRow[x.Provider] = x
			}
		}
	}
	kind, vid := viewerOf(r)
	provs := []overviewProvider{}
	type scoreRow struct {
		Provider    string        `json:"provider"`
		Label       string        `json:"label"`
		Rating      *float64      `json:"rating,omitempty"`
		RatingCount *int          `json:"ratingCount,omitempty"`
		Stars       *int          `json:"stars,omitempty"`
		Categories  []provider.KV `json:"categories"`
	}
	scores := []scoreRow{}
	reviews := []map[string]any{}
	events := []map[string]any{}
	camps := []map[string]any{}
	for _, p := range hv.Providers {
		if allowed != nil && !allowed[p.Provider] {
			continue
		}
		op := overviewProvider{Provider: p.Provider, Label: labels[p.Provider], ProviderHotelID: p.ProviderHotelID, SourceURL: p.SourceURL, IsActive: p.IsActive, Supported: s.detailsSvc.Supports(p.Provider)}
		if op.Label == "" {
			op.Label = p.Provider
		}
		if row, ok := byRow[p.Provider]; ok {
			op.Error = row.Error
			if row.Version > 0 {
				op.Fetched, op.Version, op.ChangedAt = true, row.Version, row.ChangedAt
				fa := row.FetchedAt
				op.FetchedAt = &fa
				seen := s.db.SeenVersion(r.Context(), kind, vid, hotelID, p.Provider)
				op.UnseenChange = seen > 0 && seen < row.Version
			}
		}
		if d, ok := bySnap[p.Provider]; ok {
			op.Rating, op.RatingCount, op.Stars, op.City = d.Rating, d.RatingCount, d.Stars, d.City
			op.Reviews, op.Events, op.Campaigns, op.Images, op.Rooms = len(d.Reviews), len(d.Events), len(d.Campaigns), len(d.Images), len(d.Rooms)
			if d.Rating != nil || len(d.Scores) > 0 {
				cats := d.Scores
				if cats == nil {
					cats = []provider.KV{}
				}
				scores = append(scores, scoreRow{Provider: p.Provider, Label: op.Label, Rating: d.Rating, RatingCount: d.RatingCount, Stars: d.Stars, Categories: cats})
			}
			for _, x := range d.Reviews {
				reviews = append(reviews, map[string]any{"provider": p.Provider, "label": op.Label, "author": x.Author, "date": x.Date, "title": x.Title, "text": x.Text, "score": x.Score})
			}
			for _, x := range d.Events {
				events = append(events, map[string]any{"provider": p.Provider, "label": op.Label, "title": x.Title, "date": x.Date, "text": x.Text})
			}
			for _, x := range d.Campaigns {
				camps = append(camps, map[string]any{"provider": p.Provider, "label": op.Label, "title": x.Title, "text": x.Text})
			}
		}
		provs = append(provs, op)
	}
	changes, _ := s.db.ListDetailChanges(r.Context(), hotelID, nil, 60)
	if allowed != nil {
		f := changes[:0]
		for _, c := range changes {
			if allowed[c.Provider] {
				f = append(f, c)
			}
		}
		changes = f
	}
	out := map[string]any{
		"hotel": map[string]any{"internalHotelId": hv.InternalHotelID, "hotelName": hv.HotelName, "meta": meta},
		"providers": provs, "scores": scores, "reviews": reviews, "events": events, "campaigns": camps, "changes": changes,
		"labels": labels,
	}
	if admin {
		clients, _ := s.db.ClientsOfHotel(r.Context(), hotelID)
		out["clients"] = clients
	}
	writeJSON(w, 200, out)
}

func (s *Server) adminHotelOverview(w http.ResponseWriter, r *http.Request) {
	s.hotelOverview(w, r, r.PathValue("id"), nil, true)
}

func (s *Server) portalHotelOverview(w http.ResponseWriter, r *http.Request) {
	id := auth.From(r.Context())
	hid := r.PathValue("id")
	if ok, _ := s.db.ClientOwnsHotel(r.Context(), id.ClientID, hid); !ok {
		writeError(w, r, apperr.NotFoundf("Otel bulunamadi: %s", hid))
		return
	}
	pl := planFrom(r.Context())
	eff, _ := s.portalSvc.EffectiveProviders(r.Context(), id.ClientID, pl.AllowedProviders)
	allowed := map[string]bool{}
	for _, n := range eff {
		allowed[n] = true
	}
	s.hotelOverview(w, r, hid, allowed, false)
}

// ---- Admin: site detayi (ayarlar + saglik + bu sitedeki oteller + kullanan musteriler) ----

// GET /api/admin/sites/{name}
func (s *Server) adminSiteDetail(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := s.reg.Get(name); err != nil {
		writeError(w, r, apperr.NotFoundf("Site bulunamadi: %s", name))
		return
	}
	var site map[string]any
	for _, x := range s.siteRows(r.Context()) {
		if x["name"] == name {
			site = x
		}
	}
	ms, err := s.db.ListMappings(r.Context(), db.MappingFilter{Provider: name, OrderBy: "created_at DESC"})
	if err != nil {
		writeError(w, r, err)
		return
	}
	metas, _ := s.db.ListHotelMeta(r.Context())
	dets, _ := s.db.ListDetailsForProvider(r.Context(), name)
	hotels := make([]map[string]any, 0, len(ms))
	for _, m := range ms {
		item := map[string]any{"internalHotelId": m.InternalHotelID, "providerHotelId": m.ProviderHotelID, "hotelName": m.HotelName, "sourceUrl": m.MetaString("sourceUrl"), "isActive": m.IsActive, "createdAt": m.CreatedAt}
		if meta, ok := metas[m.InternalHotelID]; ok {
			item["meta"] = meta
			if meta.Name != nil && *meta.Name != "" {
				item["hotelName"] = meta.Name
			}
		}
		if d, ok := dets[m.InternalHotelID]; ok {
			item["detail"] = map[string]any{"version": d.Version, "fetchedAt": d.FetchedAt, "changedAt": d.ChangedAt, "error": d.Error}
		}
		hotels = append(hotels, item)
	}
	clients, _ := s.db.ClientsUsingProviderList(r.Context(), name)
	changes, _ := s.db.ListDetailChanges(r.Context(), "", nil, 500)
	recent := []db.DetailChange{}
	for _, c := range changes {
		if c.Provider == name {
			recent = append(recent, c)
			if len(recent) >= 30 {
				break
			}
		}
	}
	writeJSON(w, 200, map[string]any{"site": site, "hotels": hotels, "clients": clients, "changes": recent})
}

// GET /api/admin/proxies/status — IP rotasyonu durumu
func (s *Server) adminProxyStatus(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{"browserProxy": s.cfg.BrowserProxyURL != "", "staticProxy": s.cfg.ProxyHost != "", "webshare": s.cfg.WebshareAPIKey != ""}
	if s.deps.Proxy != nil {
		for k, v := range s.deps.Proxy.Status() {
			out[k] = v
		}
	}
	writeJSON(w, 200, out)
}

// ---- Admin: arama detayi ve planli arama gecmisi ----

// GET /api/admin/searches/{id}
func (s *Server) adminSearchDetail(w http.ResponseWriter, r *http.Request) {
	id := int64(atoiDefault(r.PathValue("id"), 0))
	sr, err := s.db.GetSearchByID(r.Context(), id)
	if err != nil {
		notFoundOr(w, r, err, "Arama bulunamadi")
		return
	}
	c, _ := s.db.GetClient(r.Context(), sr.ClientID)
	var user *db.PortalUser
	if sr.UserID != nil {
		user, _ = s.db.GetPortalUser(r.Context(), *sr.UserID)
	}
	var sched *db.Schedule
	if sr.ScheduleID != nil {
		sched, _ = s.db.GetScheduleByID(r.Context(), *sr.ScheduleID)
	}
	writeJSON(w, 200, map[string]any{"search": sr, "client": c, "user": user, "schedule": sched, "labels": s.providerLabels()})
}

// GET /api/admin/schedules/{id}/searches
func (s *Server) adminScheduleSearches(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	sc, err := s.db.GetScheduleByID(r.Context(), id)
	if err != nil {
		notFoundOr(w, r, err, "Planli arama bulunamadi")
		return
	}
	list, err := s.db.ListSearches(r.Context(), sc.ClientID, &id, atoiDefault(r.URL.Query().Get("limit"), 100))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"schedule": sc, "items": list})
}

// POST /api/portal/schedules/{id}/active {isActive}
func (s *Server) portalSetScheduleActive(w http.ResponseWriter, r *http.Request) {
	sid, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	var body struct {
		IsActive bool `json:"isActive"`
	}
	if err := decodeBody(r, &body); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	id := auth.From(r.Context())
	sc, err := s.db.GetSchedule(r.Context(), id.ClientID, sid)
	if err != nil {
		notFoundOr(w, r, err, "Planli arama bulunamadi")
		return
	}
	var next *time.Time
	if body.IsActive {
		n, err := nextRunFrom(sc.Weekdays, sc.RunTime, sc.Timezone, sc.StartsOn)
		if err == nil {
			next = &n
		}
	}
	if err := s.db.SetScheduleActive(r.Context(), id.ClientID, sid, body.IsActive, next); err != nil {
		writeError(w, r, err)
		return
	}
	sc, _ = s.db.GetSchedule(r.Context(), id.ClientID, sid)
	writeJSON(w, 200, sc)
}

// ---- Sifre sifirlama linki ----

func resetTokenHash(tok string) string {
	h := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(h[:])
}

func (s *Server) portalBaseURL(r *http.Request) string {
	if u := strings.TrimSpace(s.cfg.PublicPortalURL); u != "" {
		return strings.TrimRight(u, "/") + "/"
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return scheme + "://" + host + "/portal/"
}

// POST /api/admin/portal-users/{id}/reset-link — link uretir, SMTP ayarliysa e-posta ile gonderir; link admin'e de doner.
func (s *Server) adminUserResetLink(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	u, err := s.db.GetPortalUser(r.Context(), id)
	if err != nil {
		notFoundOr(w, r, err, "Kullanici bulunamadi")
		return
	}
	if !u.IsActive {
		writeError(w, r, apperr.BadRequestf("Kullanici pasif; once aktiflestirin"))
		return
	}
	var body struct {
		Send *bool `json:"send"`
	}
	_ = decodeBody(r, &body)
	raw := make([]byte, 24)
	_, _ = rand.Read(raw)
	tok := hex.EncodeToString(raw)
	ttl := s.cfg.PasswordResetTTL
	if ttl <= 0 {
		ttl = time.Hour
	}
	who := "admin"
	if a := auth.From(r.Context()); a != nil && a.ClientName != "" {
		who = a.ClientName
	}
	pr, err := s.db.CreatePasswordReset(r.Context(), u.ID, resetTokenHash(tok), time.Now().Add(ttl), who)
	if err != nil {
		writeError(w, r, err)
		return
	}
	link := s.portalBaseURL(r) + "#reset/" + tok
	out := map[string]any{"link": link, "expiresAt": pr.ExpiresAt, "email": u.Email, "emailConfigured": s.mailer != nil && s.mailer.Configured(), "emailSent": false}
	if (body.Send == nil || *body.Send) && s.mailer != nil && s.mailer.Configured() {
		name := u.Email
		if u.Name != nil && *u.Name != "" {
			name = *u.Name
		}
		mins := int(ttl.Minutes())
		text := fmt.Sprintf("Merhaba %s,\n\nSmartRate portal sifrenizi yenilemek icin asagidaki baglantiyi kullanin (%d dakika gecerli):\n%s\n\nBu istegi siz yapmadiysaniz bu e-postayi dikkate almayin.", name, mins, link)
		html := fmt.Sprintf(`<p>Merhaba %s,</p><p>SmartRate portal şifrenizi yenilemek için aşağıdaki bağlantıyı kullanın (<b>%d dakika</b> geçerli):</p><p><a href="%s" style="background:#3B82FF;color:#fff;padding:10px 18px;border-radius:8px;text-decoration:none;display:inline-block">Şifremi yenile</a></p><p style="color:#64748B;font-size:12px">Bağlantı: %s<br>Bu isteği siz yapmadıysanız bu e-postayı dikkate almayın.</p>`, name, mins, link, link)
		if err := s.mailer.Send(u.Email, "SmartRate — şifre yenileme bağlantısı", text, html); err != nil {
			msg := err.Error()
			s.db.MarkPasswordResetSent(r.Context(), pr.ID, u.Email, &msg)
			out["emailError"] = msg
		} else {
			s.db.MarkPasswordResetSent(r.Context(), pr.ID, u.Email, nil)
			out["emailSent"] = true
		}
	}
	writeJSON(w, 201, out)
}

func (s *Server) loadReset(ctx context.Context, tok string) (*db.PasswordReset, *db.PortalUser, error) {
	if len(tok) < 20 {
		return nil, nil, apperr.BadRequestf("Gecersiz baglanti")
	}
	pr, err := s.db.GetPasswordReset(ctx, resetTokenHash(tok))
	if err != nil {
		return nil, nil, apperr.NotFoundf("Baglanti gecersiz veya kullanilmis")
	}
	if pr.UsedAt != nil {
		return nil, nil, apperr.New(apperr.Forbidden, "Bu baglanti daha once kullanilmis; yeni bir baglanti isteyin")
	}
	if time.Now().After(pr.ExpiresAt) {
		return nil, nil, apperr.New(apperr.Forbidden, "Baglantinin suresi dolmus; yeni bir baglanti isteyin")
	}
	u, err := s.db.GetPortalUser(ctx, pr.UserID)
	if err != nil || !u.IsActive {
		return nil, nil, apperr.New(apperr.Forbidden, "Kullanici bulunamadi veya pasif")
	}
	return pr, u, nil
}

// POST /api/portal/auth/reset/check {token}
func (s *Server) portalResetCheck(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	_ = decodeBody(r, &body)
	pr, u, err := s.loadReset(r.Context(), strings.TrimSpace(body.Token))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"email": u.Email, "name": u.Name, "expiresAt": pr.ExpiresAt})
}

// POST /api/portal/auth/reset/complete {token, password}
func (s *Server) portalResetComplete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := decodeBody(r, &body); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	if len(body.Password) < 8 {
		badRequest(w, "Sifre en az 8 karakter olmali")
		return
	}
	if !s.usage.RateAllow(r.Context(), -3*int(hash32(clientIP(r))), 10) {
		writeError(w, r, apperr.New(apperr.TooMany, "Cok fazla deneme; 1 dk bekleyin"))
		return
	}
	pr, u, err := s.loadReset(r.Context(), strings.TrimSpace(body.Token))
	if err != nil {
		writeError(w, r, err)
		return
	}
	h, _ := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	hs := string(h)
	if err := s.db.UpdatePortalUser(r.Context(), u.ID, &hs, nil, nil); err != nil {
		writeError(w, r, err)
		return
	}
	_ = s.db.UsePasswordReset(r.Context(), pr.ID)
	s.loginLog(r, intp(u.ClientID), intp(u.ID), u.Email, nil, nil, "password_reset", "sifre baglanti ile yenilendi")
	writeJSON(w, 200, map[string]any{"ok": true, "email": u.Email, "message": "Sifreniz guncellendi; yeni sifrenizle giris yapabilirsiniz."})
}

// nextRunFrom baslangic tarihi varsa oradan, yoksa simdiden itibaren ilk calisma.
func nextRunFrom(weekdays []int16, runTime, tz string, startsOn *string) (time.Time, error) {
	after := time.Now()
	if startsOn != nil && *startsOn != "" {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			loc = time.FixedZone("TRT", 3*3600)
		}
		if d, err := time.ParseInLocation("2006-01-02", *startsOn, loc); err == nil && d.After(after) {
			after = d.Add(-time.Minute)
		}
	}
	return portal.NextRun(weekdays, runTime, tz, after)
}

var _ = sort.Strings
