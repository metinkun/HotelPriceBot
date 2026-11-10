import React from "react";
import { Modal, Button, Badge, KV, Alert, ResultTable } from "../index.js";
import { fmtDate, parseReq, statusTone, statusLabel } from "../format.js";

/*
SearchDetailModal: bir aramanin tum detayi (istek + sonuc matrisi). admin: {search, client, user, schedule, labels}; portal: {search}
*/
export function SearchDetailModal({ data, onClose, labelOf, ownHotelId, footer }) {
  if (!data) return null;
  const s = data.search || data;
  const req = parseReq(s.request);
  const lab = labelOf || ((n) => (data.labels || {})[n] || n);
  const res = s.result;
  return (
    <Modal open onClose={onClose} size="lg" title={<span className="sr-row center" style={{ gap: 8 }}>Arama #{s.id} <Badge tone={statusTone(s.status)}>{statusLabel(s.status)}</Badge>{s.scheduleId && <Badge tone="info" dot={false}>planlı</Badge>}</span>}
      subtitle={`${fmtDate(s.createdAt)}${s.finishedAt ? " → " + fmtDate(s.finishedAt) : ""}`} footer={<>{footer}<Button variant="secondary" onClick={onClose}>Kapat</Button></>}>
      <KV items={[
        ["Müşteri", data.client ? `${data.client.name}${data.client.email ? " · " + data.client.email : ""}` : null],
        ["Kullanıcı", data.user ? data.user.email : null],
        ["Planlı arama", data.schedule ? `${data.schedule.name} (#${data.schedule.id})` : s.scheduleId ? `#${s.scheduleId}` : null],
        ["Tür", s.kind === "packages" ? "Otel + uçak + transfer" : "Sadece otel"],
        ["Tarih", res?.checkIn ? `${res.checkIn} → ${res.checkOut}` : req.checkIn ? `${req.checkIn} → ${req.checkOut}` : `+${req.daysAhead} gün, ${req.nights} gece`],
        ["Kişi", `${req.adults || 2} yetişkin${req.children ? `, ${req.children} çocuk` : ""}${req.airportCode ? ` · kalkış ${req.airportCode}` : ""}`],
        ["Oteller", (req.hotelIds || []).length + " otel"],
        ["Siteler", (req.providers || []).length ? req.providers.map(lab).join(", ") : "tüm etkin siteler"],
        ["İş kimliği", s.jobId || null],
      ]} />
      {s.error && <Alert tone="bad" className="" style={{ marginTop: 12 }}>{s.error}</Alert>}
      <div style={{ marginTop: 14 }}>
        {res ? <ResultTable result={res} labelOf={lab} kind={s.kind} ownHotelId={ownHotelId} /> : <p className="muted">Sonuç yok{s.status === "queued" || s.status === "running" ? " (sürüyor)" : ""}.</p>}
      </div>
    </Modal>
  );
}
