import React, { useEffect, useState } from "react";
import { api, addDays } from "../api.js";
import { PageHeader, Card, Stat, DataTable, Field, Input, Select, Alert } from "@kit";

export default function Usage() {
  const [from, setFrom] = useState(addDays(-14));
  const [to, setTo] = useState(addDays(0));
  const [clientId, setClientId] = useState("");
  const [clients, setClients] = useState([]);
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");

  useEffect(() => { api("/api/admin/clients").then(setClients).catch(() => {}); }, []);
  const load = async () => { setLoading(true); try { setData(await api(`/api/admin/usage?from=${from}&to=${to}${clientId ? "&clientId=" + clientId : ""}`)); setErr(""); } catch (e) { setErr(e.message); } finally { setLoading(false); } };
  useEffect(() => { load(); }, [from, to, clientId]);

  const name = (id) => clients.find((c) => c.id === id)?.name || `#${id}`;
  return (
    <div>
      <PageHeader title="Kullanım" description="Gün, müşteri ve site bazında kredi tüketimi. Faturalandırmanın temeli bu rapordur: site çağrısı başına 1 kredi, önbellek 0." />
      <Card>
        <div className="sr-row">
          <Field label="Başlangıç"><Input type="date" value={from} onChange={(e) => setFrom(e.target.value)} style={{ width: 170 }} /></Field>
          <Field label="Bitiş"><Input type="date" value={to} onChange={(e) => setTo(e.target.value)} style={{ width: 170 }} /></Field>
          <Field label="Müşteri"><Select value={clientId} onChange={(e) => setClientId(e.target.value)} style={{ width: 220 }}><option value="">Tümü</option>{clients.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}</Select></Field>
        </div>
      </Card>
      {err && <Alert tone="bad">{err}</Alert>}
      <div className="sr-grid" style={{ marginTop: 16 }}>
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
          empty={{ icon: "chart", title: "Kayıt yok", text: "Seçilen aralıkta kullanım bulunmuyor." }} />
      </div>
    </div>
  );
}
