import React, { useEffect, useMemo, useState } from "react";
import { Card, Stat, Badge, Button, Tabs, Alert, Skeleton, KV, EmptyState, DataTable, Icon, ChangeRecordModal } from "../index.js";
import { fmtDate, fmtRel } from "../format.js";

/*
HotelOverviewView: "bizim" otel detay ekrani (admin + portal ortak).
  api, apiBase, hotelId, onBack, onOpenStatic(provider?) -> acentenin statik bilgileri (mevcut ekran)
  onOpenClient(id) (admin), onEditMeta() (admin)
  Sekmeler: Siteler, Puanlar, Yorumlar, Etkinlikler, Kampanyalar, Degisiklikler (+ Musteriler admin). Her sekme kendi icinde kayar.
*/
export function HotelOverviewView({ api, apiBase, hotelId, onBack, onOpenStatic, onOpenClient, onEditMeta, onRunSearch }) {
  const [d, setD] = useState(null);
  const [err, setErr] = useState("");
  const [tab, setTab] = useState("sites");
  const [rec, setRec] = useState(null);
  const load = () => api(`${apiBase}/hotels/${hotelId}/overview`).then(setD).catch((e) => setErr(e.message));
  useEffect(() => { load(); }, [hotelId]);
  if (err) return <Alert tone="bad">{err}</Alert>;
  if (!d) return <div className="sr-stack"><Skeleton h={28} w="40%" /><Skeleton h={90} /><Skeleton h={300} /></div>;
  const h = d.hotel, m = h.meta || {};
  const name = m.name || h.hotelName || hotelId;
  const provs = d.providers || [];
  const fetched = provs.filter((p) => p.fetched);
  const ratings = provs.filter((p) => p.rating != null);
  const avg = ratings.length ? ratings.reduce((a, p) => a + p.rating, 0) / ratings.length : null;
  const stars = m.stars || (provs.find((p) => p.stars)?.stars);
  const loc = [m.region, m.city, m.country].filter(Boolean).join(" · ") || provs.find((p) => p.city)?.city;
  const unseen = provs.filter((p) => p.unseenChange).length;
  const tabs = [
    { value: "sites", label: "Siteler", icon: "globe", count: provs.length },
    { value: "scores", label: "Puanlar", icon: "award", count: (d.scores || []).length || undefined },
    { value: "reviews", label: "Yorumlar", icon: "message", count: (d.reviews || []).length || undefined },
    { value: "events", label: "Etkinlikler", icon: "music", count: (d.events || []).length || undefined },
    { value: "campaigns", label: "Kampanyalar", icon: "megaphone", count: (d.campaigns || []).length || undefined },
    { value: "changes", label: "Değişiklikler", icon: "diff", count: (d.changes || []).length || undefined },
    ...(d.clients ? [{ value: "clients", label: "Müşteriler", icon: "users", count: d.clients.length }] : []),
  ];
  const labelOf = (p) => (d.labels || {})[p] || p;
  const openStatic = (p) => onOpenStatic && onOpenStatic(p || "");

  return (
    <div>
      <Card>
        <div className="sr-overview-head">
          <div style={{ minWidth: 0 }}>
            <h2>{name} {stars ? <span style={{ color: "var(--warn)", fontSize: 16 }}>{"★".repeat(stars)}</span> : null}</h2>
            <div className="muted" style={{ marginTop: 4 }}>{loc || "Konum bilgisi yok"}{m.chain ? ` · ${m.chain}` : ""}</div>
            <div className="sr-overview-badges">
              <Badge tone="neutral" dot={false}><code style={{ background: "transparent", padding: 0 }}>{hotelId}</code></Badge>
              {m.externalId && <Badge tone="neutral" dot={false}>dış kimlik {m.externalId}</Badge>}
              {m.isActive === false ? <Badge tone="bad">pasif</Badge> : <Badge tone="ok">aktif</Badge>}
              {unseen > 0 && <Badge tone="warn">{unseen} sitede yeni değişiklik</Badge>}
            </div>
          </div>
          <div className="sr-row center" style={{ gap: 8 }}>
            {onBack && <Button variant="secondary" icon="chevronLeft" onClick={onBack}>Geri</Button>}
            {onEditMeta && <Button variant="secondary" icon="pencil" onClick={onEditMeta}>Künye</Button>}
            {onRunSearch && <Button variant="secondary" icon="search" onClick={onRunSearch}>Fiyat ara</Button>}
            <Button icon="file" onClick={() => openStatic("")}>Statik bilgiler</Button>
          </div>
        </div>
        {m.notes && <p className="muted small" style={{ marginTop: 10, whiteSpace: "pre-wrap" }}>{m.notes}</p>}
      </Card>
      <div className="sr-grid" style={{ marginTop: 16 }}>
        <Stat icon="globe" label="Eşleşen site" value={provs.length} sub={`${provs.filter((p) => p.isActive).length} aktif eşleştirme`} />
        <Stat icon="file" label="Statik detay" value={fetched.length} unit={`/ ${provs.filter((p) => p.supported).length}`} sub="detay çekilen site" soft />
        <Stat icon="award" label="Ortalama puan" value={avg != null ? avg.toFixed(1) : "—"} unit={avg != null ? "/ 10" : ""} sub={ratings.length ? `${ratings.length} site puanlıyor` : "puan yok"} soft />
        <Stat icon="message" label="Yorum" value={(d.reviews || []).length} sub={`${(d.events || []).length} etkinlik · ${(d.campaigns || []).length} kampanya`} soft />
      </div>
      <Card style={{ marginTop: 16 }} pad0>
        <div style={{ padding: "6px 20px 0" }}><Tabs value={tab} onChange={setTab} items={tabs} /></div>
        <div style={{ padding: "0 20px 20px" }}>
          {tab === "sites" && (
            <DataTable rows={provs} rowKey={(p) => p.provider} dense pageSize={0} onRowDoubleClick={(p) => openStatic(p.provider)}
              columns={[
                { key: "label", title: "Site", primary: true, render: (p) => <span className="sr-row center" style={{ gap: 6 }}>{p.label}{p.unseenChange && <span className="sr-dot" style={{ width: 7, height: 7, borderRadius: 4, background: "var(--warn)", display: "inline-block" }} title="Görülmemiş değişiklik" />}</span>, sub: (p) => <code>{p.providerHotelId}</code>, sortable: true, sortValue: (p) => p.label },
                { key: "rating", title: "Puan", align: "num", render: (p) => (p.rating != null ? <b>{p.rating}</b> : <span className="muted">—</span>), sortable: true, sortValue: (p) => p.rating || 0, sub: (p) => (p.ratingCount ? `${p.ratingCount} yorum` : "") },
                { key: "content", title: "İçerik", hideOnMobile: true, render: (p) => (p.fetched ? <span className="muted small">{p.images} görsel · {p.rooms} oda · {p.reviews} yorum · {p.events} etkinlik · {p.campaigns} kampanya</span> : <span className="muted">—</span>) },
                { key: "detail", title: "Statik detay", render: (p) => (!p.supported ? <Badge tone="neutral" dot={false}>desteklenmiyor</Badge> : p.fetched ? <Badge tone={p.unseenChange ? "warn" : "ok"} dot={false}>sürüm {p.version}</Badge> : p.error ? <Badge tone="bad" title={p.error}>hata</Badge> : <Badge tone="neutral">çekilmedi</Badge>), filterValue: (p) => (!p.supported ? "desteklenmiyor" : p.fetched ? "çekildi" : p.error ? "hata" : "çekilmedi") },
                { key: "changedAt", title: "Son değişiklik", hideOnMobile: true, render: (p) => (p.changedAt ? <span title={fmtDate(p.changedAt)}>{fmtRel(p.changedAt)}</span> : <span className="muted">—</span>), sortable: true, sortValue: (p) => p.changedAt || "" },
                { key: "isActive", title: "Eşleştirme", render: (p) => (p.isActive ? <Badge tone="ok">aktif</Badge> : <Badge tone="neutral">pasif</Badge>) },
              ]}
              actions={(p) => [{ icon: "file", title: "Statik bilgiler", onClick: (x) => openStatic(x.provider) }, { icon: "external", title: "Sitede aç", hidden: !p.sourceUrl, onClick: (x) => window.open(x.sourceUrl, "_blank", "noreferrer") }]}
              empty={{ icon: "globe", title: "Bu otel hiçbir siteyle eşleşmemiş" }} />
          )}
          {tab === "scores" && (<div className="sr-tabpane">
            {(d.scores || []).length === 0 ? <EmptyState icon="award" title="Puan bilgisi yok" text="Siteler puan yayınlamıyor ya da statik detay henüz çekilmedi." /> :
              <div className="sr-grid cols-3">{d.scores.map((s) => { const cats = s.categories.map((c) => ({ ...c, v: scoreNum(c.value) })); return (
                <Card key={s.provider} flat title={s.label} subtitle={s.ratingCount ? `${s.ratingCount} yorum` : undefined}>
                  <div className="sr-score"><span className="sr-score-val">{s.rating != null ? s.rating : "—"}</span><span className="muted small">/ 10{s.stars ? ` · ${"★".repeat(s.stars)}` : ""}</span></div>
                  {cats.length > 0 && <div className="sr-score-bars">{cats.map((c, i) => <div key={i}><span className="truncate" title={c.key}>{c.key}</span><span className="bar"><i style={{ width: (c.v != null ? Math.min(100, c.v.pct) : 0) + "%" }} /></span><b>{c.value}</b></div>)}</div>}
                </Card>); })}</div>}
          </div>)}
          {tab === "reviews" && (<div className="sr-tabpane">
            {(d.reviews || []).length === 0 ? <EmptyState icon="message" title="Yorum yok" text="Siteler yorum metni yayınladığında burada listelenir; yeni yorumlar bildirim üretir." /> :
              <div className="sr-stack">{d.reviews.map((r, i) => <div key={i} className="sr-review"><div className="sr-review-head"><Badge tone="info" dot={false}>{r.label}</Badge>{r.author && <b>{r.author}</b>}{r.date && <span>{r.date}</span>}{r.score != null && <Badge tone="ok" dot={false}>{r.score}</Badge>}</div>{r.title && <b style={{ display: "block", marginBottom: 4 }}>{r.title}</b>}<p>{r.text}</p></div>)}</div>}
          </div>)}
          {tab === "events" && (<div className="sr-tabpane">
            {(d.events || []).length === 0 ? <EmptyState icon="music" title="Etkinlik yok" text="Sanatçı programı, animasyon takvimi gibi bilgiler siteler yayınladıkça burada görünür." /> :
              <DataTable rows={d.events} rowKey={(e, i) => i} dense pageSize={0} columns={[{ key: "label", title: "Site", width: 140, render: (e) => <Badge tone="info" dot={false}>{e.label}</Badge> }, { key: "title", title: "Etkinlik", primary: true, sub: (e) => e.text || "" }, { key: "date", title: "Tarih", width: 140, render: (e) => e.date || <span className="muted">—</span> }]} />}
          </div>)}
          {tab === "campaigns" && (<div className="sr-tabpane">
            {(d.campaigns || []).length === 0 ? <EmptyState icon="megaphone" title="Kampanya yok" text="Siteler kampanya duyurduğunda burada listelenir ve bildirim üretilir." /> :
              <DataTable rows={d.campaigns} rowKey={(c, i) => i} dense pageSize={0} columns={[{ key: "label", title: "Site", width: 140, render: (c) => <Badge tone="info" dot={false}>{c.label}</Badge> }, { key: "title", title: "Kampanya", primary: true }, { key: "text", title: "Detay", render: (c) => <span style={{ whiteSpace: "pre-wrap" }}>{c.text || "—"}</span> }]} />}
          </div>)}
          {tab === "changes" && (<div className="sr-tabpane">
            <DataTable rows={d.changes || []} dense pageSize={20} onRowDoubleClick={setRec}
              columns={[
                { key: "changedAt", title: "Zaman", width: 160, render: (c) => fmtDate(c.changedAt), sortable: true, sortValue: (c) => c.changedAt },
                { key: "provider", title: "Site", width: 150, render: (c) => <Badge tone="info" dot={false}>{labelOf(c.provider)}</Badge>, filterValue: (c) => labelOf(c.provider) },
                { key: "summary", title: "Özet", primary: true, sub: (c) => `sürüm ${c.version} · ${(Array.isArray(c.diff) ? c.diff : []).length} alan` },
              ]}
              actions={(c) => [{ icon: "eye", title: "Eski / yeni", onClick: setRec }, { icon: "file", title: "Statik bilgiler", onClick: (x) => openStatic(x.provider) }]}
              empty={{ icon: "diff", title: "Değişiklik kaydı yok", text: "Günlük kontrol değişiklik bulduğunda burada listelenir." }} />
          </div>)}
          {tab === "clients" && d.clients && (
            <DataTable rows={d.clients} dense pageSize={0} onRowDoubleClick={(c) => onOpenClient && onOpenClient(c.id)}
              columns={[{ key: "name", title: "Müşteri", primary: true, sub: (c) => c.email || "" }, { key: "kind", title: "Tür", render: (c) => <Badge tone={c.kind === "otel" ? "info" : "brand"} dot={false}>{c.kind === "otel" ? "OTEL" : "ACENTE"}</Badge> }, { key: "planCode", title: "Paket", render: (c) => c.planCode ? <code>{c.planCode}</code> : <span className="muted">—</span> }, { key: "isActive", title: "Durum", render: (c) => (c.isActive ? <Badge tone="ok">aktif</Badge> : <Badge tone="bad">pasif</Badge>) }]}
              actions={(c) => [{ icon: "eye", title: "Müşteri detayı", onClick: (x) => onOpenClient && onOpenClient(x.id) }]} empty={{ icon: "users", title: "Bu oteli kullanan müşteri yok" }} />
          )}
        </div>
      </Card>
      <ChangeRecordModal record={rec} meta={{ label: rec ? labelOf(rec.provider) : "", hotelName: name }} onClose={() => setRec(null)} footer={rec && <Button variant="secondary" icon="file" onClick={() => { const p = rec.provider; setRec(null); openStatic(p); }}>Statik bilgilere git</Button>} />
    </div>
  );
}

// "8,6" / "4.5" / "92%" -> {val, pct(0-100)}
function scoreNum(v) {
  const m = String(v || "").replace(",", ".").match(/(\d+(?:\.\d+)?)\s*(%|\/\s*(\d+))?/);
  if (!m) return null;
  const n = parseFloat(m[1]);
  if (m[2] === "%") return { val: n, pct: n };
  const base = m[3] ? parseFloat(m[3]) : n <= 5 ? 5 : n <= 10 ? 10 : 100;
  return { val: n, pct: (n / base) * 100 };
}
