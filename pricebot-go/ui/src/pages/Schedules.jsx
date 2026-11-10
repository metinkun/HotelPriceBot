import React, { useEffect, useState } from "react";
import { api, fmtDate, parseReq, WEEKDAYS_SHORT } from "../api.js";
import { PageHeader, DataTable, Modal, Button, Badge, Alert, SearchDetailModal, statusTone, statusLabel, useToast } from "@kit";

const range = (s) => (s.startsOn || s.endsOn ? `${s.startsOn || "…"} → ${s.endsOn || "…"}` : "");
const expired = (s) => s.endsOn && s.endsOn < new Date().toISOString().slice(0, 10);

export default function Schedules({ navigate }) {
  const [list, setList] = useState([]);
  const [clients, setClients] = useState([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");
  const [hist, setHist] = useState(null); // {schedule, items}
  const [detail, setDetail] = useState(null);
  const toast = useToast();
  useEffect(() => { Promise.all([api("/api/admin/schedules"), api("/api/admin/clients")]).then(([s, c]) => { setList(s); setClients(c); }).catch((e) => setErr(e.message)).finally(() => setLoading(false)); }, []);
  const name = (id) => clients.find((c) => c.id === id)?.name || `#${id}`;
  const open = (s) => navigate("clientdetail", s.clientId);
  const openHist = async (s) => { try { setHist(await api(`/api/admin/schedules/${s.id}/searches?limit=100`)); } catch (e) { toast.error(e.message); } };
  const openSearch = async (x) => { try { setDetail(await api(`/api/admin/searches/${x.id}`)); } catch (e) { toast.error(e.message); } };
  return (
    <div>
      <PageHeader title="Planlı aramalar" description="Tüm müşterilerin tekrarlı aramaları. Planlı aramayı müşteri portaldan yönetir; buradan geçmiş çalışmaları ve sonuçlarını görebilir, müşteri detayına geçebilirsiniz." />
      {err && <Alert tone="bad">{err}</Alert>}
      <DataTable rows={list} loading={loading} search={{ placeholder: "Müşteri veya planlı arama adı…", keys: [(s) => name(s.clientId), "name"] }}
        columns={[
          { key: "clientId", title: "Müşteri", primary: true, render: (s) => name(s.clientId), sortable: true, sortValue: (s) => name(s.clientId), filterValue: (s) => name(s.clientId) },
          { key: "name", title: "Planlı arama", sortable: true, sub: (s) => range(s) },
          { key: "time", title: "Zaman", render: (s) => <span>{s.weekdays.length ? s.weekdays.map((d) => WEEKDAYS_SHORT[d]).join(", ") : "Her gün"} {s.runTime} <span className="muted small">({s.timezone})</span></span> },
          { key: "kind", title: "Arama", render: (s) => { const r = parseReq(s.request); return `${s.kind === "packages" ? "Paket" : "Otel"} · ${(r.hotelIds || []).length} otel · +${r.daysAhead}g / ${r.nights} gece`; }, hideOnMobile: true },
          { key: "nextRunAt", title: "Sonraki", render: (s) => fmtDate(s.nextRunAt), sortable: true, filterValue: (s) => fmtDate(s.nextRunAt) },
          { key: "lastRunAt", title: "Son", render: (s) => <span className="muted">{fmtDate(s.lastRunAt)}</span>, hideOnMobile: true, filterValue: (s) => fmtDate(s.lastRunAt) },
          { key: "isActive", title: "Durum", render: (s) => (expired(s) ? <Badge tone="neutral">süresi doldu</Badge> : s.isActive ? <Badge tone="ok">aktif</Badge> : <Badge tone="neutral">pasif</Badge>), filterValue: (s) => (expired(s) ? "süresi doldu" : s.isActive ? "aktif" : "pasif") },
        ]}
        actions={(s) => [{ icon: "history", title: "Geçmiş çalışmalar", tone: "primary", onClick: openHist }, { icon: "users", title: "Müşteri detayı", onClick: open }]}
        onRowDoubleClick={openHist} empty={{ icon: "calendar", title: "Planlı arama yok", text: "Müşteriler portaldan planlı arama oluşturduğunda burada listelenir." }} />

      <Modal open={!!hist} onClose={() => setHist(null)} size="lg" title={hist ? `Geçmiş · ${hist.schedule.name}` : ""} subtitle={hist ? `${name(hist.schedule.clientId)} · ${hist.items.length} çalışma. Satıra çift tıklayınca sonuç tablosu açılır.` : ""} footer={<Button variant="secondary" onClick={() => setHist(null)}>Kapat</Button>}>
        {hist && <DataTable rows={hist.items} dense pageSize={25} onRowDoubleClick={openSearch}
          columns={[
            { key: "createdAt", title: "Çalışma", primary: true, render: (x) => fmtDate(x.createdAt), sortable: true, sortValue: (x) => x.createdAt },
            { key: "finishedAt", title: "Bitiş", render: (x) => fmtDate(x.finishedAt), hideOnMobile: true },
            { key: "status", title: "Durum", render: (x) => <Badge tone={statusTone(x.status)}>{statusLabel(x.status)}</Badge>, filterValue: (x) => statusLabel(x.status) },
            { key: "error", title: "Not", render: (x) => <span className="muted small truncate" style={{ maxWidth: 260, display: "inline-block" }} title={x.error || ""}>{x.error || "—"}</span> },
          ]}
          actions={(x) => [{ icon: "eye", title: "Sonuçlar", onClick: openSearch }]} empty={{ icon: "history", title: "Henüz çalışmamış" }} />}
      </Modal>
      <SearchDetailModal data={detail} onClose={() => setDetail(null)} />
    </div>
  );
}
