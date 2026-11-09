package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"golang.org/x/crypto/bcrypt"

	"pricebot/internal/apperr"
	"pricebot/internal/auth"
	"pricebot/internal/db"
)

// ---- Portal girisi: sifre + cihaz dogrulama (WebAuthn / Face ID / parmak izi) ----
//
// Akis:
//  1. POST /auth/login {email,password}
//     - cihaz dogrulamasi kapali (musteri ayari) -> {step:"done", token}
//     - kullanicinin kayitli cihazi yok        -> {step:"register", setupToken, options}  (bu cihazi kaydet)
//     - kayitli cihaz var                      -> {step:"verify",   setupToken, options}  (biyometrik onay)
//  2. verify basarisiz / bu cihazda anahtar yok -> POST /auth/webauthn/register/begin {setupToken} -> options
//     cihaz siniri doluysa 409 device_limit + cihaz listesi -> POST /auth/devices/replace-request (admin onayi)
//  3. POST /auth/webauthn/register/finish | verify/finish -> token (dev claim = cihaz id)

const portalDeviceKey ctxKey = "portalDevice"

func portalDeviceFrom(ctx context.Context) *db.Device {
	if v, ok := ctx.Value(portalDeviceKey).(*db.Device); ok {
		return v
	}
	return nil
}

type setupSession struct {
	UserID   int
	ClientID int
	Email    string
	Kind     string // pc | mobile (tarayici bilgisinden)
	UA       string
	IP       string
	Session  *webauthn.SessionData
	Exp      time.Time
}

var (
	setupMu  sync.Mutex
	setupMap = map[string]*setupSession{}
)

func putSetup(ss *setupSession) string {
	b := make([]byte, 18)
	_, _ = rand.Read(b)
	tok := "st_" + hex.EncodeToString(b)
	ss.Exp = time.Now().Add(5 * time.Minute)
	setupMu.Lock()
	for k, v := range setupMap {
		if time.Now().After(v.Exp) {
			delete(setupMap, k)
		}
	}
	setupMap[tok] = ss
	setupMu.Unlock()
	return tok
}

func getSetup(tok string) *setupSession {
	setupMu.Lock()
	defer setupMu.Unlock()
	ss, ok := setupMap[tok]
	if !ok || time.Now().After(ss.Exp) {
		return nil
	}
	return ss
}

func dropSetup(tok string) { setupMu.Lock(); delete(setupMap, tok); setupMu.Unlock() }

var reMobileUA = regexp.MustCompile(`(?i)Mobile|Android|iPhone|iPad|iPod|Windows Phone`)

func deviceKindFromUA(ua string) string {
	if reMobileUA.MatchString(ua) {
		return "mobile"
	}
	return "pc"
}

// webauthnFor istegin origin'ine gore RP yapilandirmasi (prod'da WEBAUTHN_RPID / WEBAUTHN_ORIGINS ile sabitlenebilir).
func (s *Server) webauthnFor(r *http.Request) (*webauthn.WebAuthn, error) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		scheme := "http"
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
		origin = scheme + "://" + r.Host
	}
	rpid := s.cfg.WebAuthnRPID
	if rpid == "" {
		if u, err := url.Parse(origin); err == nil {
			rpid = u.Hostname()
		}
	}
	origins := s.cfg.WebAuthnOrigins
	if len(origins) == 0 {
		origins = []string{origin}
	}
	return webauthn.New(&webauthn.Config{
		RPDisplayName: "SmartRate",
		RPID:          rpid,
		RPOrigins:     origins,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			AuthenticatorAttachment: protocol.Platform, // cihazin kendi biyometrisi (Face ID, Touch ID, Windows Hello)
			UserVerification:        protocol.VerificationRequired,
			ResidentKey:             protocol.ResidentKeyRequirementPreferred,
		},
		Timeouts: webauthn.TimeoutsConfig{
			Login:        webauthn.TimeoutConfig{Enforce: true, Timeout: 2 * time.Minute, TimeoutUVD: 2 * time.Minute},
			Registration: webauthn.TimeoutConfig{Enforce: true, Timeout: 2 * time.Minute, TimeoutUVD: 2 * time.Minute},
		},
	})
}

// waUser go-webauthn kullanici arayuzu.
type waUser struct {
	id      []byte
	name    string
	display string
	creds   []webauthn.Credential
}

