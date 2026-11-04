import React, { useEffect, useState } from "react";
import { api, fmtDate, parseReq } from "../api.js";
import { PageHeader, Card, DataTable, Badge, Alert, EmptyState, statusTone, statusLabel } from "@kit";
import ResultTable from "./ResultTable.jsx";

export default function History({ me, param }) {
  const [list, setList] = useState([]);
  const [loading, setLoading] = useState(true);
  const [sel, setSel] = useState(null);
  const [err, setErr] = useState("");
  const load = () => api("/api/portal/searches?limit=100").then(setList).catch((e) => setErr(e.message)).finally(() => setLoading(false));
  const open = async (id) => { try { setSel(await api("/api/portal/searches/" + id)); } catch (e) { setErr(e.message); } };
  useEffect(() => { load(); if (param) open(param); }, []);
  useEffect(() => {
    if (!sel || sel.status === "done" || sel.status === "failed") return;
    const t = setTimeout(() => open(sel.id), 2000); return () => clearTimeout(t);
  }, [sel]);
  const req = (s) => parseReq(s.request);
  const when = (r) => (r.checkIn ? `${r.checkIn} → ${r.checkOut}` : `+${r.daysAhead} gün, ${r.nights} gece`);

  return (
    <div>
      <PageHeader title="Arama geçmişi" description="Anlık ve planlı aramaların sonuçları. Soldan bir arama seçin; süren aramalar kendiliğinden güncellenir." />
      {err && <Alert tone="bad">{err}</Alert>}
      <div className="sr-row start" style={{ gap: 16 }}>
        <div style={{ flex: "1 1 360px", maxWidth: 480, minWidth: 0 }}>
          <DataTable dense rows={list} loading={loading} pageSize={30} selectedKey={sel?.id} onRowClick={(s) => open(s.id)}
            columns={[
              { key: "createdAt", title: "Arama", primary: true, render: (s) => fmtDate(s.createdAt), sub: (s) => { const r = req(s); return `${s.kind === "packages" ? "Paket" : "Otel"} · ${(r.hotelIds || []).length} otel · ${when(r)}`; } },
              { key: "status", title: "Durum", render: (s) => <Badge tone={statusTone(s.status)}>{statusLabel(s.status)}</Badge> },
              { key: "scheduleId", title: "Kaynak", render: (s) => (s.scheduleId ? <Badge tone="info" dot={false}>planlı</Badge> : <Badge tone="neutral" dot={false}>anlık</Badge>), hideOnMobile: true },
            ]}
            empty={{ icon: "history", title: "Kayıt yok", text: "Henüz arama yapılmadı." }} />
        </div>
        <div style={{ flex: "2 1 520px", minWidth: 0 }}>
          {!sel ? <Card><EmptyState icon="search" title="Bir arama seçin" text="Detay ve karşılaştırma tablosu burada görünür." /></Card> : (
            <Card title={`Arama #${sel.id}`} subtitle={`${sel.result?.checkIn || ""} → ${sel.result?.checkOut || ""} · ${fmtDate(sel.createdAt)}`} actions={<><Badge tone={statusTone(sel.status)}>{statusLabel(sel.status)}</Badge>{sel.scheduleId && <Badge tone="info" dot={false}>planlı</Badge>}</>}>
              {sel.error && <Alert tone="bad">{sel.error}</Alert>}
              {sel.result ? <ResultTable result={sel.result} providers={me.providers} kind={sel.kind} /> : <div className="muted">Sonuç bekleniyor…</div>}
            </Card>
          )}
        </div>
      </div>
    </div>
  );
}
