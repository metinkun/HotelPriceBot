import React, { useEffect, useRef, useState } from "react";
import { api, getToken, setToken, fmtDate } from "./api.js";
import { AppShell, AuthLayout, AuthVisual, Button, Field, Input, Badge, Alert, Card, Icon } from "@kit";
import * as wa from "./webauthn.js";
import Dashboard from "./pages/Dashboard.jsx";
import Hotels from "./pages/Hotels.jsx";
import Search from "./pages/Search.jsx";
import History from "./pages/History.jsx";
import Schedules from "./pages/Schedules.jsx";
import Sites from "./pages/Sites.jsx";
import Devices from "./pages/Devices.jsx";
import HotelDetail from "./pages/HotelDetail.jsx";
import Notifications from "./pages/Notifications.jsx";

// key, menu etiketi, ikon, bilesen, ust cubuk basligi, (menu grubu). Etiketi bos olanlar menude gorunmez.
const PAGES = [
  { key: "dashboard", label: "Özet", icon: "home", comp: Dashboard, title: "Özet", section: "Genel" },
  { key: "search", label: "Fiyat Ara", icon: "search", comp: Search, title: "Fiyat araması" },
  { key: "hotels", label: "Otellerim", icon: "building", comp: Hotels, title: "Otellerim", section: "Takip" },
  { key: "schedules", label: "Planlı Aramalar", icon: "calendar", comp: Schedules, title: "Planlı aramalar" },
  { key: "history", label: "Geçmiş", icon: "history", comp: History, title: "Arama geçmişi" },
  { key: "sites", label: "Siteler", icon: "globe", comp: Sites, title: "Siteler", section: "Hesap" },
  { key: "devices", label: "Cihazlarım", icon: "smartphone", comp: Devices, title: "Cihazlarım" },
  { key: "notifications", label: "Bildirimler", icon: "bell", comp: Notifications, title: "Bildirimler" },
  { key: "hoteldetail", label: "", icon: "building", comp: HotelDetail, title: "Otel detayı", parent: "hotels" },
];

export const navigate = (key, param) => { location.hash = param != null ? `${key}/${param}` : key; };
const parseHash = () => { const parts = (location.hash || "#dashboard").slice(1).split("/"); return { key: parts[0] || "dashboard", param: parts[1] || "", extra: parts[2] || "" }; };