func (u *waUser) WebAuthnID() []byte                         { return u.id }
func (u *waUser) WebAuthnName() string                       { return u.name }
func (u *waUser) WebAuthnDisplayName() string                { return u.display }
func (u *waUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }

func credentialOf(d db.Device) webauthn.Credential {
	tr := make([]protocol.AuthenticatorTransport, 0, len(d.Transports))
	for _, t := range d.Transports {
		tr = append(tr, protocol.AuthenticatorTransport(t))
	}
	return webauthn.Credential{ID: d.CredentialID, PublicKey: d.PublicKey, Transport: tr, Authenticator: webauthn.Authenticator{AAGUID: d.AAGUID, SignCount: uint32(d.SignCount)}}
}

func (s *Server) loadWAUser(ctx context.Context, u *db.PortalUser) (*waUser, []db.Device, error) {
	id, err := s.db.EnsureWebauthnID(ctx, u.ID)
	if err != nil {
		return nil, nil, err
	}
	devs, err := s.db.ListDevices(ctx, u.ID, false)
	if err != nil {
		return nil, nil, err
	}
	wu := &waUser{id: id, name: u.Email, display: u.Email}
	if u.Name != nil && *u.Name != "" {
		wu.display = *u.Name
	}
	for _, d := range devs {
		wu.creds = append(wu.creds, credentialOf(d))
	}
	return wu, devs, nil
}

func strp(s string) *string { return &s }
func intp(i int) *int       { return &i }

// loginLog giris denemesini kaydeder (asenkron).
func (s *Server) loginLog(r *http.Request, clientID, userID *int, email string, devID *int, kind *string, result, detail string) {
	l := db.LoginLog{ClientID: clientID, UserID: userID, Email: strp(email), IP: strp(clientIP(r)), UserAgent: strp(r.UserAgent()), DeviceID: devID, DeviceKind: kind, Result: result}
	if detail != "" {
		l.Detail = strp(detail)
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.db.InsertLoginLog(ctx, l)
	}()
}

func (s *Server) issuePortalToken(u *db.PortalUser, c *db.Client, devID int) (map[string]any, error) {
	exp := time.Now().Add(24 * time.Hour)
	tok, err := auth.SignToken(s.jwtSecret(), auth.Claims{Sub: fmt.Sprint(u.ID), Email: u.Email, Role: "portal", Exp: exp.Unix(), Iat: time.Now().Unix(), Dev: devID})
	if err != nil {
		return nil, err
	}
	go s.db.TouchPortalLogin(context.Background(), u.ID)
	return map[string]any{"step": "done", "token": tok, "expiresAt": exp, "user": map[string]any{"id": u.ID, "email": u.Email, "name": u.Name}, "client": map[string]any{"id": c.ID, "name": c.Name, "kind": c.Kind}}, nil
}

// deviceLimits kullanicinin cihaz kullanimi / siniri.
func (s *Server) deviceLimits(ctx context.Context, c *db.Client, devs []db.Device) map[string]any {
	var plan *db.Plan
	if c.PlanID != nil {
		plan, _ = s.db.GetPlan(ctx, *c.PlanID)
	}
	lim := db.EffectiveLimits(c, plan)
	pc, mob := 0, 0
	for _, d := range devs {
		if d.Kind == "mobile" {
			mob++
		} else {
			pc++
		}
	}
	return map[string]any{"pc": map[string]int{"used": pc, "max": lim.MaxDevicesPC}, "mobile": map[string]int{"used": mob, "max": lim.MaxDevicesMobile}}
}

func deviceSlotFree(limits map[string]any, kind string) bool {
	m := limits[kind].(map[string]int)
	return m["max"] <= 0 || m["used"] < m["max"]
}

func publicDevices(devs []db.Device) []map[string]any {
	out := []map[string]any{}
	for _, d := range devs {
		out = append(out, map[string]any{"id": d.ID, "name": d.Name, "kind": d.Kind, "createdAt": d.CreatedAt, "lastUsedAt": d.LastUsedAt})
	}
	return out
}

