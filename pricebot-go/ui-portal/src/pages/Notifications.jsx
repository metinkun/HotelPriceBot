import React, { useEffect, useState } from "react";
import { api, fmtDate } from "../api.js";

export default function Notifications({ me, apiBase = "/api/portal" }) {
  const [data, setData] = useState({ unread: 0, items: [] });
  const [err, setErr] = useState("");
  const label = (n) => (me?.providers?.find((p) => p.name === n) || {}).label || n;
  const load = () => api(`${apiBase}/notifications?limit=100`).then(setData).catch((e) => setErr(e.message));
  useEffect(() => { load(); }, []);
  const readAll = async () => { await api(`${apiBase}/notifications/read`, { method: "POST", body: {} }); load(); };
  return (
    <div className="card">
      <div className="row" style={{ justifyContent: "space-between" }}><h3>Bildirimler {data.unread > 0 && <span className="badge warn">{data.unread} okunmadı</span>}</h3>{data.unread > 0 && <button className="btn secondary" onClick={readAll}>Tümünü okundu yap</button>}</div>
      {err && <div className="err">{err}</div>}
      <table><tbody>{data.items.map((n) => (
        <tr key={n.id} style={{ fontWeight: n.readAt ? 400 : 600 }}>
          <td className="muted" style={{ whiteSpace: "nowrap" }}>{fmtDate(n.createdAt)}</td>
          <td>{n.provider && <span className="badge brand">{label(n.provider)}</span>}</td>
          <td>{n.title}<div className="muted" style={{ fontWeight: 400 }}>{n.body}</div></td>
          <td>{n.internalHotelId && <a href={`#hoteldetail/${n.internalHotelId}/${n.provider || ""}`}>Detayı aç →</a>}</td>
        </tr>))}</tbody></table>
      {data.items.length === 0 && <div className="muted">Bildirim yok.</div>}
    </div>
  );
}
