import React, { useEffect, useState } from "react";
import { api } from "../api.js";

export default function Dashboard() {
  const [health, setHealth] = useState(null);
  const [jobs, setJobs] = useState(null);
  const [usage, setUsage] = useState(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  const load = async () => {
    try {
      const [h, j, u] = await Promise.all([api("/api/admin/providers/health"), api("/api/admin/jobs/stats"), api("/api/admin/usage?from=" + today() + "&to=" + today())]);
      setHealth(h); setJobs(j); setUsage(u); setErr("");
    } catch (e) { setErr(e.message); }
  };
  useEffect(() => { load(); const t = setInterval(load, 15000); return () => clearInterval(t); }, []);

  const runCanary = async () => { setBusy(true); try { await api("/api/admin/canary/run", { method: "POST" }); await load(); } catch (e) { setErr(e.message); } finally { setBusy(false); } };
  const reset = async (name) => { try { await api(`/api/admin/providers/${name}/reset-circuit`, { method: "POST" }); await load(); } catch (e) { setErr(e.message); } };

  if (err) return <div className="err">{err}</div>;
  if (!health) return <div className="muted">Yükleniyor…</div>;
  const canary = health.canary;
  return (
    <div>
      <h2>Özet</h2>
      <div className="grid">
        <div className="stat"><div className="k">Bugünkü kredi</div><div className="v">{usage?.totals?.credits ?? 0}</div></div>
        <div className="stat"><div className="k">Bugünkü istek</div><div className="v">{usage?.totals?.requests ?? 0}</div></div>
        <div className="stat"><div className="k">Kuyrukta / çalışan iş</div><div className="v">{jobs?.queued ?? 0} / {jobs?.running ?? 0}</div></div>
        <div className="stat"><div className="k">Cache</div><div className="v">{health.cache?.enabled ? `${health.cache.ttlSeconds}s` : "kapalı"}</div></div>
        <div className="stat"><div className="k">Kanarya</div><div className="v">{canary ? `${canary.healthy}/${canary.total}` : "—"}</div>
          <div className="muted" style={{ fontSize: 12 }}>{canary ? new Date(canary.ranAt).toLocaleString("tr-TR") : "henüz çalışmadı"}</div></div>
      </div>
      <div className="card" style={{ marginTop: 16 }}>
        <div className="row" style={{ justifyContent: "space-between" }}><h3>Provider sağlığı</h3>
          <button className="btn secondary" onClick={runCanary} disabled={busy}>{busy ? "Çalışıyor…" : "Kanaryayı şimdi çalıştır"}</button></div>
        <table>
          <thead><tr><th>Provider</th><th>Durum</th><th>Eşzamanlı</th><th>Toplam</th><th>Başarılı</th><th>Hata</th><th>Bulunamadı</th><th>Ort. ms</th><th>Son hata</th><th></th></tr></thead>
          <tbody>
            {health.providers.map((p) => {
              const c = canary?.checks?.find((x) => x.provider === p.provider);
              const badge = p.circuitOpen ? <span className="badge bad">devre açık</span> : c ? (c.ok && c.found ? <span className="badge ok">sağlıklı</span> : <span className="badge warn">kanarya: {c.error ? "hata" : "fiyat yok"}</span>) : <span className="badge">bilinmiyor</span>;
              return (
                <tr key={p.provider}>
                  <td><b>{p.provider}</b>{health.capabilities?.[p.provider]?.requiresPuppeteer && <span className="badge" style={{ marginLeft: 6 }}>tarayıcı</span>}</td>
                  <td>{badge}</td><td>{p.inFlight}/{p.concurrency}</td><td>{p.total}</td><td>{p.success}</td><td>{p.failed}</td><td>{p.notFound}</td><td>{p.avgLatencyMs}</td>
                  <td className="muted" style={{ maxWidth: 260, fontSize: 12 }}>{p.lastError}</td>
                  <td>{p.circuitOpen && <button className="btn secondary" onClick={() => reset(p.provider)}>Sıfırla</button>}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function today() { return new Date().toISOString().slice(0, 10); }
