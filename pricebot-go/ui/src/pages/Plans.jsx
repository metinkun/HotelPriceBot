import React, { useEffect, useState } from "react";
import { api } from "../api.js";

const empty = { code: "", name: "", dailyQuota: 0, allowedProviders: [], allowPackages: true, allowFresh: false, maxConcurrency: 5, ratePerMinute: 60, isActive: true };

export default function Plans() {
  const [plans, setPlans] = useState([]);
  const [providers, setProviders] = useState([]);
  const [edit, setEdit] = useState(null);
  const [err, setErr] = useState("");

  const load = async () => { try { const [p, h] = await Promise.all([api("/api/admin/plans"), fetch("/health").then((r) => r.json())]); setPlans(p); setProviders(h.providers || []); } catch (e) { setErr(e.message); } };
  useEffect(() => { load(); }, []);

  const save = async (e) => {
    e.preventDefault();
    try {
      const body = { ...edit, dailyQuota: Number(edit.dailyQuota), maxConcurrency: Number(edit.maxConcurrency), ratePerMinute: Number(edit.ratePerMinute) };
      if (edit.id) await api("/api/admin/plans/" + edit.id, { method: "PUT", body }); else await api("/api/admin/plans", { method: "POST", body });
      setEdit(null); await load(); setErr("");
    } catch (ex) { setErr(ex.message); }
  };
  const toggleProv = (n) => { const s = new Set(edit.allowedProviders); s.has(n) ? s.delete(n) : s.add(n); setEdit({ ...edit, allowedProviders: [...s] }); };

  return (
    <div>
      <div className="row" style={{ justifyContent: "space-between" }}><h2>Planlar</h2><button className="btn" onClick={() => setEdit({ ...empty })}>Yeni plan</button></div>
      {err && <div className="err">{err}</div>}
      <p className="muted">Kota birimi: <b>kredi</b>. Bir siteye atılan her gerçek sorgu 1 kredi; cache'ten dönen sonuç 0 kredi. 0 = sınırsız. İzinli provider boş = tümü.</p>
      <div className="card"><table>
        <thead><tr><th>Kod</th><th>Ad</th><th>Günlük kredi</th><th>Provider</th><th>Paket</th><th>Fresh</th><th>İstek/dk</th><th>Durum</th><th></th></tr></thead>
        <tbody>{plans.map((p) => (
          <tr key={p.id}><td><code>{p.code}</code></td><td>{p.name}</td><td>{p.dailyQuota || "∞"}</td><td>{p.allowedProviders.length ? p.allowedProviders.join(", ") : <span className="muted">tümü</span>}</td>
            <td>{p.allowPackages ? "✓" : "—"}</td><td>{p.allowFresh ? "✓" : "—"}</td><td>{p.ratePerMinute}</td>
            <td>{p.isActive ? <span className="badge ok">aktif</span> : <span className="badge bad">pasif</span>}</td>
            <td><button className="btn secondary" onClick={() => setEdit({ ...p })}>Düzenle</button></td></tr>))}</tbody>
      </table></div>
      {edit && (
        <div className="card"><h3>{edit.id ? "Planı düzenle" : "Yeni plan"}</h3>
          <form onSubmit={save}>
            <div className="row">
              <div><label>Kod</label><input value={edit.code} onChange={(e) => setEdit({ ...edit, code: e.target.value })} required /></div>
              <div><label>Ad</label><input value={edit.name} onChange={(e) => setEdit({ ...edit, name: e.target.value })} required /></div>
              <div><label>Günlük kredi (0=∞)</label><input type="number" min="0" value={edit.dailyQuota} onChange={(e) => setEdit({ ...edit, dailyQuota: e.target.value })} /></div>
              <div><label>İstek/dk</label><input type="number" min="1" value={edit.ratePerMinute} onChange={(e) => setEdit({ ...edit, ratePerMinute: e.target.value })} /></div>
              <div><label>Eşzamanlı</label><input type="number" min="1" value={edit.maxConcurrency} onChange={(e) => setEdit({ ...edit, maxConcurrency: e.target.value })} /></div>
            </div>
            <div className="row" style={{ marginTop: 10 }}>
              <label><input type="checkbox" checked={edit.allowPackages} onChange={(e) => setEdit({ ...edit, allowPackages: e.target.checked })} /> Paket sorgusu</label>
              <label><input type="checkbox" checked={edit.allowFresh} onChange={(e) => setEdit({ ...edit, allowFresh: e.target.checked })} /> Cache atlama (fresh)</label>
              <label><input type="checkbox" checked={edit.isActive} onChange={(e) => setEdit({ ...edit, isActive: e.target.checked })} /> Aktif</label>
            </div>
            <div style={{ marginTop: 10 }}><label>İzinli provider'lar (boş = tümü)</label>
              <div className="row">{providers.map((n) => <label key={n}><input type="checkbox" checked={edit.allowedProviders.includes(n)} onChange={() => toggleProv(n)} /> {n}</label>)}</div></div>
            <div className="row" style={{ marginTop: 12 }}><button className="btn">Kaydet</button><button type="button" className="btn secondary" onClick={() => setEdit(null)}>Vazgeç</button></div>
          </form>
        </div>
      )}
    </div>
  );
}
