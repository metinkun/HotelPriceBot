import React, { useEffect, useState } from "react";
import { api, getToken, setToken } from "./api.js";
import { AppShell, AuthLayout, AuthVisual, Button, Field, Input, Badge, Alert } from "@kit";
import Dashboard from "./pages/Dashboard.jsx";
import Hotels from "./pages/Hotels.jsx";
import Search from "./pages/Search.jsx";
import History from "./pages/History.jsx";
import Schedules from "./pages/Schedules.jsx";
import Sites from "./pages/Sites.jsx";
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
  { key: "notifications", label: "Bildirimler", icon: "bell", comp: Notifications, title: "Bildirimler" },
  { key: "hoteldetail", label: "", icon: "building", comp: HotelDetail, title: "Otel detayı", parent: "hotels" },
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
    try { const r = await api("/api/portal/auth/login", { method: "POST", body: { email, password } }); setToken(r.token); onLogin(); }
    catch (ex) { setErr(ex.message); } finally { setBusy(false); }
  };
  return (
    <AuthLayout product="Portal" headline="Otel fiyatlarını tek ekranda karşılaştırın." text="13 acentenin güncel fiyatları yan yana. Anlık arama yapın, haftalık planlar kurun, otel bilgileri değişince haberiniz olsun."
      points={["Anlık ve planlı fiyat karşılaştırması", "Otel + uçak + transfer paketleri", "Acente bazlı otel detayı ve değişiklik bildirimleri"]} visual={<AuthVisual />}>
      <form onSubmit={submit} className="sr-stack">
        <div><h2>Giriş yap</h2><div className="sub">Hesap bilgileriniz size e-posta ile iletilmiştir.</div></div>
        {err && <Alert tone="bad">{err}</Alert>}
        <Field label="E-posta"><Input type="email" icon="mail" value={email} onChange={(e) => setEmail(e.target.value)} autoFocus autoComplete="username" /></Field>
        <Field label="Şifre"><Input type="password" icon="lock" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" /></Field>
        <Button type="submit" size="lg" block loading={busy} variant="gradient">Giriş yap</Button>
      </form>
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
  const quota = me.plan?.dailyQuota ? `${me.usedToday}/${me.plan.dailyQuota}` : `${me.usedToday}`;
  return (
    <AppShell product="Portal" nav={nav} current={current.parent || current.key} onNavigate={(k) => navigate(k)} title={current.title}
      user={{ name: me.client.name, sub: me.user.email }} onLogout={() => { setToken(""); setMe(null); }}
      mobileNavKeys={["dashboard", "search", "hotels", "notifications"]}
      topbarExtra={<span className="hide-mobile"><Badge tone="info" dot={false} title="Plan ve bugünkü kredi kullanımı">Plan: {me.plan?.code || "—"} · Bugün {quota} kredi</Badge></span>}>
      <Page key={route.key + "/" + route.param} me={me} refreshMe={loadMe} param={route.param} extra={route.extra} navigate={navigate} />
    </AppShell>
  );
}
