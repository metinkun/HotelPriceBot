import React, { useEffect, useState } from "react";
import { api } from "../api.js";

const WEEKDAYS = ["Paz", "Pzt", "Sal", "Çar", "Per", "Cum", "Cmt"];

export default function Schedules() {
  const [list, setList] = useState([]);
  const [clients, setClients] = useState([]);
  const [err, setErr] = useState("");
  useEffect(() => { Promise.all([api("/api/admin/schedules"), api("/api/admin/clients")]).then(([s, c]) => { setList(s); setClients(c); }).catch((e) => setErr(e.message)); }, []);
  const name = (id) => clients.find((c) => c.id === id)?.name || `#${id}`;
  return (
    <div>
      <h2>Planlı aramalar (tüm müşteriler)</h2>
      {err && <div className="err">{err}</div>}
      <div className="card"><table>
        <thead><tr><th>Müşteri</th><th>Ad</th><th>Zaman</th><th>Tür</th><th>Sonraki</th><th>Son</th><th>Durum</th></tr></thead>
        <tbody>{list.map((s) => { const r = JSON.parse(s.request); return <tr key={s.id}><td><b>{name(s.clientId)}</b></td><td>{s.name}</td>
          <td>{s.weekdays.length ? s.weekdays.map((d) => WEEKDAYS[d]).join(", ") : "Her gün"} {s.runTime} <span className="muted">({s.timezone})</span></td>
          <td>{s.kind} · {(r.hotelIds || []).length} otel · +{r.daysAhead}g/{r.nights}gece</td>
          <td>{s.nextRunAt ? new Date(s.nextRunAt).toLocaleString("tr-TR") : "—"}</td><td className="muted">{s.lastRunAt ? new Date(s.lastRunAt).toLocaleString("tr-TR") : "—"}</td>
          <td>{s.isActive ? <span className="badge ok">aktif</span> : <span className="badge">pasif</span>}</td></tr>; })}</tbody>
      </table>{list.length === 0 && <div className="muted">Plan yok.</div>}</div>
    </div>
  );
}