// POST /api/portal/auth/login {email, password}
func (s *Server) portalLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeBody(r, &body); err != nil || body.Email == "" || body.Password == "" {
		badRequest(w, "email ve password zorunlu")
		return
	}
	if !s.usage.RateAllow(r.Context(), -2*int(hash32(clientIP(r))), 10) {
		writeError(w, r, apperr.New(apperr.TooMany, "Cok fazla giris denemesi; 1 dk bekleyin"))
		return
	}
	u, err := s.db.GetPortalUserByEmail(r.Context(), body.Email)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(body.Password)) != nil {
		var cid, uid *int
		if u != nil {
			cid, uid = intp(u.ClientID), intp(u.ID)
		}
		s.loginLog(r, cid, uid, body.Email, nil, nil, "bad_password", "")
		writeJSON(w, 401, map[string]any{"error": "E-posta veya sifre hatali"})
		return
	}
	if !u.IsActive {
		s.loginLog(r, intp(u.ClientID), intp(u.ID), u.Email, nil, nil, "inactive", "kullanici pasif")
		writeJSON(w, 403, map[string]any{"error": "Kullanici pasif"})
		return
	}
	c, err := s.db.GetClient(r.Context(), u.ClientID)
	if err != nil || !c.IsActive {
		s.loginLog(r, intp(u.ClientID), intp(u.ID), u.Email, nil, nil, "inactive", "musteri pasif")
		writeJSON(w, 403, map[string]any{"error": "Hesap pasif"})
		return
	}
	if demoExpired(c) {
		s.loginLog(r, intp(c.ID), intp(u.ID), u.Email, nil, nil, "expired", "deneme suresi doldu")
		writeJSON(w, 403, map[string]any{"error": "Deneme sureniz doldu; devam etmek icin yetkilinizle iletisime gecin", "code": "demo_expired"})
		return
	}
	kind := deviceKindFromUA(r.UserAgent())
	if !c.RequireDeviceAuth {
		out, err := s.issuePortalToken(u, c, 0)
		if err != nil {
			writeError(w, r, err)
			return
		}
		s.loginLog(r, intp(c.ID), intp(u.ID), u.Email, nil, strp(kind), "ok", "cihaz dogrulamasi kapali")
		writeJSON(w, 200, out)
		return
	}
	wa, err := s.webauthnFor(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	wu, devs, err := s.loadWAUser(r.Context(), u)
	if err != nil {
		writeError(w, r, err)
		return
	}
	ss := &setupSession{UserID: u.ID, ClientID: c.ID, Email: u.Email, Kind: kind, UA: r.UserAgent(), IP: clientIP(r)}
	limits := s.deviceLimits(r.Context(), c, devs)
	if len(devs) == 0 {
		if !deviceSlotFree(limits, kind) {
			s.loginLog(r, intp(c.ID), intp(u.ID), u.Email, nil, strp(kind), "device_limit", "")
			writeJSON(w, 409, map[string]any{"error": "Bu tur icin cihaz siniri dolu", "code": "device_limit", "limits": limits})
			return
		}
		opts, sess, err := wa.BeginRegistration(wu)
		if err != nil {
			writeError(w, r, err)
			return
		}
		ss.Session = sess
		tok := putSetup(ss)
		s.loginLog(r, intp(c.ID), intp(u.ID), u.Email, nil, strp(kind), "device_required", "ilk cihaz kaydi")
		writeJSON(w, 200, map[string]any{"step": "register", "setupToken": tok, "options": opts, "deviceKind": kind, "limits": limits, "user": map[string]any{"email": u.Email}})
		return
	}
	opts, sess, err := wa.BeginLogin(wu)
	if err != nil {
		writeError(w, r, err)
		return
	}
	ss.Session = sess
	tok := putSetup(ss)
	writeJSON(w, 200, map[string]any{"step": "verify", "setupToken": tok, "options": opts, "deviceKind": kind, "limits": limits, "devices": publicDevices(devs), "user": map[string]any{"email": u.Email}})
}

func (s *Server) setupFromBody(w http.ResponseWriter, r *http.Request, body map[string]json.RawMessage) (*setupSession, string, *db.PortalUser, *db.Client, bool) {
	var tok string
	_ = json.Unmarshal(body["setupToken"], &tok)
	ss := getSetup(tok)
	if ss == nil {
		writeJSON(w, 401, map[string]any{"error": "Kurulum oturumu gecersiz veya suresi dolmus; yeniden giris yapin", "code": "setup_expired"})
		return nil, "", nil, nil, false
	}
	u, err := s.db.GetPortalUser(r.Context(), ss.UserID)
	if err != nil || !u.IsActive {
		writeJSON(w, 401, map[string]any{"error": "Kullanici bulunamadi"})
		return nil, "", nil, nil, false
	}
	c, err := s.db.GetClient(r.Context(), ss.ClientID)
	if err != nil || !c.IsActive {
		writeJSON(w, 403, map[string]any{"error": "Hesap pasif"})
		return nil, "", nil, nil, false
	}
	return ss, tok, u, c, true
}

