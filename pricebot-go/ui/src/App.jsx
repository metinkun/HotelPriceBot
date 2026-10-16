import React, { useEffect, useState } from "react";
import { api, getToken, setToken } from "./api.js";
import Dashboard from "./pages/Dashboard.jsx";
import Clients from "./pages/Clients.jsx";
import Plans from "./pages/Plans.jsx";
import Usage from "./pages/Usage.jsx";
import Logs from "./pages/Logs.jsx";
import Hotels from "./pages/Hotels.jsx";
import Admins from "./pages/Admins.jsx";

const PAGES = [
  ["dashboard", "Özet", Dashboard],
  ["clients", "Müşteriler", Clients],
  ["plans", "Planlar", Plans],
  ["usage", "Kullanım", Usage],
  ["logs", "İstek Logları", Logs],
  ["hotels", "Oteller", Hotels],
  ["admins", "Adminler", Admins],
];

function Login({ onLogin }) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const submit = async (e) => {
    e.preventDefault(); setBusy(true); setErr("");
    try {
      const r = await api("/api/admin/auth/login", { method: "POST", body: { email, password } });
      setToken(r.token); onLogin(r.admin);
    } catch (ex) { setErr(ex.message); } finally { setBusy(false); }
  };
  return (
    <div className="login card">
      <h2>PriceBot Admin</h2>
      <form onSubmit={submit}>
        <label>E-posta</label>
        <input value={email} onChange={(e) => setEmail(e.target.value)} autoFocus />
        <label style={{ marginTop: 10 }}>Şifre</label>
        <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
        {err && <div className="err">{err}</div>}
        <button className="btn" style={{ marginTop: 14, width: "100%" }} disabled={busy}>Giriş</button>
      </form>
      <p className="muted" style={{ marginTop: 12 }}>İlk admin ADMIN_EMAIL / ADMIN_PASSWORD ortam değişkenleriyle oluşturulur.</p>
    </div>
  );
}

export default function App() {
  const [me, setMe] = useState(null);
  const [checked, setChecked] = useState(false);
  const [page, setPage] = useState(() => (location.hash || "#dashboard").slice(1));

  useEffect(() => {
    const onHash = () => setPage((location.hash || "#dashboard").slice(1));
    const onLogout = () => setMe(null);
    window.addEventListener("hashchange", onHash);
    window.addEventListener("pricebot:logout", onLogout);
    return () => { window.removeEventListener("hashchange", onHash); window.removeEventListener("pricebot:logout", onLogout); };
  }, []);

  useEffect(() => {
    if (!getToken()) { setChecked(true); return; }
    api("/api/admin/auth/me").then((r) => setMe(r)).catch(() => setMe(null)).finally(() => setChecked(true));
  }, []);

  if (!checked) return null;
  if (!me) return <Login onLogin={(a) => setMe(a)} />;

  const current = PAGES.find((p) => p[0] === page) || PAGES[0];
  const Page = current[2];
  return (
    <div className="app">
      <aside className="side">
        <h1>PriceBot Admin</h1>
        {PAGES.map(([key, label]) => (
          <button key={key} className={key === current[0] ? "active" : ""} onClick={() => { location.hash = key; }}>{label}</button>
        ))}
        <div className="foot">
          <div>{me.email}</div>
          <button onClick={() => { setToken(""); setMe(null); }} style={{ marginTop: 8 }}>Çıkış</button>
          <div style={{ marginTop: 12 }}><a href="/docs" target="_blank" rel="noreferrer" style={{ color: "#93c5fd" }}>API Dokümanı</a></div>
        </div>
      </aside>
      <main className="main"><Page /></main>
    </div>
  );
}
