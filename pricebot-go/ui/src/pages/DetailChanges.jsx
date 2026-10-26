import React, { useEffect, useState } from "react";
import { api } from "../api.js";
const fmt = (s) => (s ? new Date(s).toLocaleString("tr-TR") : "—");

export default function DetailChanges() {
  const [d, setD] = useState(null);
  const [err, setErr] = useState("");
  const [msg, setMsg] = useState("");
  const load = () => api("/api/admin/details/changes?limit=200").then(setD).catch((e) => setErr(e.message));
  useEffect(() => { load(); }, []);
  const runAll = async () => { try { const r = await api("/api/admin/details/run", { method: "POST" }); setMsg(r.message); } catch (e) { setErr(e.message); } };
  if (!d) return <div className="card">{err || "Yükleniyor…"}</div>;
  return (
    <div>
      <div className="row" style={{ justifyContent: "space-between" }}><h2>Otel detay değişiklikleri</h2><button className="btn" onClick={runAll}>Tüm otelleri şimdi kontrol et</button></div>
      {err && <div className="err">{err}</div>}{msg && <div className="badge ok">{msg}</div>}
      <div className="card"><b>Detay destekleyen acenteler:</b> {d.supported.join(", ") || "—"}
        {d.lastRun && <div className="muted" style={{ marginTop: 6 }}>Son kontrol: {fmt(d.lastRun.startedAt)} → {fmt(d.lastRun.finishedAt)} · {d.lastRun.total} kayıt · {d.lastRun.changed} değişti · {d.lastRun.failed} hata</div>}</div>
      <div className="card"><table><thead><tr><th>Zaman</th><th>Otel</th><th>Acente</th><th>Özet</th><th></th></tr></thead>
        <tbody>{d.items.map((c) => <tr key={c.id}><td className="muted">{fmt(c.changedAt)}</td><td><code>{c.internalHotelId}</code></td><td><span className="badge brand">{c.provider}</span></td><td>{c.summary}</td><td><a href={`#hoteldetail/${c.internalHotelId}`}>Detay →</a></td></tr>)}</tbody></table>
        {d.items.length === 0 && <div className="muted">Henüz değişiklik kaydı yok. Günlük kontrol her gece 04:00'te çalışır.</div>}</div>
    </div>
  );
}