// POST /api/portal/auth/webauthn/register/begin {setupToken} — bu cihazda kayitli anahtar yoksa yeni cihaz kaydi baslat
func (s *Server) portalWebAuthnRegisterBegin(w http.ResponseWriter, r *http.Request) {
	var body map[string]json.RawMessage
	if err := decodeBody(r, &body); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	ss, tok, u, c, ok := s.setupFromBody(w, r, body)
	if !ok {
		return
	}
	wu, devs, err := s.loadWAUser(r.Context(), u)
	if err != nil {
		writeError(w, r, err)
		return
	}
	limits := s.deviceLimits(r.Context(), c, devs)
	if !deviceSlotFree(limits, ss.Kind) {
		s.loginLog(r, intp(c.ID), intp(u.ID), u.Email, nil, strp(ss.Kind), "device_limit", "")
		writeJSON(w, 409, map[string]any{"error": "Cihaz siniriniz dolu. Bir cihazi kaldirmak icin yoneticinizden onay isteyebilirsiniz.", "code": "device_limit", "limits": limits, "devices": publicDevices(devs), "setupToken": tok})
		return
	}
	wa, err := s.webauthnFor(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	opts, sess, err := wa.BeginRegistration(wu)
	if err != nil {
		writeError(w, r, err)
		return
	}
	ss.Session = sess
	writeJSON(w, 200, map[string]any{"step": "register", "setupToken": tok, "options": opts, "deviceKind": ss.Kind, "limits": limits})
}

// POST /api/portal/auth/webauthn/register/finish {setupToken, name, credential}
func (s *Server) portalWebAuthnRegisterFinish(w http.ResponseWriter, r *http.Request) {
	var body map[string]json.RawMessage
	if err := decodeBody(r, &body); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	ss, tok, u, c, ok := s.setupFromBody(w, r, body)
	if !ok || ss.Session == nil {
		if ok {
			badRequest(w, "Once register/begin cagrilmali")
		}
		return
	}
	wu, devs, err := s.loadWAUser(r.Context(), u)
	if err != nil {
		writeError(w, r, err)
		return
	}
	limits := s.deviceLimits(r.Context(), c, devs)
	if !deviceSlotFree(limits, ss.Kind) {
		writeJSON(w, 409, map[string]any{"error": "Cihaz siniriniz dolu", "code": "device_limit", "limits": limits, "devices": publicDevices(devs), "setupToken": tok})
		return
	}
	parsed, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(body["credential"]))
	if err != nil {
		s.loginLog(r, intp(c.ID), intp(u.ID), u.Email, nil, strp(ss.Kind), "webauthn_failed", "kayit yaniti cozulemedi")
		writeError(w, r, apperr.BadRequestf("Cihaz kaydi dogrulanamadi: %v", err))
		return
	}
	wa, err := s.webauthnFor(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	cred, err := wa.CreateCredential(wu, *ss.Session, parsed)
	if err != nil {
		s.loginLog(r, intp(c.ID), intp(u.ID), u.Email, nil, strp(ss.Kind), "webauthn_failed", err.Error())
		writeError(w, r, apperr.BadRequestf("Cihaz kaydi dogrulanamadi: %v", err))
		return
	}
	var name string
	_ = json.Unmarshal(body["name"], &name)
	name = strings.TrimSpace(name)
	if name == "" {
		name = defaultDeviceName(ss.UA, ss.Kind)
	}
	tr := make([]string, 0, len(cred.Transport))
	for _, t := range cred.Transport {
		tr = append(tr, string(t))
	}
	dev, err := s.db.CreateDevice(r.Context(), db.Device{UserID: u.ID, ClientID: c.ID, Name: name, Kind: ss.Kind, CredentialID: cred.ID, PublicKey: cred.PublicKey, SignCount: int64(cred.Authenticator.SignCount), AAGUID: cred.Authenticator.AAGUID, Transports: tr, UserAgent: strp(ss.UA)})
	if err != nil {
		writeError(w, r, err)
		return
	}
	dropSetup(tok)
	out, err := s.issuePortalToken(u, c, dev.ID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	out["device"] = dev
	s.loginLog(r, intp(c.ID), intp(u.ID), u.Email, intp(dev.ID), strp(ss.Kind), "ok", "yeni cihaz kaydedildi: "+name)
	writeJSON(w, 200, out)
}

// POST /api/portal/auth/webauthn/verify/finish {setupToken, credential}
func (s *Server) portalWebAuthnVerifyFinish(w http.ResponseWriter, r *http.Request) {
	var body map[string]json.RawMessage
	if err := decodeBody(r, &body); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	ss, tok, u, c, ok := s.setupFromBody(w, r, body)
	if !ok || ss.Session == nil {
		if ok {
			badRequest(w, "Kurulum oturumu eksik")
		}
		return
	}
	wu, _, err := s.loadWAUser(r.Context(), u)
	if err != nil {
		writeError(w, r, err)
		return
	}
	parsed, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(body["credential"]))
	if err != nil {
		s.loginLog(r, intp(c.ID), intp(u.ID), u.Email, nil, strp(ss.Kind), "webauthn_failed", "dogrulama yaniti cozulemedi")
		writeError(w, r, apperr.BadRequestf("Cihaz dogrulamasi basarisiz: %v", err))
		return
	}
	wa, err := s.webauthnFor(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	cred, err := wa.ValidateLogin(wu, *ss.Session, parsed)
	if err != nil {
		s.loginLog(r, intp(c.ID), intp(u.ID), u.Email, nil, strp(ss.Kind), "webauthn_failed", err.Error())
		writeJSON(w, 401, map[string]any{"error": "Cihaz dogrulamasi basarisiz. Bu cihaz kayitli degilse 'Bu cihazi kaydet' secenegini kullanin.", "code": "webauthn_failed", "setupToken": tok})
		return
	}
	dev, err := s.db.GetDeviceByCredential(r.Context(), cred.ID)
	if err != nil || dev.RemovedAt != nil || dev.UserID != u.ID {
		s.loginLog(r, intp(c.ID), intp(u.ID), u.Email, nil, strp(ss.Kind), "removed_device", "")
		writeJSON(w, 401, map[string]any{"error": "Bu cihazin kaydi kaldirilmis. Yeniden kaydetmek icin 'Bu cihazi kaydet' secenegini kullanin.", "code": "device_removed", "setupToken": tok})
		return
	}
	s.db.TouchDevice(r.Context(), dev.ID, int64(cred.Authenticator.SignCount))
	dropSetup(tok)
	out, err := s.issuePortalToken(u, c, dev.ID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	out["device"] = dev
	s.loginLog(r, intp(c.ID), intp(u.ID), u.Email, intp(dev.ID), strp(dev.Kind), "ok", "")
	writeJSON(w, 200, out)
}

// POST /api/portal/auth/devices/replace-request {setupToken, removeDeviceId, newDeviceName} — sinir doluyken admin onayi iste (giris oncesi)
func (s *Server) portalDeviceReplaceRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SetupToken     string `json:"setupToken"`
		RemoveDeviceID int    `json:"removeDeviceId"`
		NewDeviceName  string `json:"newDeviceName"`
	}
	if err := decodeBody(r, &body); err != nil || body.RemoveDeviceID <= 0 {
		badRequest(w, "setupToken ve removeDeviceId zorunlu")
		return
	}
	ss := getSetup(body.SetupToken)
	if ss == nil {
		writeJSON(w, 401, map[string]any{"error": "Kurulum oturumu gecersiz veya suresi dolmus; yeniden giris yapin", "code": "setup_expired"})
		return
	}
	dev, err := s.db.GetDevice(r.Context(), body.RemoveDeviceID)
	if err != nil || dev.UserID != ss.UserID || dev.RemovedAt != nil {
		writeError(w, r, apperr.NotFoundf("Cihaz bulunamadi"))
		return
	}
	pending, _ := s.db.ListApprovals(r.Context(), "pending", nil, intp(ss.UserID), 20)
	for _, a := range pending {
		if a.Type == "device_replace" || a.Type == "device_remove" {
			writeJSON(w, 200, map[string]any{"message": "Bekleyen bir cihaz talebiniz zaten var; yonetici onayi bekleniyor.", "approval": a})
			return
		}
	}
	a, err := s.db.CreateApproval(r.Context(), ss.ClientID, intp(ss.UserID), "device_replace", map[string]any{
		"removeDeviceId": dev.ID, "removeDeviceName": dev.Name, "removeDeviceKind": dev.Kind,
		"newDeviceName": strings.TrimSpace(body.NewDeviceName), "newDeviceKind": ss.Kind, "userAgent": ss.UA, "ip": ss.IP, "email": ss.Email,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	s.loginLog(r, intp(ss.ClientID), intp(ss.UserID), ss.Email, intp(dev.ID), strp(ss.Kind), "device_limit", "degistirme talebi olusturuldu #"+fmt.Sprint(a.ID))
	writeJSON(w, 202, map[string]any{"message": "Talebiniz yoneticinize iletildi. Onaylandiginda bu cihazla yeniden giris yapabilirsiniz.", "approval": a})
}

func defaultDeviceName(ua, kind string) string {
	switch {
	case strings.Contains(ua, "iPhone"):
		return "iPhone"
	case strings.Contains(ua, "iPad"):
		return "iPad"
	case strings.Contains(ua, "Android"):
		return "Android telefon"
	case strings.Contains(ua, "Macintosh"):
		return "Mac"
	case strings.Contains(ua, "Windows"):
		return "Windows bilgisayar"
	case strings.Contains(ua, "Linux"):
		return "Linux bilgisayar"
	}
	if kind == "mobile" {
		return "Mobil cihaz"
	}
	return "Bilgisayar"
}

// ---- Portal (oturum acik): cihazlarim ve onay talepleri ----

// GET /api/portal/devices
func (s *Server) portalDevices(w http.ResponseWriter, r *http.Request) {
	u := portalUserFrom(r.Context())
	id := auth.From(r.Context())
	c, _ := s.db.GetClient(r.Context(), id.ClientID)
	devs, err := s.db.ListDevices(r.Context(), u.ID, false)
	if err != nil {
		writeError(w, r, err)
		return
	}
	pending, _ := s.db.ListApprovals(r.Context(), "pending", nil, intp(u.ID), 20)
	cur := portalDeviceFrom(r.Context())
	curID := 0
	if cur != nil {
		curID = cur.ID
	}
	writeJSON(w, 200, map[string]any{
		"devices": devs, "currentDeviceId": curID, "limits": s.deviceLimits(r.Context(), c, devs),
		"removalsToday": s.db.UserDeviceRemovalsToday(r.Context(), u.ID), "dailyRemovalLimit": 1,
		"requireDeviceAuth": c.RequireDeviceAuth, "pendingApprovals": pending,
	})
}

// DELETE /api/portal/devices/{id} — gunde 1 kaldirma serbest; fazlasi admin onayina duser
func (s *Server) portalRemoveDevice(w http.ResponseWriter, r *http.Request) {
	u := portalUserFrom(r.Context())
	id := auth.From(r.Context())
	did := atoiDefault(r.PathValue("id"), 0)
	dev, err := s.db.GetDevice(r.Context(), did)
	if err != nil || dev.UserID != u.ID || dev.RemovedAt != nil {
		writeError(w, r, apperr.NotFoundf("Cihaz bulunamadi"))
		return
	}
	if s.db.UserDeviceRemovalsToday(r.Context(), u.ID) >= 1 {
		pending, _ := s.db.ListApprovals(r.Context(), "pending", nil, intp(u.ID), 20)
		for _, a := range pending {
			if a.Type == "device_remove" || a.Type == "device_replace" {
				writeJSON(w, 200, map[string]any{"status": "pending", "message": "Bekleyen bir cihaz talebiniz zaten var.", "approval": a})
				return
			}
		}
		a, err := s.db.CreateApproval(r.Context(), id.ClientID, intp(u.ID), "device_remove", map[string]any{"removeDeviceId": dev.ID, "removeDeviceName": dev.Name, "removeDeviceKind": dev.Kind, "email": u.Email})
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, 202, map[string]any{"status": "pending", "message": "Gunluk cihaz degisiklik hakkiniz doldu; talebiniz yonetici onayina gonderildi.", "approval": a})
		return
	}
	if err := s.db.RemoveDevice(r.Context(), dev.ID, "user"); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"status": "removed", "message": "Cihaz kaldirildi.", "deviceId": dev.ID})
}

// GET /api/portal/approvals — kullanicinin talepleri
func (s *Server) portalApprovals(w http.ResponseWriter, r *http.Request) {
	u := portalUserFrom(r.Context())
	list, err := s.db.ListApprovals(r.Context(), "", nil, intp(u.ID), 50)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, list)
}
