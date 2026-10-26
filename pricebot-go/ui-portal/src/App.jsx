import React, { useEffect, useState } from "react";
import { api, getToken, setToken } from "./api.js";
import { Logo, LoginArt, HeroArt, Icon } from "./Art.jsx";
import Dashboard from "./pages/Dashboard.jsx";
import Hotels from "./pages/Hotels.jsx";
import Search from "./pages/Search.jsx";
import History from "./pages/History.jsx";
import Schedules from "./pages/Schedules.jsx";
import Sites from "./pages/Sites.jsx";
import HotelDetail from "./pages/HotelDetail.jsx";
import Notifications from "./pages/Notifications.jsx";

const PAGES = [
  ["dashboard", "Özet", "home", Dashboard, "Hoş geldiniz", "Otellerinizin güncel fiyatlarını tek ekrandan takip edin."],
  ["search", "Fiyat Ara", "search", Search, "Fiyat araması", "Seçtiğiniz oteller için tüm etkin sitelerden anlık fiyat karşılaştırması."],
  ["hotels", "Otellerim", "hotel", Hotels, "Otellerim", "Takip etmek istediğiniz otelleri site linkleriyle ekleyin."],
  ["schedules", "Planlı Aramalar", "clock", Schedules, "Planlı aramalar", "Her hafta belirlediğiniz gün ve saatte otomatik fiyat araması."],
  ["history", "Geçmiş", "history", History, "Arama geçmişi", "Anlık ve planlı aramaların sonuçları."],
  ["sites", "Siteler", "sites", Sites, "Siteler", "Hesabınız için etkin olan fiyat kaynakları."],
  ["notifications", "Bildirimler", "history", Notifications, "Bildirimler", "Acentelerdeki otel bilgisi değişiklikleri."],
  ["hoteldetail", "", "hotel", HotelDetailPage, "Otel detayı", "Otelin her acentedeki statik bilgileri: görseller, özellikler, odalar, politikalar, soru-cevap."],
];

function HotelDetailPage({ me }) {
  const parts = (location.hash || "").slice(1).split("/");
  return <HotelDetail me={me} hotelId={parts[1]} key={parts[1]} />;
}

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
    <div className="login-wrap">
      <div className="login-art">
        <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 18 }}><div style={{ width: 40 }}><Logo /></div><b style={{ fontSize: 18 }}>PriceBot Portal</b></div>
        <h1>Otel fiyatlarını tek ekranda karşılaştırın</h1>
        <p>Etstur, Jolly, Setur, Tatilbudur ve diğer sitelerden anlık ve planlı fiyat takibi. Haftalık aramalar kurun, sonuçlar hazır olduğunda görün.</p>
        <LoginArt />
      </div>
      <div className="login-form">
        <form onSubmit={submit}>
          <h2>Giriş yap</h2>
          <div className="sub">Hesap bilgileriniz size e-posta ile iletilmiştir.</div>
          <label>E-posta</label>
          <input value={email} onChange={(e) => setEmail(e.target.value)} autoFocus />
          <label style={{ marginTop: 12 }}>Şifre</label>
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
          {err && <div className="err">{err}</div>}
          <button className="btn" style={{ marginTop: 18, width: "100%" }} disabled={busy}>{busy ? "Giriş yapılıyor…" : "Giriş"}</button>
        </form>
      </div>
    </div>
  );
}

export default function App() {
  const [me, setMe] = useState(null);
  const [checked, setChecked] = useState(false);
  const [page, setPage] = useState(() => (location.hash || "#dashboard").slice(1).split("/")[0]);
  const [unread, setUnread] = useState(0);
  useEffect(() => { if (!me) return; const f = () => api("/api/portal/notifications?unread=true&limit=1").then((d) => setUnread(d.unread)).catch(() => {}); f(); const t = setInterval(f, 60000); return () => clearInterval(t); }, [me, page]);

  const loadMe = () => api("/api/portal/me").then(setMe).catch(() => setMe(null)).finally(() => setChecked(true));
  useEffect(() => {
    const onHash = () => setPage((location.hash || "#dashboard").slice(1).split("/")[0]);
    const onLogout = () => setMe(null);
    window.addEventListener("hashchange", onHash); window.addEventListener("pricebot:logout", onLogout);
    return () => { window.removeEventListener("hashchange", onHash); window.removeEventListener("pricebot:logout", onLogout); };
  }, []);
  useEffect(() => { if (!getToken()) { setChecked(true); return; } loadMe(); }, []);

  if (!checked) return null;
  if (!me) return <Login onLogin={loadMe} />;
  const current = PAGES.find((p) => p[0] === page) || PAGES[0];
  const Page = current[3];
  return (
    <div className="shell">
      <aside className="nav">
        <div className="logo"><Logo /> PriceBot</div>
        {PAGES.filter((p) => p[1]).map(([key, label, icon]) => <button key={key} className={key === current[0] ? "active" : ""} onClick={() => { location.hash = key; }}><Icon name={icon} /> {label}{key === "notifications" && unread > 0 && <span className="badge warn" style={{ marginLeft: "auto" }}>{unread}</span>}</button>)}
        <div className="spacer" />
        <div className="user"><b>{me.client.name}</b>{me.user.email}<div style={{ marginTop: 8 }}>Plan: {me.plan.code} · Bugün {me.usedToday}{me.plan.dailyQuota ? "/" + me.plan.dailyQuota : ""} kredi</div>
          <button onClick={() => { setToken(""); setMe(null); }} style={{ marginTop: 8, padding: "6px 10px" }}>Çıkış</button></div>
      </aside>
      <main className="main">
        <div className="hero"><h1>{current[4]}</h1><p>{current[5]}</p><HeroArt /></div>
        <div className="content"><Page me={me} refreshMe={loadMe} /></div>
      </main>
    </div>
  );
}
