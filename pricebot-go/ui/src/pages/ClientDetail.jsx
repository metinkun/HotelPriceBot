import React, { useEffect, useState } from "react";
import { api } from "../api.js";

const WEEKDAYS = ["Paz", "Pzt", "Sal", "Çar", "Per", "Cum", "Cmt"];

// Musteri detayi: tip, etkin siteler (ac/kapa), portal kullanicilari, oteller, planli aramalar, arama gecmisi
export default function ClientDetail({ clientId, onClose }) {
  const [data, setData] = useState(null);
  const [searches, setSearches] = useState([]);
  const [err, setErr] = useState("");
  const [newUser, setNewUser] = useState({ email: "", password: "", name: "" });

  const load = async () => {
    try { const [d, s] = await Promise.all([api(`/api/admin/clients/${clientId}/settings`), api(`/api/admin/clients/${clientId}/searches?limit=20`)]); setData(d); setSearches(s); setErr(""); }
    catch (e) { setErr(e.message); }
  };
  useEffect(() => { load(); }, [clientId]);
  if (!data) return <div className="card">{err ? <div className="err">{err}</div> : "Yükleniyor…"}</div>;

  const st = data.settings;
  const enabled = new Set(st.enabledProviders);
  const allOn = st.enabledProviders.length === 0;
  const save = async (patch) => { try { await api(`/api/admin/clients/${clientId}/settings`, { method: "PUT", body: patch }); await load(); } catch (e) { setErr(e.message); } };
  const toggleProv = (n) => {
    // bos liste = hepsi etkin; ilk kapatmada listeyi tum provider'lar ile baslat
    let list = allOn ? [...data.providers] : [...st.enabledProviders];
    list = list.includes(n) ? list.filter((x) => x !== n) : [...list, n];
    save({ enabledProviders: list });
  };
  const createUser = async (e) => { e.preventDefault(); try { await api(`/api/admin/clients/${clientId}/portal-users`, { method: "POST", body: { ...newUser, name: newUser.name || null } }); setNewUser({ email: "", password: "", name: "" }); await load(); } catch (ex) { setErr(ex.message); } };
  const userAction = async (u, patch) => { try { await api(`/api/admin/portal-users/${u.id}`, { method: "PUT", body: patch }); await load(); } catch (e) { setErr(e.message); } };
  const resetPw = (u) => { const p = prompt(`${u.email} için yeni şifre (min 8):`); if (p && p.length >= 8) userAction(u, { password: p }); };
  const delUser = async (u) => { if (!confirm(`${u.email} silinsin mi?`)) return; try { await api(`/api/admin/portal-users/${u.id}`, { method: "DELETE" }); await load(); } catch (e) { setErr(e.message); } };

  return (
    <div className="card">
      <div className="row" style={{ justifyContent: "space-between" }}><h3>Müşteri ayarları #{clientId}</h3><button className="btn secondary" onClick={onClose}>Kapat</button></div>
      {err && <div className="err">{err}</div>}
      <div className="row" style={{ marginTop: 8 }}>
        <div><label>Müşteri tipi</label>
          <select value={st.clientType} onChange={(e) => save({ clientType: e.target.value })}>
            <option value="api">API entegrasyonu</option><option value="portal">Web portal</option><option value="both">API + portal</option>
          </select></div>
        <div className="muted" style={{ paddingBottom: 8 }}>Portal: <a href="/portal/" target="_blank" rel="noreferrer">/portal/</a> · API: X-API-Key ile <code>/api/v1/*</code></div>
      </div>

      <h3 style={{ marginTop: 18 }}>Etkin siteler {allOn && <span className="badge ok">tümü</span>}</h3>
      <div className="row">{data.providers.map((n) => {
        const on = allOn || enabled.has(n);
        return <label key={n} style={{ display: "flex", alignItems: "center", gap: 6, padding: "6px 10px", border: "1px solid #e5e7eb", borderRadius: 8, background: on ? "#dcfce7" : "#fff", cursor: "pointer" }}>
          <input type="checkbox" checked={on} onChange={() => toggleProv(n)} style={{ minWidth: 0 }} /> {n}</label>;
      })}</div>
      <p className="muted" style={{ fontSize: 12 }}>Kapalı siteler portalda görünmez ve API'de 403 döner. Plan kısıtları ayrıca uygulanır.</p>

      <h3 style={{ marginTop: 18 }}>Portal kullanıcıları</h3>
      <table><thead><tr><th>E-posta</th><th>Ad</th><th>Son giriş</th><th>Durum</th><th></th></tr></thead>
        <tbody>{data.portalUsers.map((u) => <tr key={u.id}><td>{u.email}</td><td>{u.name}</td><td className="muted">{u.lastLoginAt ? new Date(u.lastLoginAt).toLocaleString("tr-TR") : "—"}</td>
          <td>{u.isActive ? <span className="badge ok">aktif</span> : <span className="badge bad">pasif</span>}</td>
          <td style={{ whiteSpace: "nowrap" }}><button className="btn secondary" onClick={() => resetPw(u)}>Şifre</button> <button className="btn secondary" onClick={() => userAction(u, { isActive: !u.isActive })}>{u.isActive ? "Pasife al" : "Aktifleştir"}</button> <button className="btn danger" onClick={() => delUser(u)}>Sil</button></td></tr>)}</tbody></table>
      <form className="row" onSubmit={createUser} style={{ marginTop: 10 }}>
        <div><label>E-posta</label><input type="email" value={newUser.email} onChange={(e) => setNewUser({ ...newUser, email: e.target.value })} required /></div>
        <div><label>Şifre (min 8)</label><input type="password" minLength={8} value={newUser.password} onChange={(e) => setNewUser({ ...newUser, password: e.target.value })} required /></div>
        <div><label>Ad</label><input value={newUser.name} onChange={(e) => setNewUser({ ...newUser, name: e.target.value })} /></div>
        <button className="btn">Kullanıcı ekle</button>
      </form>

      <h3 style={{ marginTop: 18 }}>Oteller ({data.hotels.length})</h3>
      <div className="row">{data.hotels.map((h) => <span key={h.id} className="badge">{h.label || h.internalHotelId}</span>)}{data.hotels.length === 0 && <span className="muted">yok</span>}</div>

      <h3 style={{ marginTop: 18 }}>Planlı aramalar ({data.schedules.length})</h3>
      <table><thead><tr><th>Ad</th><th>Zaman</th><th>Sonraki</th><th>Durum</th></tr></thead>
        <tbody>{data.schedules.map((s) => <tr key={s.id}><td>{s.name}</td><td>{s.weekdays.length ? s.weekdays.map((d) => WEEKDAYS[d]).join(", ") : "Her gün"} {s.runTime}</td><td>{s.nextRunAt ? new Date(s.nextRunAt).toLocaleString("tr-TR") : "—"}</td><td>{s.isActive ? <span className="badge ok">aktif</span> : <span className="badge">pasif</span>}</td></tr>)}</tbody></table>

      <h3 style={{ marginTop: 18 }}>Son aramalar</h3>
      <table><thead><tr><th>Zaman</th><th>Tür</th><th>Durum</th><th>Kaynak</th></tr></thead>
        <tbody>{searches.map((s) => <tr key={s.id}><td className="muted">{new Date(s.createdAt).toLocaleString("tr-TR")}</td><td>{s.kind}</td><td><span className={"badge " + (s.status === "done" ? "ok" : s.status === "failed" ? "bad" : "warn")}>{s.status}</span></td><td>{s.scheduleId ? "planlı" : "anlık"}</td></tr>)}</tbody></table>
      {searches.length === 0 && <div className="muted">Kayıt yok.</div>}
    </div>
  );
}
