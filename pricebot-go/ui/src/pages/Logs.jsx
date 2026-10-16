import React, { useEffect, useState } from "react";
import { api } from "../api.js";

export default function Logs() {
  const [rows, setRows] = useState([]);
  const [clients, setClients] = useState([]);
  const [clientId, setClientId] = useState("");
  const [err, setErr] = useState("");
  useEffect(() => { api("/api/admin/clients").then(setClients).catch(() => {}); }, []);
  const load = async () => { try { setRows(await api(`/api/admin/logs?limit=200${clientId ? "&clientId=" + clientId : ""}`)); setErr(""); } catch (e) { setErr(e.message); } };
  useEffect(() => { load(); const t = setInterval(load, 10000); return () => clearInterval(t); }, [clientId]);
  const name = (id) => clients.find((c) => c.id === id)?.name || (id ? `#${id}` : "—");
  return (
    <div>
      <h2>İstek logları</h2>
      <div className="card row"><div><label>Müşteri</label><select value={clientId} onChange={(e) => setClientId(e.target.value)}><option value="">Tümü</option>{clients.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}</select></div></div>
      {err && <div className="err">{err}</div>}
      <div className="card"><table>
        <thead><tr><th>Zaman</th><th>Müşteri</th><th>İstek</th><th>Durum</th><th>Süre</th><th>IP</th><th>Request Id</th></tr></thead>
        <tbody>{rows.map((r) => <tr key={r.id}><td className="muted">{new Date(r.at).toLocaleString("tr-TR")}</td><td>{name(r.clientId)}</td><td><code>{r.method} {r.path}</code></td>
          <td><span className={"badge " + (r.status < 400 ? "ok" : r.status < 500 ? "warn" : "bad")}>{r.status}</span></td><td>{r.durationMs} ms</td><td className="muted">{r.ip}</td><td className="muted" style={{ fontSize: 12 }}>{r.requestId}</td></tr>)}</tbody>
      </table>{rows.length === 0 && <div className="muted">Kayıt yok.</div>}</div>
    </div>
  );
}
