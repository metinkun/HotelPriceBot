import React, { useEffect, useState } from "react";
import { api, fmtDate } from "../api.js";

// Otel detay sayfasi: acente butonlari; secilen acentenin statik verisi ortak basliklar altinda.
// Degisen alanlar ilk goruntulemede sari vurgulanir (sunucu highlightPaths doner, goruldu olarak isaretler).
export default function HotelDetail({ me, hotelId, apiBase = "/api/portal", canRefresh = true }) {
  const [ov, setOv] = useState(null);
  const [prov, setProv] = useState("");
  const [view, setView] = useState(null);
  const [tab, setTab] = useState("ozet");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const label = (n) => (me?.providers?.find((p) => p.name === n) || {}).label || n;

  const loadOv = async () => { try { const d = await api(`${apiBase}/hotels/${hotelId}/details`); setOv(d); setErr(""); const first = d.providers.find((p) => p.fetched) || d.providers.find((p) => p.supported); if (!prov && first) setProv(first.provider); } catch (e) { setErr(e.message); } };
  useEffect(() => { loadOv(); }, [hotelId]);
  useEffect(() => { if (!prov) return; setView(null); api(`${apiBase}/hotels/${hotelId}/details?provider=${prov}`).then(setView).catch((e) => setView({ error: e.message })); }, [prov, hotelId]);

  const refresh = async (p) => { setBusy(true); try { await api(`${apiBase}/hotels/${hotelId}/details/refresh${p ? "?provider=" + p : ""}`, { method: "POST" }); await loadOv(); if (p === prov || !p) { const v = await api(`${apiBase}/hotels/${hotelId}/details?provider=${prov}`); setView(v); } } catch (e) { setErr(e.message); } finally { setBusy(false); } };

  if (!ov) return <div className="card">{err ? <div className="err">{err}</div> : "Yükleniyor…"}</div>;
  const snap = view?.snapshot;
  const hl = new Set(view?.highlightPaths || []);
  const H = ({ path, children, block }) => { const on = [...hl].some((p) => p === path || p.startsWith(path + "[") || p.startsWith(path + ".")); return block ? <div className={on ? "hl" : ""}>{children}</div> : <span className={on ? "hl" : ""}>{children}</span>; };
  const tabs = [["ozet", "Genel"], ["gorseller", "Görseller"], ["ozellikler", "Özellikler"], ["odalar", "Odalar"], ["politika", "Politikalar"], ["sss", "Soru-Cevap"], ["acente", "Acente görünümü"], ["degisiklik", "Değişiklikler"]];

  return (
    <div>
      <div className="card">
        <div className="row" style={{ justifyContent: "space-between" }}>
          <div><h3 style={{ marginBottom: 2 }}>{ov.hotel.hotelName || hotelId}</h3><span className="muted">{hotelId} · {ov.providers.length} acente</span></div>
          {canRefresh && <button className="btn secondary" disabled={busy} onClick={() => refresh("")}>{busy ? "Çekiliyor…" : "Tüm acenteleri yenile"}</button>}
        </div>
        {err && <div className="err">{err}</div>}
        <div className="chips" style={{ marginTop: 12 }}>
          {ov.providers.map((p) => (
            <span key={p.provider} className={"chip " + (p.provider === prov ? "on" : "") + (!p.supported ? " off" : "")} onClick={() => p.supported && setProv(p.provider)} title={p.supported ? (p.fetched ? "v" + p.version + " · " + fmtDate(p.fetchedAt) : "henüz çekilmedi") : "bu acente için detay desteği yok"}>
              {label(p.provider)}{p.unseenChange && <b style={{ color: "#d97706" }}> ●</b>}{!p.fetched && p.supported && <small className="muted"> (yok)</small>}
            </span>))}
        </div>
      </div>

      {prov && (
        <div className="card">
          <div className="row" style={{ justifyContent: "space-between" }}>
            <div><b>{label(prov)}</b> {view?.version ? <span className="muted">· sürüm {view.version} · {fmtDate(view.fetchedAt)}</span> : null}
              {view?.highlight && <span className="badge warn" style={{ marginLeft: 8 }}>Değişti: {view.lastChange?.summary}</span>}</div>
            <div className="row">{snap?.sourceUrl && <a className="btn ghost" href={snap.sourceUrl} target="_blank" rel="noreferrer">Acentede aç ↗</a>}{canRefresh && <button className="btn ghost" disabled={busy} onClick={() => refresh(prov)}>Yenile</button>}</div>
          </div>
          {view?.error && !snap?.name && <div className="err">{view.error}</div>}
          {!view && <div className="muted">Yükleniyor…</div>}
          {snap && (<>
            <div className="chips" style={{ margin: "12px 0" }}>{tabs.map(([k, l]) => <span key={k} className={"chip " + (tab === k ? "on" : "")} onClick={() => setTab(k)}>{l}</span>)}</div>
            {tab === "ozet" && (<div>
              <h3><H path="name">{snap.name}</H> {snap.stars ? <H path="stars">{"★".repeat(snap.stars)}</H> : null}</h3>
              <div className="muted"><H path="address">{snap.address}</H> {snap.city && <H path="city">· {snap.city}</H>} {snap.rating != null && <H path="rating">· Puan {snap.rating}{snap.ratingCount ? ` (${snap.ratingCount} yorum)` : ""}</H>}</div>
              <H path="description" block><p style={{ whiteSpace: "pre-wrap", lineHeight: 1.6 }}>{snap.description || <span className="muted">Açıklama yok</span>}</p></H>
              {snap.boards?.length > 0 && <div><b>Pansiyon:</b> <H path="boards">{snap.boards.join(", ")}</H></div>}
              {snap.contact && <div className="muted" style={{ marginTop: 6 }}><H path="contact">{[snap.contact.phone, snap.contact.email, snap.contact.website].filter(Boolean).join(" · ")}</H></div>}
              {snap.location && <div className="muted">Konum: {snap.location.lat}, {snap.location.lng} · <a href={`https://maps.google.com/?q=${snap.location.lat},${snap.location.lng}`} target="_blank" rel="noreferrer">haritada aç</a></div>}
            </div>)}
            {tab === "gorseller" && (<div className="gallery">{(snap.images || []).map((im, i) => <a key={i} href={im.url} target="_blank" rel="noreferrer" className={hl.has(`images[${i}]`) ? "hl" : ""}><img src={im.url} alt={im.caption || ""} loading="lazy" /></a>)}{!snap.images?.length && <div className="muted">Görsel yok.</div>}</div>)}
            {tab === "ozellikler" && (<div className="row" style={{ alignItems: "flex-start" }}>{(snap.facilities || []).map((g) => <div key={g.title} className="card" style={{ minWidth: 220, flex: 1 }}><h3>{g.title}</h3><ul>{g.items.map((it, i) => <li key={i} className={hl.has(`facilities.${g.title}[${i}]`) || [...hl].some((p) => p.startsWith(`facilities.${g.title}[`) && it && false) ? "hl" : ""}>{it}</li>)}</ul></div>)}{!snap.facilities?.length && <div className="muted">Özellik yok.</div>}</div>)}
            {tab === "odalar" && (<div>{(snap.rooms || []).map((r) => <div key={r.name} className={"card " + ([...hl].some((p) => p.startsWith("rooms." + r.name)) ? "hl" : "")}><h3>{r.name} {r.size ? <span className="muted">· {r.size} m²</span> : null} {r.capacity ? <span className="muted">· {r.capacity}</span> : null}</h3>{r.description && <p>{r.description}</p>}{r.features?.length > 0 && <div className="chips">{r.features.map((f, i) => <span key={i} className="chip">{f}</span>)}</div>}{r.images?.length > 0 && <div className="gallery small">{r.images.slice(0, 6).map((u, i) => <img key={i} src={u} loading="lazy" />)}</div>}</div>)}{!snap.rooms?.length && <div className="muted">Oda bilgisi yok.</div>}</div>)}
            {tab === "politika" && (<table><tbody>{Object.entries(snap.policies || {}).filter(([k, v]) => v && (!Array.isArray(v) || v.length)).map(([k, v]) => <tr key={k} className={hl.has("policies." + k) || [...hl].some((p) => p.startsWith("policies." + k)) ? "hl" : ""}><th style={{ width: 160 }}>{{ checkIn: "Giriş", checkOut: "Çıkış", childPolicy: "Çocuk", petPolicy: "Evcil hayvan", cancellation: "İptal", other: "Diğer" }[k] || k}</th><td>{Array.isArray(v) ? v.join(" · ") : v}</td></tr>)}</tbody></table>)}
            {tab === "sss" && (<div>{(snap.faq || []).map((q, i) => <div key={i} className={"card " + (hl.has("faq." + q.question) ? "hl" : "")}><b>{q.question}</b><p style={{ margin: "6px 0 0" }}>{q.answer}</p></div>)}{!snap.faq?.length && <div className="muted">Soru-cevap yok.</div>}</div>)}
            {tab === "acente" && (<div>{(snap.sections || []).map((s, i) => <div key={i} className={"card " + (hl.has("sections." + s.title) ? "hl" : "")}><h3>{s.title}</h3>{s.kind === "list" && <ul>{s.items.map((x, j) => <li key={j}>{x}</li>)}</ul>}{s.kind === "pairs" && <table><tbody>{s.pairs.map((p, j) => <tr key={j}><th>{p.key}</th><td>{p.value}</td></tr>)}</tbody></table>}{s.kind === "html" && <div dangerouslySetInnerHTML={{ __html: s.html }} />}{(s.kind === "text" || !s.kind) && <p style={{ whiteSpace: "pre-wrap" }}>{s.text}</p>}</div>)}{!snap.sections?.length && <div className="muted">Acente bölümleri yok.</div>}</div>)}
            {tab === "degisiklik" && (<div>{(ov.changes || []).filter((c) => c.provider === prov).map((c) => <div key={c.id} className="card"><b>{fmtDate(c.changedAt)}</b> · sürüm {c.version} · {c.summary}<table style={{ marginTop: 8 }}><tbody>{(c.diff || []).slice(0, 50).map((d, i) => <tr key={i}><td><code>{d.path}</code></td><td><span className={"badge " + (d.kind === "added" ? "ok" : d.kind === "removed" ? "bad" : "warn")}>{d.kind}</span></td><td className="muted" style={{ maxWidth: 260, overflow: "hidden", textOverflow: "ellipsis" }}>{String(d.old ?? "")}</td><td style={{ maxWidth: 260, overflow: "hidden", textOverflow: "ellipsis" }}>{String(d.new ?? "")}</td></tr>)}</tbody></table></div>)}{!(ov.changes || []).some((c) => c.provider === prov) && <div className="muted">Bu acente için kayıtlı değişiklik yok.</div>}</div>)}
          </>)}
        </div>
      )}
    </div>
  );
}
