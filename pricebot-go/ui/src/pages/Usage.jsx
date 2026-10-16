import React, { useEffect, useState } from "react";
import { api } from "../api.js";

const d = (n) => { const t = new Date(); t.setDate(t.getDate() + n); return t.toISOString().slice(0, 10); };

export default function Usage() {
  const [from, setFrom] = useState(d(-14));
  const [to, setTo] = useState(d(0));
  const [clientId, setClientId] = useState("");
  const [clients, setClients] = useState([]);
  const [data, setData] = useState(null);
  const [err, setErr] = useState("");

  useEffect(() => { api("/api/admin/clients").then(setClients).catch(() => {}); }, []);
  const load = async () => { try { setData(await api(`/api/admin/usage?from=${from}&to=${to}${clientId ? "&clientId=" + clientId : ""}`)); setErr(""); } catch (e) { setErr(e.message); } };
  useEffect(() => { load(); }, [from, to, clientId]);

  const name = (id) => clients.find((c) => c.id === id)?.name || `#${id}`;
  return (
    <div>
      <h2>Kullanım</h2>
      <div className="card row">
        <div><label>Başlangıç</label><input type="date" value={from} onChange={(e) => setFrom(e.target.value)} /></div>
        <div><label>Bitiş</label><input type="date" value={to} onChange={(e) => setTo(e.target.value)} /></div>
        <div><label>Müşteri</label><select value={clientId} onChange={(e) => setClientId(e.target.value)}><option value="">Tümü</option>{clients.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}</select></div>
      </div>
      {err && <div className="err">{err}</div>}
      {data && (<>
        <div className="grid">
          <div className="stat"><div className="k">Kredi (site çağrısı)</div><div className="v">{data.totals.credits}</div></div>
          <div className="stat"><div className="k">İstek</div><div className="v">{data.totals.requests}</div></div>
        </div>
        <div className="card" style={{ marginTop: 16 }}><table>
          <thead><tr><th>Gün</th><th>Müşteri</th><th>Provider</th><th>Kredi</th><th>İstek</th></tr></thead>
          <tbody>{data.rows.map((r, i) => <tr key={i}><td>{r.day}</td><td>{name(r.clientId)}</td><td>{r.provider === "*" ? <span className="muted">(istek sayacı)</span> : r.provider}</td><td>{r.credits}</td><td>{r.requests}</td></tr>)}</tbody>
        </table>{data.rows.length === 0 && <div className="muted">Kayıt yok.</div>}</div>
      </>)}
    </div>
  );
}
