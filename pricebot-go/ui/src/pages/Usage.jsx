import React, { useEffect, useState } from "react";
import { api, addDays, fmtDay } from "../api.js";
import { PageHeader, Card, Stat, DataTable, Field, Input, Select, Tabs, Alert } from "@kit";

export default function Usage() {
  const [tab, setTab] = useState("searches");
  const [from, setFrom] = useState(addDays(-14));
  const [to, setTo] = useState(addDays(0));
  const [clientId, setClientId] = useState("");
  const [clients, setClients] = useState([]);
  const [data, setData] = useState(null);   // site cagrilari (kredi)
  const [daily, setDaily] = useState(null); // arama sayilari
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");

  useEffect(() => { api("/api/admin/clients").then(setClients).catch(() => {}); }, []);
  const load = async () => {
    setLoading(true);
    const qs = `from=${from}&to=${to}${clientId ? "&clientId=" + clientId : ""}`;
    try { const [u, d] = await Promise.all([api(`/api/admin/usage?${qs}`), api(`/api/admin/usage/daily?${qs}`)]); setData(u); setDaily(d); setErr(""); } catch (e) { setErr(e.message); } finally { setLoading(false); }
  };
  useEffect(() => { load(); }, [from, to, clientId]);

  const name = (id) => clients.find((c) => c.id === id)?.name || `#${id}`;
  return (
    <div>
      <PageHeader title="Kullanım" description="Müşteriye gösterilen birim günlük arama sayısıdır (bir arama kaç siteye giderse gitsin 1). Site çağrıları yalnızca burada, iç maliyet takibi için görünür." />
      <Card>
        <div className="sr-row">
          <Field label="Başlangıç"><Input type="date" value={from} onChange={(e) => setFrom(e.target.value)} style={{ width: 170 }} /></Field>
          <Field label="Bitiş"><Input type="date" value={to} onChange={(e) => setTo(e.target.value)} style={{ width: 170 }} /></Field>
          <Field label="Müşteri"><Select value={clientId} onChange={(e) => setClientId(e.target.value)} style={{ width: 220 }}><option value="">Tümü</option>{clients.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}</Select></Field>
        </div>
      </Card>
      {err && <Alert tone="bad">{err}</Alert>}
      <div style={{ marginTop: 16 }}><Tabs value={tab} onChange={setTab} items={[{ value: "searches", label: "Arama sayıları", icon: "search" }, { value: "calls", label: "Site çağrıları (detay)", icon: "zap" }]} /></div>
      {tab === "searches" && (<>
        <div className="sr-grid">
          <Stat icon="search" label="Arama" value={daily?.totals?.searches ?? "—"} sub="müşteriye gösterilen birim" />
          <Stat icon="calendar" label="Planlı çalışma" value={daily?.totals?.scheduledRuns ?? "—"} soft />
          <Stat icon="server" label="API işi" value={daily?.totals?.apiJobs ?? "—"} soft />
          <Stat icon="zap" label="Site çağrısı" value={daily?.totals?.siteCalls ?? "—"} soft sub="iç maliyet" />
        </div>
        <div style={{ marginTop: 16 }}>
          <DataTable rows={daily?.rows || []} rowKey={(r) => `${r.day}-${r.clientId}`} loading={loading} dense pageSize={50}
            columns={[
              { key: "day", title: "Gün", primary: true, sortable: true, render: (r) => fmtDay(r.day), sortValue: (r) => r.day },
              { key: "clientId", title: "Müşteri", render: (r) => <a href={`#clientdetail/${r.clientId}`}>{name(r.clientId)}</a>, sortable: true, sortValue: (r) => name(r.clientId) },
              { key: "searches", title: "Arama", align: "num", sortable: true },
              { key: "scheduledRuns", title: "Planlı", align: "num", sortable: true },
              { key: "apiJobs", title: "API", align: "num", sortable: true, hideOnMobile: true },
              { key: "siteCalls", title: "Site çağrısı", align: "num", sortable: true },
            ]}
            empty={{ icon: "chart", title: "Kayıt yok", text: "Seçilen aralıkta arama bulunmuyor." }} />
        </div>
      </>)}
      {tab === "calls" && (<>
        <div className="sr-grid">
          <Stat icon="zap" label="Kredi (site çağrısı)" value={data?.totals?.credits ?? "—"} />
          <Stat icon="activity" label="İstek" value={data?.totals?.requests ?? "—"} soft />
        </div>
        <div style={{ marginTop: 16 }}>
          <DataTable rows={data?.rows || []} rowKey={(r, i) => i} loading={loading} dense pageSize={50}
            columns={[
              { key: "day", title: "Gün", primary: true, sortable: true },
              { key: "clientId", title: "Müşteri", render: (r) => name(r.clientId), sortable: true, sortValue: (r) => name(r.clientId) },
              { key: "provider", title: "Site", render: (r) => (r.provider === "*" ? <span className="muted">(istek sayacı)</span> : r.provider), sortable: true },
              { key: "credits", title: "Kredi", align: "num", sortable: true },
              { key: "requests", title: "İstek", align: "num", sortable: true },
            ]}
            empty={{ icon: "chart", title: "Kayıt yok", text: "Seçilen aralıkta site çağrısı bulunmuyor." }} />
        </div>
      </>)}
    </div>
  );
}
