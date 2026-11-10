import React, { useEffect, useState } from "react";
import { api, fmtDate } from "../api.js";
import { PageHeader, Card, DataTable, Button, Badge, Alert, KV, ChangeRecordModal, useConfirm, useToast } from "@kit";

export default function DetailChanges({ navigate }) {
  const [d, setD] = useState(null);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");
  const [rec, setRec] = useState(null);
  const [hotels, setHotels] = useState({});
  const confirm = useConfirm(); const toast = useToast();
  const load = () => api("/api/admin/details/changes?limit=300").then(setD).catch((e) => setErr(e.message)).finally(() => setLoading(false));
  useEffect(() => { load(); api("/api/admin/hotels").then((l) => setHotels(Object.fromEntries(l.map((h) => [h.internalHotelId, h.hotelName || h.internalHotelId])))).catch(() => {}); }, []);
  const runAll = async () => {
    if (!(await confirm({ title: "Tüm oteller şimdi kontrol edilsin mi?", message: "13 acente × tüm oteller yeniden çekilir; 3-4 dakika sürer ve tarayıcı belleğini yoğun kullanır. Normalde her gece 04:00'te otomatik çalışır.", confirmText: "Şimdi çalıştır" }))) return;
    try { const r = await api("/api/admin/details/run", { method: "POST" }); toast.success(r.message || "Kontrol başlatıldı."); } catch (e) { toast.error(e.message); }
  };
  const openStatic = (c) => { location.hash = `hotelstatic/${c.internalHotelId}/${c.provider || ""}`; };
  const hotelName = (id) => hotels[id] || id;
  const diffLen = (c) => (Array.isArray(c.diff) ? c.diff.length : 0);
  const lr = d?.lastRun;
  return (
    <div>
      <PageHeader title="Otel detay değişiklikleri" description="Günlük kontrolde acentelerde tespit edilen otel bilgisi değişiklikleri. Çift tıklayınca eski ve yeni hali karşılaştırmalı açılır." actions={<Button icon="play" variant="secondary" onClick={runAll}>Tüm otelleri şimdi kontrol et</Button>} />
      {err && <Alert tone="bad">{err}</Alert>}
      {d && (
        <Card title="Son kontrol" subtitle={lr ? "Günlük kontrol her gece 04:00'te çalışır." : "Henüz kontrol çalışmadı; her gece 04:00'te otomatik başlar."}>
          {lr && <KV items={[["Başlangıç → bitiş", `${fmtDate(lr.startedAt)} → ${fmtDate(lr.finishedAt)}`], ["Kayıt", `${lr.total} eşleştirme`], ["Sonuç", <span className="sr-row center" style={{ gap: 6 }}><Badge tone="ok" dot={false}>{lr.unchanged ?? lr.total - lr.changed - lr.failed} değişmedi</Badge><Badge tone="warn" dot={false}>{lr.changed} değişti</Badge><Badge tone={lr.failed ? "bad" : "neutral"} dot={false}>{lr.failed} hata</Badge></span>]]} />}
          <div className="sr-row center" style={{ gap: 6, marginTop: 12 }}><span className="sr-label">Detay destekleyen acenteler:</span>{(d.supported || []).map((p) => <Badge key={p} tone="neutral" dot={false}>{p}</Badge>)}{!d.supported?.length && <span className="muted">—</span>}</div>
        </Card>
      )}
      <div style={{ marginTop: 16 }}>
        <DataTable rows={d?.items || []} loading={loading} search={{ placeholder: "Otel, acente veya özet…", keys: ["internalHotelId", (c) => hotelName(c.internalHotelId), "provider", "summary"] }}
          columns={[
            { key: "changedAt", title: "Zaman", render: (c) => fmtDate(c.changedAt), sortable: true, sortValue: (c) => c.changedAt, width: 170, filterValue: (c) => fmtDate(c.changedAt) },
            { key: "internalHotelId", title: "Otel", render: (c) => <a href={`#hoteldetail/${c.internalHotelId}`} onClick={(e) => e.stopPropagation()}>{hotelName(c.internalHotelId)}</a>, sub: (c) => <code>{c.internalHotelId}</code>, sortable: true, sortValue: (c) => hotelName(c.internalHotelId), filterValue: (c) => hotelName(c.internalHotelId) + " " + c.internalHotelId },
            { key: "provider", title: "Acente", render: (c) => <Badge tone="info" dot={false}>{c.provider}</Badge>, sortable: true },
            { key: "summary", title: "Özet", primary: true, sub: (c) => `sürüm ${c.version} · ${diffLen(c)} alan` },
          ]}
          actions={(c) => [{ icon: "eye", title: "Eski / yeni", onClick: setRec }, { icon: "file", title: "Statik bilgiler", onClick: openStatic }]}
          onRowDoubleClick={setRec} empty={{ icon: "diff", title: "Henüz değişiklik kaydı yok", text: "Günlük kontrol her gece 04:00'te çalışır; değişiklik bulununca burada listelenir." }} />
      </div>
      <ChangeRecordModal record={rec} meta={{ label: rec?.provider, hotelName: rec ? hotelName(rec.internalHotelId) : "" }} onClose={() => setRec(null)}
        footer={rec && <Button variant="secondary" icon="file" onClick={() => { const c = rec; setRec(null); openStatic(c); }}>Statik bilgilere git</Button>} />
    </div>
  );
}