// Giris: 1) e-posta + sifre  2) cihaz dogrulamasi (kayit veya dogrulama). Sunucu adimlari: done | register | verify.
function Login({ onLogin }) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [stage, setStage] = useState(null); // {step, setupToken, options, deviceKind, limits, devices, user}
  const [deviceName, setDeviceName] = useState("");
  const [replace, setReplace] = useState(null); // {devices, limits, pick, newName, sent}
  const [notice, setNotice] = useState("");
  const autoStarted = useRef(false);
  const supported = wa.supported();

  const finish = (r) => { setToken(r.token); onLogin(); };
  const fail = (ex) => { setErr(ex.data?.error || ex.message || String(ex)); };

  const submit = async (e) => {
    e.preventDefault(); setBusy(true); setErr(""); setNotice("");
    try {
      const r = await api("/api/portal/auth/login", { method: "POST", body: { email, password } });
      if (r.step === "done") return finish(r);
      setStage(r); autoStarted.current = false;
      setDeviceName(r.deviceKind === "mobile" ? "Telefonum" : "Bilgisayarım");
    } catch (ex) {
      if (ex.data?.code === "demo_expired") setErr("Deneme süreniz doldu. Devam etmek için hesap yetkilinizle iletişime geçin.");
      else if (ex.data?.code === "device_limit") { setErr("Cihaz sınırınız dolu; bu cihazı kaydetmek için yöneticinizden onay istemeniz gerekir."); }
      else fail(ex);
    } finally { setBusy(false); }
  };

  // Kayitli cihazla dogrulama
  const verify = async () => {
    if (!stage || !supported) return;
    setBusy(true); setErr("");
    try {
      const credential = await wa.get(stage.options);
      const r = await api("/api/portal/auth/webauthn/verify/finish", { method: "POST", body: { setupToken: stage.setupToken, credential } });
      finish(r);
    } catch (ex) {
      if (ex.data?.code === "setup_expired") { setStage(null); setErr("Doğrulama süresi doldu; lütfen yeniden giriş yapın."); }
      else if (ex.name) setErr(wa.describeError(ex));
      else fail(ex);
    } finally { setBusy(false); }
  };
  useEffect(() => { if (stage?.step === "verify" && !autoStarted.current) { autoStarted.current = true; verify(); } }, [stage]);

  // Bu cihazi kaydet (ilk cihaz veya dogrulama basarisizken yeni cihaz)
  const registerBegin = async () => {
    setBusy(true); setErr("");
    try {
      const r = await api("/api/portal/auth/webauthn/register/begin", { method: "POST", body: { setupToken: stage.setupToken } });
      setStage({ ...stage, ...r, step: "register" }); setReplace(null);
    } catch (ex) {
      if (ex.data?.code === "device_limit") setReplace({ devices: ex.data.devices || [], limits: ex.data.limits, pick: ex.data.devices?.[0]?.id || null, newName: deviceName, sent: false });
      else if (ex.data?.code === "setup_expired") { setStage(null); setErr("Süre doldu; lütfen yeniden giriş yapın."); }
      else fail(ex);
    } finally { setBusy(false); }
  };
  const registerFinish = async () => {
    if (!supported) return;
    setBusy(true); setErr("");
    try {
      const credential = await wa.create(stage.options);
      const r = await api("/api/portal/auth/webauthn/register/finish", { method: "POST", body: { setupToken: stage.setupToken, name: deviceName, credential } });
      finish(r);
    } catch (ex) {
      if (ex.data?.code === "device_limit") setReplace({ devices: ex.data.devices || [], limits: ex.data.limits, pick: ex.data.devices?.[0]?.id || null, newName: deviceName, sent: false });
      else if (ex.data?.code === "setup_expired") { setStage(null); setErr("Süre doldu; lütfen yeniden giriş yapın."); }
      else if (ex.name) setErr(wa.describeError(ex));
      else fail(ex);
    } finally { setBusy(false); }
  };
  const requestReplace = async () => {
    if (!replace?.pick) return;
    setBusy(true); setErr("");
    try {
      const r = await api("/api/portal/auth/devices/replace-request", { method: "POST", body: { setupToken: stage.setupToken, removeDeviceId: replace.pick, newDeviceName: replace.newName || deviceName } });
      setReplace({ ...replace, sent: true, message: r.message });
    } catch (ex) { fail(ex); } finally { setBusy(false); }
  };
  const reset = () => { setStage(null); setReplace(null); setErr(""); setPassword(""); };

  const stepBar = (
    <div className="sr-row center" style={{ gap: 8, fontSize: 12, fontWeight: 600, color: "var(--muted)" }}>
      <span style={{ color: stage ? "var(--ok)" : "var(--primary-text)" }}>1 Şifre</span><Icon name="chevronRight" size={12} /><span style={{ color: stage ? "var(--primary-text)" : undefined }}>2 Cihaz</span>
    </div>
  );

  return (
    <AuthLayout product="Portal" headline="Otel fiyatlarını tek ekranda karşılaştırın." text="13 acentenin güncel fiyatları yan yana. Anlık arama yapın, haftalık planlar kurun, otel bilgileri değişince haberiniz olsun."
      points={["Anlık ve planlı fiyat karşılaştırması", "Otel + uçak + transfer paketleri", "Kayıtlı cihazla güvenli giriş (Face ID / Touch ID)"]} visual={<AuthVisual />}>
      {!stage ? (
        <form onSubmit={submit} className="sr-stack">
          <div><h2>Giriş yap</h2><div className="sub">Hesap bilgileriniz size e-posta ile iletilmiştir.</div></div>
          {stepBar}
          {notice && <Alert tone="info">{notice}</Alert>}
          {err && <Alert tone="bad">{err}</Alert>}
          <Field label="E-posta"><Input type="email" icon="mail" value={email} onChange={(e) => setEmail(e.target.value)} autoFocus autoComplete="username" /></Field>
          <Field label="Şifre"><Input type="password" icon="lock" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" /></Field>
          <Button type="submit" size="lg" block loading={busy} variant="gradient">Giriş yap</Button>
        </form>
      ) : (
        <div className="sr-stack">
          <div><h2>{stage.step === "register" ? "Bu cihazı kaydedin" : "Cihaz doğrulaması"}</h2><div className="sub">{stage.user?.email || email}</div></div>
          {stepBar}
          {!supported && <Alert tone="warn">Bu tarayıcı cihaz doğrulamayı desteklemiyor; güncel Chrome, Safari veya Edge kullanın.</Alert>}
          {err && <Alert tone="bad">{err}</Alert>}
          {replace ? (
            <Card title="Cihaz sınırı dolu" subtitle={`Bilgisayar ${replace.limits?.pc?.used}/${replace.limits?.pc?.max || "∞"} · Mobil ${replace.limits?.mobile?.used}/${replace.limits?.mobile?.max || "∞"}`}>
              {replace.sent ? <Alert tone="ok">{replace.message || "Talebiniz yöneticinize iletildi; onaylanınca bu cihazla tekrar giriş yapın."}</Alert> : (<>
                <p className="muted small" style={{ marginBottom: 10 }}>Yeni cihazı eklemek için mevcut bir cihazın kaldırılması gerekir. Hangi cihaz kaldırılsın? Talebiniz yönetici onayına gider.</p>
                <div className="sr-stack" style={{ gap: 6 }}>
                  {replace.devices.map((d) => (
                    <label key={d.id} className="sr-check" style={{ padding: "8px 10px", border: "1px solid var(--line)", borderRadius: 10, background: replace.pick === d.id ? "var(--primary-soft)" : undefined }}>
                      <input type="radio" name="dev" checked={replace.pick === d.id} onChange={() => setReplace({ ...replace, pick: d.id })} />
                      <span><Icon name={d.kind === "mobile" ? "smartphone" : "monitor"} size={14} /> {d.name} <span className="muted small">· son kullanım {fmtDate(d.lastUsedAt)}</span></span>
                    </label>))}
                </div>
                <Field label="Yeni cihazın adı" style={{ marginTop: 12 }}><Input value={replace.newName} onChange={(e) => setReplace({ ...replace, newName: e.target.value })} /></Field>
                <Button block icon="clipboardCheck" style={{ marginTop: 12 }} loading={busy} disabled={!replace.pick} onClick={requestReplace}>Yöneticiden değişiklik iste</Button>
              </>)}
            </Card>
          ) : stage.step === "register" ? (
            <Card>
              <div className="sr-row center" style={{ gap: 12, marginBottom: 10 }}><span className="sr-stat-ic"><Icon name="fingerprint" size={22} /></span><div><b>Face ID, Touch ID veya Windows Hello</b><div className="muted small">Portala yalnızca kayıtlı cihazlardan girilebilir. Cihazınızın kilit yöntemiyle onaylayın; şifreniz değişmez.</div></div></div>
              {stage.limits && <div className="muted small" style={{ marginBottom: 10 }}>Kullanım: bilgisayar {stage.limits.pc?.used}/{stage.limits.pc?.max || "∞"} · mobil {stage.limits.mobile?.used}/{stage.limits.mobile?.max || "∞"}</div>}
              <Field label="Cihaz adı" hint="Cihazlarım sayfasında bu adla görünür"><Input value={deviceName} onChange={(e) => setDeviceName(e.target.value)} /></Field>
              <Button block size="lg" icon="fingerprint" style={{ marginTop: 12 }} loading={busy} disabled={!supported} onClick={registerFinish}>Cihazı kaydet</Button>
            </Card>
          ) : (
            <Card>
              <div className="sr-row center" style={{ gap: 12, marginBottom: 10 }}><span className="sr-stat-ic"><Icon name="shieldCheck" size={22} /></span><div><b>Kayıtlı cihazınızla onaylayın</b><div className="muted small">Cihazınızın kilit yöntemi (Face ID / Touch ID / Windows Hello) açılacak.</div></div></div>
              {stage.devices?.length > 0 && <div className="sr-chips" style={{ marginBottom: 12 }}>{stage.devices.map((d) => <Badge key={d.id} tone="neutral" dot={false}><Icon name={d.kind === "mobile" ? "smartphone" : "monitor"} size={12} /> {d.name}</Badge>)}</div>}
              <Button block size="lg" icon="shieldCheck" loading={busy} disabled={!supported} onClick={verify}>Doğrula</Button>
              <Button block variant="ghost" icon="plus" style={{ marginTop: 8 }} disabled={busy || !supported} onClick={registerBegin}>Bu cihazı kaydet</Button>
            </Card>
          )}
          <Button variant="ghost" icon="chevronLeft" onClick={reset}>Farklı hesapla giriş</Button>
        </div>
      )}
    </AuthLayout>
  );
}

