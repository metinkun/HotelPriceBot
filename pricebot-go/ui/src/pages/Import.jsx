import React, { useEffect, useState } from "react";
import { api } from "../api.js";

// Toplu otel ekleme: isim listesi -> acentelerde arama -> eslesme raporu -> onayla ve ekle
export default function Import() {
  const [providers, setProviders] = useState([]);
  const [text, setText] = useState("");
  const [minScore, setMinScore] = useState(0.6);
  const [sel, setSel] = useState([]);
  const [rep, setRep] = useState(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  useEffect(() => { api("/api/admin/hotels/import/providers").then((d) => { setProviders(d.providers); setSel(d.providers); }).catch((e) => setErr(e.message)); }, []);
  const loadTRNC = async () => { try { const d = await api("/api/admin/hotels/import/trnc"); setText(d.map((h) => `${h.externalId}\t${h.name}`).join("\n")); } catch (e) { setErr(e.message); } };
  const parse = () => text.split("\n").map((l) => l.trim()).filter(Boolean).map((l, i) => { const m = l.match(/^(\d+)\s*[\t;,]\s*(.+)$/); return m ? { externalId: Number(m[1]), name: m[2].trim() } : { externalId: i + 1, name: l }; });
  const run = async (apply) => {
    setBusy(true); setErr(""); setRep(null);
    try {
      const started = await api("/api/admin/hotels/import", { method: "POST", body: { hotels: parse(), providers: sel, minScore: Number(minScore), apply } });
      const poll = async () => {
        const r = await api("/api/admin/hotels/import/" + started.importId);
        setRep(r);
        if (r.status === "running") setTimeout(poll, 3000); else setBusy(false);
      };
      poll();
    } catch (e) { setErr(e.message); setBusy(false); }
  };
  const toggle = (n) => setSel(sel.includes(n) ? sel.filter((x) => x !== n) : [...sel, n]);
  return (
    <div>
      <h2>Toplu otel ekle</h2>
      {err && <div className="err">{err}</div>}
      <div className="card">
        <p className="muted">Her satır bir otel: <code>ID[tab]Otel adı</code> veya sadece ad. Sistem her acentede isimle arar, en iyi eşleşmeyi puanlar. Önce "Sadece ara" ile raporu inceleyin, sonra "Ara ve ekle".</p>
        <div className="row"><button className="btn secondary" onClick={loadTRNC}>KKTC listesini yükle (78 otel)</button>
          <div><label>Eşik (0-1)</label><input type="number" step="0.05" min="0.3" max="1" value={minScore} onChange={(e) => setMinScore(e.target.value)} style={{ width: 90 }} /></div></div>
        <textarea value={text} onChange={(e) => setText(e.target.value)} rows={10} style={{ width: "100%", marginTop: 10, fontFamily: "monospace" }} placeholder={"3234\tACAPULCO RESORT HOTEL\n5\tMERIT CRYSTAL COVE HOTEL & CASINO"} />
        <div style={{ marginTop: 8 }}><label>Aranacak acenteler ({sel.length}/{providers.length})</label><div className="row">{providers.map((p) => <label key={p}><input type="checkbox" checked={sel.includes(p)} onChange={() => toggle(p)} /> {p}</label>)}</div>{providers.length === 0 && <span className="muted">Henüz isimle arama destekleyen acente yok.</span>}</div>
        <div className="row" style={{ marginTop: 12 }}><button className="btn secondary" disabled={busy || !text.trim()} onClick={() => run(false)}>{busy ? "Aranıyor…" : "Sadece ara (rapor)"}</button><button className="btn" disabled={busy || !text.trim()} onClick={() => { if (confirm("Eşik üstü eşleşmelerle oteller oluşturulacak. Devam?")) run(true); }}>Ara ve ekle</button></div>
      </div>
      {rep && (
        <div className="card">
          <h3>Rapor · {rep.done}/{rep.total} otel · {rep.providers.length} acente · {rep.duration} {rep.status === "running" ? <span className="badge warn">çalışıyor…</span> : rep.applied ? <span className="badge ok">eklendi</span> : <span className="badge">rapor</span>}</h3>
          {rep.status === "running" && <div className="progress" style={{ height: 6, background: "#e5e7eb", borderRadius: 999, overflow: "hidden", margin: "6px 0 10px" }}><i style={{ display: "block", height: "100%", width: (100 * rep.done / Math.max(1, rep.total)) + "%", background: "#1d4ed8" }} /></div>}
          <table><thead><tr><th>Otel</th><th>Eşleşen</th><th>Düşük güven</th><th>Sonuç</th></tr></thead>
            <tbody>{rep.results.map((r) => <tr key={r.externalId}><td><b>{r.name}</b><div className="muted">#{r.externalId}</div></td>
              <td>{r.matches.map((m) => <div key={m.provider}><span className={"badge " + (m.ambiguous ? "warn" : "ok")}>{m.provider}</span> <a href={m.url} target="_blank" rel="noreferrer">{m.name}</a> <span className="muted">{Math.round(m.score * 100)}%{m.ambiguous ? " · belirsiz" : ""}</span></div>)}</td>
              <td>{r.lowConfidence.map((m) => <div key={m.provider} className="muted">{m.provider}: {m.name} ({Math.round(m.score * 100)}%)</div>)}</td>
              <td>{r.internalHotelId && <a href={"#hoteldetail/" + r.internalHotelId}>{r.internalHotelId}</a>}{r.errors?.map((e, i) => <div key={i} className="err" style={{ fontSize: 12 }}>{e}</div>)}</td></tr>)}</tbody></table>
        </div>
      )}
    </div>
  );
}
