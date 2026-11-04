import React, { useEffect, useState } from "react";
import { api, fmtDate, parseReq, WEEKDAYS_SHORT } from "../api.js";
import { PageHeader, DataTable, Badge, Alert } from "@kit";

export default function Schedules({ navigate }) {
  const [list, setList] = useState([]);
  const [clients, setClients] = useState([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");
  useEffect(() => { Promise.all([api("/api/admin/schedules"), api("/api/admin/clients")]).then(([s, c]) => { setList(s); setClients(c); }).catch((e) => setErr(e.message)).finally(() => setLoading(false)); }, []);
  const name = (id) => clients.find((c) => c.id === id)?.name || `#${id}`;
  const open = (s) => navigate("clientdetail", s.clientId);
  return (
    <div>
      <PageHeader title="Planlı aramalar" description="Tüm müşterilerin tekrarlı aramaları. Planları müşteri portaldan yönetir; buradan müşteri detayına geçebilirsiniz." />
      {err && <Alert tone="bad">{err}</Alert>}
      <DataTable rows={list} loading={loading} search={{ placeholder: "Müşteri veya plan adı…", keys: [(s) => name(s.clientId), "name"] }}
        columns={[
          { key: "clientId", title: "Müşteri", primary: true, render: (s) => name(s.clientId), sortable: true, sortValue: (s) => name(s.clientId) },
          { key: "name", title: "Plan", sortable: true },
          { key: "time", title: "Zaman", render: (s) => <span>{s.weekdays.length ? s.weekdays.map((d) => WEEKDAYS_SHORT[d]).join(", ") : "Her gün"} {s.runTime} <span className="muted small">({s.timezone})</span></span> },
          { key: "kind", title: "Arama", render: (s) => { const r = parseReq(s.request); return `${s.kind === "packages" ? "Paket" : "Otel"} · ${(r.hotelIds || []).length} otel · +${r.daysAhead}g / ${r.nights} gece`; }, hideOnMobile: true },
          { key: "nextRunAt", title: "Sonraki", render: (s) => fmtDate(s.nextRunAt), sortable: true },
          { key: "lastRunAt", title: "Son", render: (s) => <span className="muted">{fmtDate(s.lastRunAt)}</span>, hideOnMobile: true },
          { key: "isActive", title: "Durum", render: (s) => (s.isActive ? <Badge tone="ok">aktif</Badge> : <Badge tone="neutral">pasif</Badge>) },
        ]}
        actions={(s) => [{ icon: "users", title: "Müşteri detayı", onClick: open }]}
        onRowDoubleClick={open} empty={{ icon: "calendar", title: "Planlı arama yok", text: "Müşteriler portaldan plan oluşturduğunda burada listelenir." }} />
    </div>
  );
}
