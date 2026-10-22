import React, { useEffect, useState } from "react";
import { api } from "../api.js";

export default function Hotels({ me }) {
  const [hotels, setHotels] = useState([]);
  const [name, setName] = useState("");
  const [urls, setUrls] = useState({});
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [result, setResult] = useState(null);
  const enabled = me.providers.filter((p) => p.enabled);
  const load = () => api("/api/portal/hotels").then(setHotels).catch((e) => setErr(e.message));
  useEffect(() => { load(); }, []);

  const add = async (e) => {
    e.preventDefault(); setBusy(true); setErr(""); setResult(null);
    const clean = Object.fromEntries(Object.entries(urls).filter(([, v]) => v && v.trim()));
    if (!Object.keys(clean).length) { setErr("En az bir site linki girin."); setBusy(false); return; }
    try { const r = await api("/api/portal/hotels", { method: "POST", body: { hotelName: name, urls: clean } }); setResult(r); setName(""); setUrls({}); await load(); }
    catch (ex) { setErr(ex.message + (ex.data?.details ? " " + JSON.stringify(ex.data.details) : "")); } finally { setBusy(false); }
  };
  const remove = async (id) => { if (!confirm("Otel listenizden kaldırılsın mı?")) return; try { await api("/api/portal/hotels/" + id, { method: "DELETE" }); await load(); } catch (e) { setErr(e.message); } };

  return (
    <div>
      {err && <div className="err">{err}</div>}
      <div className="card">
        <h3>Yeni otel ekle</h3>
        <p className="muted">Otelin her sitedeki detay sayfası linkini yapıştırın. Sistem otel kimliğini otomatik çözer; en az bir site yeterlidir.</p>
        <form onSubmit={add}>
          <div><label>Otel adı</label><input value={name} onChange={(e) => setName(e.target.value)} required style={{ width: 360 }} placeholder="Örn. Acapulco Resort" /></div>
          <div className="row" style={{ marginTop: 10 }}>
            {enabled.map((p) => <div key={p.name}><label>{p.label} <span className="muted">({p.domain})</span></label><input placeholder={"https://www." + p.domain + "/..."} value={urls[p.name] || ""} onChange={(e) => setUrls({ ...urls, [p.name]: e.target.value })} style={{ width: 330 }} /></div>)}
          </div>
          <button className="btn" style={{ marginTop: 12 }} disabled={busy}>{busy ? "Çözümleniyor…" : "Oteli ekle"}</button>
        </form>
        {result && <div style={{ marginTop: 10 }}><span className="badge ok">eklendi: {result.hotelName}</span> {result.errors?.map((e) => <div key={e.provider} className="err">{e.provider}: {e.error}</div>)}</div>}
      </div>
      <div className="card">
        <h3>Otellerim ({hotels.length})</h3>
        {hotels.length === 0 && <div className="muted">Henüz otel eklenmedi.</div>}
        {hotels.map((h) => (
          <div className="hotel-card" key={h.internalHotelId}>
            <div><b>{h.hotelName || h.internalHotelId}</b><div className="chips" style={{ marginTop: 6 }}>{h.providers.map((p) => <a key={p.provider} className={"chip " + (p.enabled ? "on" : "off")} href={p.sourceUrl || "#"} target="_blank" rel="noreferrer" title={p.sourceUrl || ""}>{(me.providers.find((x) => x.name === p.provider) || {}).label || p.provider}</a>)}</div></div>
            <div className="row"><button className="btn ghost" onClick={() => { location.hash = "search/" + h.internalHotelId; }}>Fiyat ara</button><button className="btn secondary" onClick={() => remove(h.internalHotelId)}>Kaldır</button></div>
          </div>
        ))}
      </div>
    </div>
  );
}