export default function App() {
  const [me, setMe] = useState(null);
  const [checked, setChecked] = useState(false);
  const [route, setRoute] = useState(parseHash);
  const [unread, setUnread] = useState(0);

  const loadMe = () => api("/api/portal/me").then(setMe).catch(() => setMe(null)).finally(() => setChecked(true));
  useEffect(() => {
    const onHash = () => setRoute(parseHash());
    const onLogout = () => setMe(null);
    window.addEventListener("hashchange", onHash); window.addEventListener("pricebot:logout", onLogout);
    return () => { window.removeEventListener("hashchange", onHash); window.removeEventListener("pricebot:logout", onLogout); };
  }, []);
  useEffect(() => { if (!getToken()) { setChecked(true); return; } loadMe(); }, []);
  useEffect(() => { if (!me) return; const f = () => api("/api/portal/notifications?unread=true&limit=1").then((d) => setUnread(d.unread || 0)).catch(() => {}); f(); const t = setInterval(f, 60000); return () => clearInterval(t); }, [me, route.key]);

  if (!checked) return null;
  if (!me) return <Login onLogin={loadMe} />;
  const current = PAGES.find((p) => p.key === route.key) || PAGES[0];
  const Page = current.comp;
  const nav = PAGES.filter((p) => p.label).map((p) => ({ key: p.key, label: p.label, icon: p.icon, section: p.section, badge: p.key === "notifications" ? unread : 0 }));
  const lim = me.limits?.dailySearches || 0;
  const used = me.usage?.searches ?? me.usedToday ?? 0;
  const quotaTone = lim > 0 && used >= lim ? "bad" : lim > 0 && used >= lim * 0.8 ? "warn" : "info";
  return (
    <AppShell product="Portal" nav={nav} current={current.parent || current.key} onNavigate={(k) => navigate(k)} title={current.title}
      user={{ name: me.client.name, sub: me.user.email }} onLogout={() => { setToken(""); setMe(null); }}
      mobileNavKeys={["dashboard", "search", "hotels", "notifications"]}
      topbarExtra={<span className="hide-mobile"><Badge tone={quotaTone} dot={false} title={`Plan: ${me.plan?.name || me.plan?.code || "—"} · bugünkü arama hakkı`}>Bugün {used}/{lim ? lim : "∞"} arama</Badge></span>}>
      <Page key={route.key + "/" + route.param} me={me} refreshMe={loadMe} param={route.param} extra={route.extra} navigate={navigate} />
    </AppShell>
  );
}
