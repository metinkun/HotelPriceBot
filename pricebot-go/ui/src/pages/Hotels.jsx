import React, { useEffect, useState } from "react";
import { api } from "../api.js";

export default function Hotels() {
  const [hotels, setHotels] = useState([]);
  const [providers, setProviders] = useState([]);
  const [name, setName] = useState("");
  const [urls, setUrls] = useState({});
  const [result, setResult] = useState(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  const load = async () => { try { const [h, hh] = await Promise.all([api("/api/admin/hotels"), fetch("/health").then((r) => r.json())]); setHotels(h); setProviders(hh.providers || []); } catch (e) { setErr(e.message); } };
  useEffect(() => { load(); }, []);

  const add = async (e) => {
    e.preventDefault(); setBusy(true); setErr(""); setResult(null);
    const clean = Object.fromEntries(Object.entries(urls).filter(([, v]) => v && v.trim()));
    try { const r = await api("/api/admin/hotels", { method: "POST", body: { hotelName: name, urls: clean } }); setResult(r); setName(""); setUrls({}); await load(); }
    catch (ex) { setErr(ex.message + (ex.data?.details ? " " + JSON.stringify(ex.data.details) : "")); } finally { setBusy(false); }
  };
  const remove = async (id) => { if (!confirm(`${id} silinsin mi?`)) return; try { await api("/api/admin/hotels/" + id, { method: "DELETE" }); await load(); } catch (e) { setErr(e.message); } };

  return (
    <div>
      <h2>Oteller</h2>
      {err && <div className="err">{err}</div>}
      <div className="card"><h3>URL'lerden otel ekle</h3>
        <form onSubmit={add}>
          <div><label>Otel adı *</label><input value={name} onChange={(e) => setName(e.target.value)} required style={{ width: 360 }} /></div>
          <div className="row" style={{ marginTop: 8 }}>{providers.map((p) => <div key={p}><label>{p}</label><input placeholder={`https://…`} value={urls[p] || ""} onChange={(e) => setUrls({ ...urls, [p]: e.target.value })} style={{ width: 300 }} /></div>)}</div>
          <button className="btn" style={{ marginTop: 10 }} disabled={busy}>{busy ? "Çözümleniyor… (Touristica 20-60 sn sürebilir)" : "Ekle"}</button>
        </form>
        {result && <div style={{ marginTop: 10 }}><span className="badge ok">eklendi: {result.internalHotelId}</span> {result.errors?.length > 0 && <span className="badge warn">hatalı: {result.errors.map((e) => e.provider).join(", ")}</span>}
          {result.errors?.map((e) => <div key={e.provider} className="err">{e.provider}: {e.error}</div>)}</div>}
      </div>
      <div className="card"><table>
        <thead><tr><th>Id</th><th>Ad</th><th>Provider'lar</th><th></th></tr></thead>
        <tbody>{hotels.map((h) => <tr key={h.internalHotelId}><td><code>{h.internalHotelId}</code></td><td>{h.hotelName}</td>
          <td>{h.providers.map((p) => <span key={p.provider} className={"badge " + (p.isActive ? "ok" : "")} style={{ marginRight: 4 }} title={p.sourceUrl || ""}>{p.provider}</span>)}</td>
          <td style={{ whiteSpace: "nowrap" }}><button className="btn secondary" onClick={() => { location.hash = "hoteldetail/" + h.internalHotelId; }}>Detay</button> <button className="btn danger" onClick={() => remove(h.internalHotelId)}>Sil</button></td></tr>)}</tbody>
      </table></div>
    </div>
  );
}
