import React, { useEffect, useState } from "react";
import { api, getToken, setToken } from "./api.js";
import { AppShell, AuthLayout, AuthVisual, Button, Field, Input, IconButton, Alert } from "@kit";
import Dashboard from "./pages/Dashboard.jsx";
import Clients from "./pages/Clients.jsx";
import ClientDetail from "./pages/ClientDetail.jsx";
import Plans from "./pages/Plans.jsx";
import Usage from "./pages/Usage.jsx";
import Logs from "./pages/Logs.jsx";
import Hotels from "./pages/Hotels.jsx";
import Admins from "./pages/Admins.jsx";
import Schedules from "./pages/Schedules.jsx";
import HotelDetail from "./pages/HotelDetail.jsx";
import Notifications from "./pages/Notifications.jsx";
import Import from "./pages/Import.jsx";
import DetailChanges from "./pages/DetailChanges.jsx";
import Approvals from "./pages/Approvals.jsx";
import LoginLogs from "./pages/LoginLogs.jsx";
import Sites from "./pages/Sites.jsx";

// key, menu etiketi, ikon, bilesen, ust cubuk basligi, (menu grubu). Etiketi bos olanlar menude gorunmez.
const PAGES = [
  { key: "dashboard", label: "Özet", icon: "home", comp: Dashboard, title: "Özet", section: "Genel" },
  { key: "clients", label: "Müşteriler", icon: "users", comp: Clients, title: "Müşteriler", section: "Müşteri" },
  { key: "plans", label: "Planlar", icon: "layers", comp: Plans, title: "Planlar" },
  { key: "schedules", label: "Planlı Aramalar", icon: "calendar", comp: Schedules, title: "Planlı aramalar" },
  { key: "approvals", label: "Onay Bekleyenler", icon: "clipboardCheck", comp: Approvals, title: "Onay bekleyen işlemler" },
  { key: "usage", label: "Kullanım", icon: "chart", comp: Usage, title: "Kullanım", section: "Operasyon" },
  { key: "logs", label: "İstek Logları", icon: "list", comp: Logs, title: "İstek logları" },
  { key: "loginlogs", label: "Giriş Logları", icon: "logIn", comp: LoginLogs, title: "Giriş logları" },
  { key: "sites", label: "Site Yönetimi", icon: "globe", comp: Sites, title: "Site yönetimi", section: "Otel" },
  { key: "hotels", label: "Oteller", icon: "building", comp: Hotels, title: "Oteller" },
  { key: "import", label: "Toplu Otel Ekle", icon: "upload", comp: Import, title: "Toplu otel ekle" },
  { key: "changes", label: "Detay Değişiklikleri", icon: "diff", comp: DetailChanges, title: "Otel detay değişiklikleri" },
  { key: "notifications", label: "Bildirimler", icon: "bell", comp: Notifications, title: "Bildirimler", section: "Sistem" },
  { key: "admins", label: "Adminler", icon: "shield", comp: Admins, title: "Adminler" },
  { key: "hoteldetail", label: "", icon: "building", comp: HotelDetail, title: "Otel detayı", parent: "hotels" },
  { key: "clientdetail", label: "", icon: "users", comp: ClientDetail, title: "Müşteri detayı", parent: "clients" },
];

export const navigate = (key, param) => { location.hash = param != null ? `${key}/${param}` : key; };
const parseHash = () => { const parts = (location.hash || "#dashboard").slice(1).split("/"); return { key: parts[0] || "dashboard", param: parts[1] || "", extra: parts[2] || "" }; };

function Login({ onLogin }) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const submit = async (e) => {
    e.preventDefault(); setBusy(true); setErr("");
    try { const r = await api("/api/admin/auth/login", { method: "POST", body: { email, password } }); setToken(r.token); onLogin(r.admin); }
    catch (ex) { setErr(ex.message); } finally { setBusy(false); }
  };
  return (
    <AuthLayout product="Admin" headline="Tüm acenteler, tek yönetim paneli." text="Müşteriler, planlar, oteller ve 13 sitenin sağlığı tek ekranda. Kanarya testi her gün sizin yerinize kontrol eder."
      points={["Müşteri ve API anahtarı yönetimi", "Site sağlığı, devre kesici, kanarya", "Otel detaylarında günlük değişiklik takibi"]} visual={<AuthVisual />}>
      <form onSubmit={submit} className="sr-stack">
        <div><h2>Yönetici girişi</h2><div className="sub">Yalnızca yetkili hesaplar.</div></div>
        {err && <Alert tone="bad">{err}</Alert>}
        <Field label="E-posta"><Input type="email" icon="mail" value={email} onChange={(e) => setEmail(e.target.value)} autoFocus autoComplete="username" /></Field>
        <Field label="Şifre"><Input type="password" icon="lock" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" /></Field>
        <Button type="submit" size="lg" block loading={busy} variant="gradient">Giriş yap</Button>
        <p className="muted small">İlk admin ADMIN_EMAIL / ADMIN_PASSWORD ortam değişkenleriyle oluşturulur.</p>
      </form>
    </AuthLayout>
  );
}

export default function App() {
  const [me, setMe] = useState(null);
  const [checked, setChecked] = useState(false);
  const [route, setRoute] = useState(parseHash);
  const [unread, setUnread] = useState(0);
  const [pending, setPending] = useState(0);

  useEffect(() => {
    const onHash = () => setRoute(parseHash());
    const onLogout = () => setMe(null);
    window.addEventListener("hashchange", onHash); window.addEventListener("pricebot:logout", onLogout);
    return () => { window.removeEventListener("hashchange", onHash); window.removeEventListener("pricebot:logout", onLogout); };
  }, []);
  useEffect(() => { if (!getToken()) { setChecked(true); return; } api("/api/admin/auth/me").then(setMe).catch(() => setMe(null)).finally(() => setChecked(true)); }, []);
  useEffect(() => {
    if (!me) return;
    const f = () => {
      api("/api/admin/notifications?unread=true&limit=1").then((d) => setUnread(d.unread || 0)).catch(() => {});
      api("/api/admin/approvals?status=pending&limit=1").then((d) => setPending(d.pending || 0)).catch(() => {});
    };
    f(); const t = setInterval(f, 60000); return () => clearInterval(t);
  }, [me, route.key]);

  if (!checked) return null;
  if (!me) return <Login onLogin={setMe} />;
  const current = PAGES.find((p) => p.key === route.key) || PAGES[0];
  const Page = current.comp;
  const nav = PAGES.filter((p) => p.label).map((p) => ({ key: p.key, label: p.label, icon: p.icon, section: p.section, badge: p.key === "notifications" ? unread : p.key === "approvals" ? pending : 0 }));
  return (
    <AppShell product="Admin" nav={nav} current={current.parent || current.key} onNavigate={(k) => navigate(k)} title={current.title}
      user={{ name: me.name || "Admin", sub: me.email }} onLogout={() => { setToken(""); setMe(null); }}
      mobileNavKeys={["dashboard", "clients", "hotels", "notifications"]}
      topbarExtra={<><IconButton icon="bell" title="Bildirimler" onClick={() => navigate("notifications")} /><a className="sr-btn secondary sm" href="/docs" target="_blank" rel="noreferrer" style={{ textDecoration: "none" }}>API Dokümanı</a></>}>
      <Page key={route.key + "/" + route.param} param={route.param} extra={route.extra} navigate={navigate} me={me} />
    </AppShell>
  );
}
