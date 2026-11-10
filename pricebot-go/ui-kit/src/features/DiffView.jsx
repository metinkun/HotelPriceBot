import React, { useMemo, useState } from "react";
import { Modal, Badge, Button, DataTable, Icon } from "../index.js";
import { fmtDate } from "../format.js";

/*
Degisiklik gosterimi (admin + portal ortak).
  diffWords(a, b)      -> kelime bazli LCS farki: [{t:'same'|'del'|'ins', s}]
  DiffPair old/new     -> yan yana eski / yeni; degisen kelimeler renkli
  ChangeTable changes  -> alan listesi; satira tiklaninca DiffModal
  DiffModal change     -> tek alan icin tam eski/yeni
  CHANGE_KIND          -> etiketler
*/
export const CHANGE_KIND = { added: ["ok", "eklendi"], removed: ["bad", "silindi"], changed: ["warn", "değişti"] };
const PATH_LABEL = { name: "Ad", description: "Açıklama", address: "Adres", city: "Şehir", region: "Bölge", stars: "Yıldız", rating: "Puan", ratingCount: "Yorum sayısı", images: "Görsel", boards: "Pansiyon", facilities: "Özellik", rooms: "Oda", faq: "Soru-cevap", sections: "Bölüm", policies: "Politika", contact: "İletişim", reviews: "Yorum", scores: "Puan", events: "Etkinlik", campaigns: "Kampanya" };
export const pathLabel = (path = "") => {
  const top = path.split(/[.\[]/)[0];
  const rest = path.slice(top.length).replace(/^[.\[]/, "").replace(/\]$/, "");
  return (PATH_LABEL[top] || top) + (rest ? " · " + rest : "");
};
const str = (v) => (v == null ? "" : typeof v === "string" ? v : JSON.stringify(v));
const tokenize = (s) => str(s).split(/(\s+)/).filter((x) => x !== "");

// Kelime bazli LCS (kucuk/orta metinler icin yeterli; 2500 token ustunde satir bazina duser)
export function diffWords(a, b) {
  let A = tokenize(a), B = tokenize(b);
  if (A.length * B.length > 2500 * 2500) { A = str(a).split("\n"); B = str(b).split("\n"); }
  const n = A.length, m = B.length;
  const dp = new Array((n + 1) * (m + 1)).fill(0);
  for (let i = n - 1; i >= 0; i--) for (let j = m - 1; j >= 0; j--) dp[i * (m + 1) + j] = A[i] === B[j] ? dp[(i + 1) * (m + 1) + j + 1] + 1 : Math.max(dp[(i + 1) * (m + 1) + j], dp[i * (m + 1) + j + 1]);
  const out = []; let i = 0, j = 0;
  const push = (t, s) => { const last = out[out.length - 1]; if (last && last.t === t) last.s += s; else out.push({ t, s }); };
  while (i < n && j < m) { if (A[i] === B[j]) { push("same", A[i]); i++; j++; } else if (dp[(i + 1) * (m + 1) + j] >= dp[i * (m + 1) + j + 1]) { push("del", A[i]); i++; } else { push("ins", B[j]); j++; } }
  while (i < n) push("del", A[i++]); while (j < m) push("ins", B[j++]);
  return out;
}

export function DiffPair({ oldValue, newValue, kind }) {
  const parts = useMemo(() => (kind === "changed" ? diffWords(oldValue, newValue) : null), [oldValue, newValue, kind]);
  const render = (side) => {
    if (!parts) return <span>{side === "old" ? str(oldValue) : str(newValue)}</span>;
    return parts.filter((p) => p.t === "same" || (side === "old" ? p.t === "del" : p.t === "ins")).map((p, i) => <span key={i} className={p.t === "same" ? "" : "sr-diff-" + p.t}>{p.s}</span>);
  };
  return (
    <div className="sr-diff-pair">
      <div className="sr-diff-col old"><div className="sr-diff-head"><Badge tone="bad" dot={false}>Eski</Badge></div><pre>{kind === "added" ? <span className="muted">(yoktu)</span> : render("old")}</pre></div>
      <div className="sr-diff-col new"><div className="sr-diff-head"><Badge tone="ok" dot={false}>Yeni</Badge></div><pre>{kind === "removed" ? <span className="muted">(kaldırıldı)</span> : render("new")}</pre></div>
    </div>
  );
}

// Tek bir alan degisikligi icin modal. change: {path, kind, old, new}; meta: {provider, label, changedAt, version, hotelName}
export function DiffModal({ change, meta, onClose, footer }) {
  if (!change) return null;
  const [tone, label] = CHANGE_KIND[change.kind] || ["neutral", change.kind];
  return (
    <Modal open onClose={onClose} size="lg" title={<span className="sr-row center" style={{ gap: 8 }}>{pathLabel(change.path)} <Badge tone={tone} dot={false}>{label}</Badge></span>}
      subtitle={[meta?.label || meta?.provider, meta?.hotelName, meta?.changedAt ? fmtDate(meta.changedAt) : null, meta?.version ? `sürüm ${meta.version}` : null].filter(Boolean).join(" · ")}
      footer={<>{footer}<Button variant="secondary" onClick={onClose}>Kapat</Button></>}>
      <DiffPair oldValue={change.old} newValue={change.new} kind={change.kind} />
      <p className="muted small" style={{ marginTop: 10 }}><span className="sr-diff-del">kırmızı</span> çıkarılan, <span className="sr-diff-ins">yeşil</span> eklenen metin.</p>
    </Modal>
  );
}

// Bir degisiklik kaydinin alan listesi; tiklaninca eski/yeni modal.
export function ChangeTable({ changes = [], meta, dense = true, pageSize = 50, footerFor }) {
  const [sel, setSel] = useState(null);
  const rows = changes.map((c, i) => ({ ...c, _i: i }));
  return (
    <>
      <DataTable rows={rows} rowKey={(r) => r._i} dense={dense} pageSize={pageSize} onRowDoubleClick={setSel} onRowClick={setSel}
        columns={[
          { key: "path", title: "Alan", primary: true, render: (d) => pathLabel(d.path), sortable: true, sortValue: (d) => d.path },
          { key: "kind", title: "Tür", width: 100, render: (d) => { const [t, l] = CHANGE_KIND[d.kind] || ["neutral", d.kind]; return <Badge tone={t} dot={false}>{l}</Badge>; }, filterValue: (d) => (CHANGE_KIND[d.kind] || [0, d.kind])[1] },
          { key: "old", title: "Eski", render: (d) => <span className="muted truncate" style={{ maxWidth: 280, display: "inline-block" }}>{str(d.old)}</span>, hideOnMobile: true },
          { key: "new", title: "Yeni", render: (d) => <span className="truncate" style={{ maxWidth: 280, display: "inline-block" }}>{str(d.new)}</span> },
        ]}
        actions={(d) => [{ icon: "eye", title: "Eski / yeni", onClick: setSel }]} empty={{ icon: "diff", title: "Alan değişikliği yok" }} />
      <DiffModal change={sel} meta={meta} onClose={() => setSel(null)} footer={footerFor ? footerFor(sel) : null} />
    </>
  );
}

// Bir degisiklik kaydinin tamami (ozet + alan listesi) modal icinde. change: {provider, version, changedAt, summary, diff}
export function ChangeRecordModal({ record, meta, onClose, footer }) {
  if (!record) return null;
  const diff = Array.isArray(record.diff) ? record.diff : (() => { try { return JSON.parse(record.diff || "[]"); } catch { return []; } })();
  return (
    <Modal open onClose={onClose} size="lg" title={<span className="sr-row center" style={{ gap: 8 }}><Icon name="diff" size={16} />{meta?.label || record.provider} · sürüm {record.version}</span>}
      subtitle={`${fmtDate(record.changedAt)} · ${record.summary || ""}${meta?.hotelName ? " · " + meta.hotelName : ""}`}
      footer={<>{footer}<Button variant="secondary" onClick={onClose}>Kapat</Button></>}>
      <ChangeTable changes={diff} meta={{ ...meta, provider: record.provider, changedAt: record.changedAt, version: record.version }} pageSize={0} />
    </Modal>
  );
}
