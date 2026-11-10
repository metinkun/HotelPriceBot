import React, { useEffect, useState } from "react";
import { Card, Badge, Button, Chip, Tabs, Alert, Skeleton, KV, EmptyState, Icon, useToast, DiffModal, ChangeRecordModal, CHANGE_KIND, pathLabel } from "../index.js";
import { fmtDate } from "../format.js";

/*
HotelDetailView: otelin acente bazli STATIK detayi (admin + portal ortak). Otel ozet ekranindan "Statik bilgiler" ile acilir.
  api, apiBase ("/api/admin" | "/api/portal"), hotelId
  labelOf(provider) -> gosterim adi; canRefresh; initialProvider (adres cubugundan)
  Degisen alanlar ilk goruntulemede sari vurgulanir; sari kutuya tiklaninca eski/yeni modal acilir.
  Sekme icerikleri kendi icinde kayar (sr-tabpane); baslik ve sekmeler sabit kalir.
*/
export function HotelDetailView({ api, apiBase, hotelId, labelOf = (n) => n, canRefresh = true, initialProvider = "", onBack, onOverview }) {
  const [ov, setOv] = useState(null);
  const [prov, setProv] = useState(initialProvider);
  const [view, setView] = useState(null);
  const [tab, setTab] = useState("ozet");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [diffSel, setDiffSel] = useState(null); // {path} -> modal
  const [rec, setRec] = useState(null);
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
  const lastDiff = Array.isArray(view?.lastChange?.diff) ? view.lastChange.diff : [];
  const hl = new Set(view?.highlightPaths || []);
  const hit = (path) => [...hl].some((p) => p === path || p.startsWith(path + "[") || p.startsWith(path + "."));
  // Vurgulu alana tiklaninca o path'e (veya alt path'lerine) ait degisiklik(ler)i modalda goster
  const openDiff = (path) => {
    const list = lastDiff.filter((d) => d.path === path || d.path.startsWith(path + "[") || d.path.startsWith(path + "."));
    if (list.length === 1) setDiffSel(list[0]);
    else if (list.length > 1) setRec({ ...view.lastChange, diff: list, summary: `${pathLabel(path)}: ${list.length} alan` });
  };
  const H = ({ path, children, block }) => {
    const on = hit(path);
    const props = on ? { className: "sr-hl clickable", onClick: (e) => { e.stopPropagation(); openDiff(path); }, title: "Değişikliği gör (eski / yeni)" } : {};
    return block ? <div {...props}>{children}</div> : <span {...props}>{children}</span>;
  };
  const HL = ({ path, children, as: Tag = "li" }) => (hl.has(path) ? <Tag className="sr-hl clickable" onClick={() => openDiff(path)} title="Değişikliği gör">{children}</Tag> : <Tag>{children}</Tag>);
  const changes = (ov.changes || []).filter((c) => c.provider === prov);
  const tabs = [
    ["ozet", "Genel", "info"], ["gorseller", "Görseller", "image", snap?.images?.length], ["ozellikler", "Özellikler", "sparkles", snap?.facilities?.length], ["odalar", "Odalar", "bed", snap?.rooms?.length],
    ["politika", "Politikalar", "shield"], ["sss", "Soru-Cevap", "help", snap?.faq?.length],
    ["puan", "Puanlar", "award", snap?.scores?.length || (snap?.rating != null ? 1 : 0)], ["yorum", "Yorumlar", "message", snap?.reviews?.length], ["etkinlik", "Etkinlikler", "music", snap?.events?.length], ["kampanya", "Kampanyalar", "megaphone", snap?.campaigns?.length],
    ["acente", "Acente görünümü", "globe", snap?.sections?.length], ["degisiklik", "Değişiklikler", "diff", changes.length],
  ].map(([value, label, icon, count]) => ({ value, label, icon, count: count || undefined }));
  const POL = { checkIn: "Giriş", checkOut: "Çıkış", childPolicy: "Çocuk", petPolicy: "Evcil hayvan", cancellation: "İptal", other: "Diğer" };
  const meta = { provider: prov, label: labelOf(prov), hotelName: ov.hotel.hotelName || hotelId, changedAt: view?.lastChange?.changedAt, version: view?.lastChange?.version };

  return (
    <div>
      <Card>
        <div className="sr-row between center">
          <div><h2>{ov.hotel.hotelName || hotelId}</h2><div className="muted small" style={{ marginTop: 4 }}><code>{hotelId}</code> · {ov.providers.length} acente · {ov.providers.filter((p) => p.fetched).length} detay çekildi · statik bilgiler</div></div>
          <div className="sr-row center" style={{ gap: 8 }}>{onOverview && <Button variant="secondary" icon="chevronLeft" onClick={onOverview}>Otel özeti</Button>}{!onOverview && onBack && <Button variant="secondary" icon="chevronLeft" onClick={onBack}>Geri</Button>}{canRefresh && <Button variant="secondary" icon="refresh" loading={busy} onClick={() => refresh("")}>Tüm acenteleri yenile</Button>}</div>
        </div>
        {err && <Alert tone="bad">{err}</Alert>}
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
            <div className="sr-row center" style={{ gap: 10 }}><h3>{labelOf(prov)}</h3>{view?.version ? <span className="muted small">sürüm {view.version} · {fmtDate(view.fetchedAt)}</span> : null}{view?.highlight && <span className="sr-hl-hint" role="button" onClick={() => setRec(view.lastChange)} style={{ cursor: "pointer" }}><Icon name="diff" size={13} />Değişti: {view.lastChange?.summary} · tümünü gör</span>}</div>
            <div className="sr-row center" style={{ gap: 6 }}>{snap?.sourceUrl && <a className="sr-btn ghost sm" href={snap.sourceUrl} target="_blank" rel="noreferrer">Acentede aç <Icon name="external" size={14} /></a>}{canRefresh && <Button size="sm" variant="secondary" icon="refresh" loading={busy} onClick={() => refresh(prov)}>Yenile</Button>}</div>
          </div>
          {view?.error && !snap?.name && <Alert tone="warn">{view.error}</Alert>}
          {!view && <div className="sr-stack"><Skeleton h={36} /><Skeleton h={200} /></div>}
          {snap && (<>
            <Tabs value={tab} onChange={setTab} items={tabs} />
            {view?.highlight && <p className="muted small" style={{ marginBottom: 10 }}>Sarı vurgulu alanlar son kontrolde değişti; üzerine tıklayarak eski ve yeni halini karşılaştırın.</p>}
            <div className="sr-tabpane">
            {tab === "ozet" && (<div className="sr-stack">
              <div><h2 style={{ fontSize: 20 }}><H path="name">{snap.name}</H> {snap.stars ? <H path="stars"><span style={{ color: "var(--warn)" }}>{"★".repeat(snap.stars)}</span></H> : null}</h2>
                <div className="muted" style={{ marginTop: 4 }}><H path="address">{snap.address}</H>{snap.city && <H path="city"> · {snap.city}</H>}{snap.rating != null && <H path="rating"> · Puan {snap.rating}{snap.ratingCount ? ` (${snap.ratingCount} yorum)` : ""}</H>}</div></div>
              <H path="description" block><p style={{ whiteSpace: "pre-wrap", lineHeight: 1.65, maxWidth: 900 }}>{snap.description || <span className="muted">Açıklama yok</span>}</p></H>
              <KV items={[["Pansiyon", snap.boards?.length ? <H path="boards">{snap.boards.join(", ")}</H> : null], ["İletişim", snap.contact ? <H path="contact">{[snap.contact.phone, snap.contact.email, snap.contact.website].filter(Boolean).join(" · ")}</H> : null], ["Konum", snap.location ? <a href={`https://maps.google.com/?q=${snap.location.lat},${snap.location.lng}`} target="_blank" rel="noreferrer">{snap.location.lat}, {snap.location.lng} · haritada aç</a> : null]]} />
            </div>)}
            {tab === "gorseller" && (<div className="sr-gallery">{(snap.images || []).map((im, i) => <a key={i} href={im.url} target="_blank" rel="noreferrer" className={hl.has(`images[${i}]`) ? "sr-hl" : ""}><img src={im.url} alt={im.caption || ""} loading="lazy" /></a>)}{!snap.images?.length && <EmptyState icon="image" title="Görsel yok" />}</div>)}
            {tab === "ozellikler" && (<div className="sr-grid cols-3">{(snap.facilities || []).map((g) => <Card key={g.title} flat title={g.title}><ul>{g.items.map((it, i) => <HL key={i} path={`facilities.${g.title}[${i}]`}>{it}</HL>)}</ul></Card>)}{!snap.facilities?.length && <EmptyState icon="sparkles" title="Özellik bilgisi yok" />}</div>)}
            {tab === "odalar" && (<div className="sr-stack">{(snap.rooms || []).map((r) => <Card key={r.name} flat className={[...hl].some((p) => p.startsWith("rooms." + r.name)) ? "sr-hl clickable" : ""} onClick={[...hl].some((p) => p.startsWith("rooms." + r.name)) ? () => openDiff("rooms." + r.name) : undefined}><h3>{r.name} {r.size ? <span className="muted">· {r.size} m²</span> : null} {r.capacity ? <span className="muted">· {r.capacity}</span> : null}</h3>{r.description && <p style={{ marginTop: 6 }}>{r.description}</p>}{r.features?.length > 0 && <div className="sr-chips" style={{ marginTop: 8 }}>{r.features.map((f, i) => <Chip key={i}>{f}</Chip>)}</div>}{r.images?.length > 0 && <div className="sr-gallery small" style={{ marginTop: 10 }}>{r.images.slice(0, 6).map((u, i) => <img key={i} src={u} loading="lazy" alt="" />)}</div>}</Card>)}{!snap.rooms?.length && <EmptyState icon="bed" title="Oda bilgisi yok" />}</div>)}
            {tab === "politika" && (<KV items={Object.entries(snap.policies || {}).filter(([, v]) => v && (!Array.isArray(v) || v.length)).map(([k, v]) => [POL[k] || k, <H path={"policies." + k}>{Array.isArray(v) ? v.join(" · ") : v}</H>])} />)}
            {tab === "sss" && (<div className="sr-stack">{(snap.faq || []).map((q, i) => <Card key={i} flat className={hl.has("faq." + q.question) ? "sr-hl clickable" : ""} onClick={hl.has("faq." + q.question) ? () => openDiff("faq." + q.question) : undefined}><b>{q.question}</b><p style={{ marginTop: 6, color: "var(--text-2)" }}>{q.answer}</p></Card>)}{!snap.faq?.length && <EmptyState icon="help" title="Soru-cevap yok" />}</div>)}
            {tab === "puan" && (<div className="sr-stack">
              {snap.rating != null && <div className="sr-score"><span className="sr-score-val"><H path="rating">{snap.rating}</H></span><span className="muted small">/ 10{snap.ratingCount ? <> · <H path="ratingCount">{snap.ratingCount} yorum</H></> : null}</span></div>}
              {(snap.scores || []).length > 0 && <Card flat title="Kategori puanları"><div className="sr-score-bars">{snap.scores.map((c, i) => <div key={i}><span className="truncate" title={c.key}>{c.key}</span><span className="bar"><i style={{ width: pct(c.value) + "%" }} /></span><b><H path={"scores." + c.key}>{c.value}</H></b></div>)}</div></Card>}
              {snap.rating == null && !(snap.scores || []).length && <EmptyState icon="award" title="Bu acente puan yayınlamıyor" />}
            </div>)}
            {tab === "yorum" && (<div className="sr-stack">{(snap.reviews || []).map((r, i) => <div key={i} className={"sr-review" + (hl.has("reviews[" + i + "]") || [...hl].some((p) => p === "reviews" ) ? " sr-hl" : "")}><div className="sr-review-head">{r.author && <b>{r.author}</b>}{r.date && <span>{r.date}</span>}{r.score != null && <Badge tone="ok" dot={false}>{r.score}</Badge>}</div>{r.title && <b style={{ display: "block", marginBottom: 4 }}>{r.title}</b>}<p>{r.text}</p></div>)}{!snap.reviews?.length && <EmptyState icon="message" title="Bu acentede yorum metni yok" />}</div>)}
            {tab === "etkinlik" && (<div className="sr-stack">{(snap.events || []).length ? <Card flat><ul>{snap.events.map((e, i) => <li key={i}><b>{e.title}</b>{e.date ? <span className="muted"> · {e.date}</span> : null}{e.text ? <div className="muted small">{e.text}</div> : null}</li>)}</ul></Card> : <EmptyState icon="music" title="Etkinlik bilgisi yok" />}</div>)}
            {tab === "kampanya" && (<div className="sr-stack">{(snap.campaigns || []).map((c, i) => <Card key={i} flat className={hit("campaigns." + c.title) ? "sr-hl clickable" : ""} onClick={hit("campaigns." + c.title) ? () => openDiff("campaigns." + c.title) : undefined}><b>{c.title}</b>{c.text && <p style={{ marginTop: 6, whiteSpace: "pre-wrap" }}>{c.text}</p>}</Card>)}{!snap.campaigns?.length && <EmptyState icon="megaphone" title="Kampanya bilgisi yok" />}</div>)}
            {tab === "acente" && (<div className="sr-stack">{(snap.sections || []).map((s, i) => <Card key={i} flat title={s.title} className={hl.has("sections." + s.title) ? "sr-hl clickable" : ""} onClick={hl.has("sections." + s.title) ? () => openDiff("sections." + s.title) : undefined}>{s.kind === "list" && <ul>{s.items.map((x, j) => <li key={j}>{x}</li>)}</ul>}{s.kind === "pairs" && <KV items={s.pairs.map((p) => [p.key, p.value])} />}{s.kind === "html" && <div dangerouslySetInnerHTML={{ __html: s.html }} />}{(s.kind === "text" || !s.kind) && <p style={{ whiteSpace: "pre-wrap" }}>{s.text}</p>}</Card>)}{!snap.sections?.length && <EmptyState icon="globe" title="Acente bölümleri yok" />}</div>)}
            {tab === "degisiklik" && (<div className="sr-stack">{changes.map((c) => <Card key={c.id} flat title={`${fmtDate(c.changedAt)} · sürüm ${c.version}`} subtitle={c.summary} actions={<Button size="sm" variant="secondary" icon="eye" onClick={() => setRec(c)}>Eski / yeni</Button>}>
              <div className="sr-table-scroll"><table className="sr-table dense"><thead><tr><th>Alan</th><th>Tür</th><th>Eski</th><th>Yeni</th></tr></thead><tbody>{(Array.isArray(c.diff) ? c.diff : []).slice(0, 40).map((d, i) => <tr key={i} className="clickable" onClick={() => setDiffSel({ ...d, _meta: c })} title="Eski / yeni için tıklayın"><td>{pathLabel(d.path)}</td><td><Badge tone={(CHANGE_KIND[d.kind] || [])[0] || "neutral"} dot={false}>{(CHANGE_KIND[d.kind] || [0, d.kind])[1]}</Badge></td><td className="muted truncate" style={{ maxWidth: 260 }}>{String(d.old ?? "")}</td><td className="truncate" style={{ maxWidth: 260 }}>{String(d.new ?? "")}</td></tr>)}</tbody></table></div>
              {(Array.isArray(c.diff) ? c.diff : []).length > 40 && <p className="muted small" style={{ marginTop: 6 }}>İlk 40 alan gösteriliyor; tamamı için "Eski / yeni".</p>}
            </Card>)}{changes.length === 0 && <EmptyState icon="diff" title="Bu acente için kayıtlı değişiklik yok" text="Günlük kontrol her gece çalışır; değişiklik bulunursa burada listelenir." />}</div>)}
            </div>
          </>)}
        </Card>
      )}
      <DiffModal change={diffSel} meta={diffSel?._meta ? { ...meta, changedAt: diffSel._meta.changedAt, version: diffSel._meta.version } : meta} onClose={() => setDiffSel(null)} />
      <ChangeRecordModal record={rec} meta={meta} onClose={() => setRec(null)} />
    </div>
  );
}

function pct(v) {
  const m = String(v || "").replace(",", ".").match(/(\d+(?:\.\d+)?)\s*(%|\/\s*(\d+))?/);
  if (!m) return 0;
  const n = parseFloat(m[1]);
  if (m[2] === "%") return Math.min(100, n);
  const base = m[3] ? parseFloat(m[3]) : n <= 5 ? 5 : n <= 10 ? 10 : 100;
  return Math.min(100, (n / base) * 100);
}
