import React, { useEffect, useState } from "react";
import { Card, Badge, Button, Chip, Tabs, Alert, Skeleton, KV, EmptyState, Icon, useToast } from "../index.js";
import { fmtDate } from "../format.js";

/*
HotelDetailView: otelin acente bazli statik detayi (admin + portal ortak).
  api, apiBase ("/api/admin" | "/api/portal"), hotelId
  labelOf(provider) -> gosterim adi; canRefresh; initialProvider (adres cubugundan)
  Degisen alanlar ilk goruntulemede sari vurgulanir (sunucu highlightPaths doner ve goruldu isaretler).
*/
export function HotelDetailView({ api, apiBase, hotelId, labelOf = (n) => n, canRefresh = true, initialProvider = "", onBack }) {
  const [ov, setOv] = useState(null);
  const [prov, setProv] = useState(initialProvider);
  const [view, setView] = useState(null);
  const [tab, setTab] = useState("ozet");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const toast = useToast();

  const loadOv = async () => {
    try { const d = await api(`${apiBase}/hotels/${hotelId}/details`); setOv(d); setErr(""); const first = d.providers.find((p) => p.fetched) || d.providers.find((p) => p.supported); setProv((cur) => cur || (first ? first.provider : "")); }
    catch (e) { setErr(e.message); }
  };
  useEffect(() => { loadOv(); }, [hotelId]);
  useEffect(() => { if (!prov) return; setView(null); api(`${apiBase}/hotels/${hotelId}/details?provider=${prov}`).then(setView).catch((e) => setView({ error: e.message })); }, [prov, hotelId]);

  const refresh = async (p) => {
    setBusy(true);
    try { const r = await api(`${apiBase}/hotels/${hotelId}/details/refresh${p ? "?provider=" + p : ""}`, { method: "POST" }); await loadOv(); if (p === prov || !p) setView(await api(`${apiBase}/hotels/${hotelId}/details?provider=${prov}`)); toast.success(p ? (r.changed ? "Yenilendi: değişiklik bulundu." : "Yenilendi: değişiklik yok.") : `${r.refreshed} acente yenilendi.`); }
    catch (e) { toast.error(e.message); } finally { setBusy(false); }
  };

  if (!ov) return err ? <Alert tone="bad">{err}</Alert> : <div className="sr-stack"><Skeleton h={28} w="40%" /><Skeleton h={60} /><Skeleton h={300} /></div>;
  const snap = view?.snapshot;
  const hl = new Set(view?.highlightPaths || []);
  const hit = (path) => [...hl].some((p) => p === path || p.startsWith(path + "[") || p.startsWith(path + "."));
  const H = ({ path, children, block }) => (block ? <div className={hit(path) ? "sr-hl" : ""}>{children}</div> : <span className={hit(path) ? "sr-hl" : ""}>{children}</span>);
  const changes = (ov.changes || []).filter((c) => c.provider === prov);
  const tabs = [
    ["ozet", "Genel", "info"], ["gorseller", "Görseller", "image", snap?.images?.length], ["ozellikler", "Özellikler", "sparkles", snap?.facilities?.length], ["odalar", "Odalar", "bed", snap?.rooms?.length],
    ["politika", "Politikalar", "shield"], ["sss", "Soru-Cevap", "help", snap?.faq?.length], ["acente", "Acente görünümü", "globe", snap?.sections?.length], ["degisiklik", "Değişiklikler", "diff", changes.length],
  ].map(([value, label, icon, count]) => ({ value, label, icon, count: count || undefined }));
  const POL = { checkIn: "Giriş", checkOut: "Çıkış", childPolicy: "Çocuk", petPolicy: "Evcil hayvan", cancellation: "İptal", other: "Diğer" };

  return (
    <div>
      <Card>
        <div className="sr-row between center">
          <div><h2>{ov.hotel.hotelName || hotelId}</h2><div className="muted small" style={{ marginTop: 4 }}><code>{hotelId}</code> · {ov.providers.length} acente · {ov.providers.filter((p) => p.fetched).length} detay çekildi</div></div>
          <div className="sr-row center" style={{ gap: 8 }}>{onBack && <Button variant="secondary" icon="chevronLeft" onClick={onBack}>Geri</Button>}{canRefresh && <Button variant="secondary" icon="refresh" loading={busy} onClick={() => refresh("")}>Tüm acenteleri yenile</Button>}</div>
        </div>
        {err && <Alert tone="bad" className="" >{err}</Alert>}
        <div className="sr-chips" style={{ marginTop: 14 }}>
          {ov.providers.map((p) => (
            <Chip key={p.provider} on={p.provider === prov} off={!p.supported} dot={p.unseenChange} onClick={p.supported ? () => setProv(p.provider) : undefined}
              title={p.supported ? (p.fetched ? `sürüm ${p.version} · ${fmtDate(p.fetchedAt)}` : "henüz çekilmedi") : "bu acente için detay desteği yok"}>
              {labelOf(p.provider)}{!p.fetched && p.supported && <small className="muted">yok</small>}{p.error && !p.fetched && <Icon name="alert" size={13} style={{ color: "var(--warn)" }} />}
            </Chip>))}
        </div>
      </Card>

      {prov && (
        <Card style={{ marginTop: 16 }}>
          <div className="sr-row between center" style={{ marginBottom: 12 }}>
            <div className="sr-row center" style={{ gap: 10 }}><h3>{labelOf(prov)}</h3>{view?.version ? <span className="muted small">sürüm {view.version} · {fmtDate(view.fetchedAt)}</span> : null}{view?.highlight && <Badge tone="warn">Değişti: {view.lastChange?.summary}</Badge>}</div>
            <div className="sr-row center" style={{ gap: 6 }}>{snap?.sourceUrl && <a className="sr-btn ghost sm" href={snap.sourceUrl} target="_blank" rel="noreferrer">Acentede aç <Icon name="external" size={14} /></a>}{canRefresh && <Button size="sm" variant="secondary" icon="refresh" loading={busy} onClick={() => refresh(prov)}>Yenile</Button>}</div>
          </div>
          {view?.error && !snap?.name && <Alert tone="warn">{view.error}</Alert>}
          {!view && <div className="sr-stack"><Skeleton h={36} /><Skeleton h={200} /></div>}
          {snap && (<>
            <Tabs value={tab} onChange={setTab} items={tabs} />
            {tab === "ozet" && (<div className="sr-stack">
              <div><h2 style={{ fontSize: 20 }}><H path="name">{snap.name}</H> {snap.stars ? <H path="stars"><span style={{ color: "var(--warn)" }}>{"★".repeat(snap.stars)}</span></H> : null}</h2>
                <div className="muted" style={{ marginTop: 4 }}><H path="address">{snap.address}</H>{snap.city && <H path="city"> · {snap.city}</H>}{snap.rating != null && <H path="rating"> · Puan {snap.rating}{snap.ratingCount ? ` (${snap.ratingCount} yorum)` : ""}</H>}</div></div>
              <H path="description" block><p style={{ whiteSpace: "pre-wrap", lineHeight: 1.65, maxWidth: 900 }}>{snap.description || <span className="muted">Açıklama yok</span>}</p></H>
              <KV items={[["Pansiyon", snap.boards?.length ? <H path="boards">{snap.boards.join(", ")}</H> : null], ["İletişim", snap.contact ? <H path="contact">{[snap.contact.phone, snap.contact.email, snap.contact.website].filter(Boolean).join(" · ")}</H> : null], ["Konum", snap.location ? <a href={`https://maps.google.com/?q=${snap.location.lat},${snap.location.lng}`} target="_blank" rel="noreferrer">{snap.location.lat}, {snap.location.lng} · haritada aç</a> : null]]} />
            </div>)}
            {tab === "gorseller" && (<div className="sr-gallery">{(snap.images || []).map((im, i) => <a key={i} href={im.url} target="_blank" rel="noreferrer" className={hl.has(`images[${i}]`) ? "sr-hl" : ""}><img src={im.url} alt={im.caption || ""} loading="lazy" /></a>)}{!snap.images?.length && <EmptyState icon="image" title="Görsel yok" />}</div>)}
            {tab === "ozellikler" && (<div className="sr-grid cols-3">{(snap.facilities || []).map((g) => <Card key={g.title} flat title={g.title}><ul>{g.items.map((it, i) => <li key={i} className={hl.has(`facilities.${g.title}[${i}]`) ? "sr-hl" : ""}>{it}</li>)}</ul></Card>)}{!snap.facilities?.length && <EmptyState icon="sparkles" title="Özellik bilgisi yok" />}</div>)}
            {tab === "odalar" && (<div className="sr-stack">{(snap.rooms || []).map((r) => <Card key={r.name} flat className={[...hl].some((p) => p.startsWith("rooms." + r.name)) ? "sr-hl" : ""}><h3>{r.name} {r.size ? <span className="muted">· {r.size} m²</span> : null} {r.capacity ? <span className="muted">· {r.capacity}</span> : null}</h3>{r.description && <p style={{ marginTop: 6 }}>{r.description}</p>}{r.features?.length > 0 && <div className="sr-chips" style={{ marginTop: 8 }}>{r.features.map((f, i) => <Chip key={i}>{f}</Chip>)}</div>}{r.images?.length > 0 && <div className="sr-gallery small" style={{ marginTop: 10 }}>{r.images.slice(0, 6).map((u, i) => <img key={i} src={u} loading="lazy" alt="" />)}</div>}</Card>)}{!snap.rooms?.length && <EmptyState icon="bed" title="Oda bilgisi yok" />}</div>)}
            {tab === "politika" && (<KV items={Object.entries(snap.policies || {}).filter(([, v]) => v && (!Array.isArray(v) || v.length)).map(([k, v]) => [POL[k] || k, <span className={hit("policies." + k) ? "sr-hl" : ""}>{Array.isArray(v) ? v.join(" · ") : v}</span>])} />)}
            {tab === "sss" && (<div className="sr-stack">{(snap.faq || []).map((q, i) => <Card key={i} flat className={hl.has("faq." + q.question) ? "sr-hl" : ""}><b>{q.question}</b><p style={{ marginTop: 6, color: "var(--text-2)" }}>{q.answer}</p></Card>)}{!snap.faq?.length && <EmptyState icon="help" title="Soru-cevap yok" />}</div>)}
            {tab === "acente" && (<div className="sr-stack">{(snap.sections || []).map((s, i) => <Card key={i} flat title={s.title} className={hl.has("sections." + s.title) ? "sr-hl" : ""}>{s.kind === "list" && <ul>{s.items.map((x, j) => <li key={j}>{x}</li>)}</ul>}{s.kind === "pairs" && <KV items={s.pairs.map((p) => [p.key, p.value])} />}{s.kind === "html" && <div dangerouslySetInnerHTML={{ __html: s.html }} />}{(s.kind === "text" || !s.kind) && <p style={{ whiteSpace: "pre-wrap" }}>{s.text}</p>}</Card>)}{!snap.sections?.length && <EmptyState icon="globe" title="Acente bölümleri yok" />}</div>)}
            {tab === "degisiklik" && (<div className="sr-stack">{changes.map((c) => <Card key={c.id} flat title={`${fmtDate(c.changedAt)} · sürüm ${c.version}`} subtitle={c.summary}>
              <div className="sr-table-scroll"><table className="sr-table dense"><thead><tr><th>Alan</th><th>Tür</th><th>Eski</th><th>Yeni</th></tr></thead><tbody>{(c.diff || []).slice(0, 80).map((d, i) => <tr key={i}><td><code>{d.path}</code></td><td><Badge tone={d.kind === "added" ? "ok" : d.kind === "removed" ? "bad" : "warn"} dot={false}>{{ added: "eklendi", removed: "silindi", changed: "değişti" }[d.kind] || d.kind}</Badge></td><td className="muted truncate" style={{ maxWidth: 260 }}>{String(d.old ?? "")}</td><td className="truncate" style={{ maxWidth: 260 }}>{String(d.new ?? "")}</td></tr>)}</tbody></table></div>
            </Card>)}{changes.length === 0 && <EmptyState icon="diff" title="Bu acente için kayıtlı değişiklik yok" text="Günlük kontrol her gece çalışır; değişiklik bulunursa burada listelenir." />}</div>)}
          </>)}
        </Card>
      )}
    </div>
  );
}
