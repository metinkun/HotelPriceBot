import React, { useEffect, useState } from "react";
import { api, fmtDate } from "../api.js";
import { PageHeader, DataTable, Select, Badge, Alert } from "@kit";

export default function Logs() {
  const [rows, setRows] = useState([]);
  const [clients, setClients] = useState([]);
  const [clientId, setClientId] = useState("");
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");
  useEffect(() => { api("/api/admin/clients").then(setClients).catch(() => {}); }, []);
  const load = async () => { try { setRows(await api(`/api/admin/logs?limit=200${clientId ? "&clientId=" + clientId : ""}`)); setErr(""); } catch (e) { setErr(e.message); } finally { setLoading(false); } };
  useEffect(() => { setLoading(true); load(); const t = setInterval(load, 10000); return () => clearInterval(t); }, [clientId]);
  const name = (id) => clients.find((c) => c.id === id)?.name || (id ? `#${id}` : "—");
  const tone = (s) => (s < 400 ? "ok" : s < 500 ? "warn" : "bad");
  return (
    <div>
      <PageHeader title="İstek logları" description="Son 200 istek; 10 saniyede bir yenilenir. Müşteri 'isteğim neden hata verdi' dediğinde istek kimliğiyle burada bulunur." />
      {err && <Alert tone="bad">{err}</Alert>}
      <DataTable rows={rows} loading={loading} dense pageSize={50}
        toolbar={<Select value={clientId} onChange={(e) => setClientId(e.target.value)} style={{ width: 220 }}><option value="">Tüm müşteriler</option>{clients.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}</Select>}
        columns={[
          { key: "at", title: "Zaman", primary: true, render: (r) => fmtDate(r.at), sortable: true, sortValue: (r) => r.at },
          { key: "clientId", title: "Müşteri", render: (r) => name(r.clientId) },
          { key: "path", title: "İstek", render: (r) => <code>{r.method} {r.path}</code> },
          { key: "status", title: "Durum", render: (r) => <Badge tone={tone(r.status)} dot={false}>{r.status}</Badge>, sortable: true },
          { key: "durationMs", title: "Süre", align: "num", render: (r) => `${r.durationMs} ms`, sortable: true, hideOnMobile: true },
          { key: "ip", title: "IP", render: (r) => <span className="muted">{r.ip}</span>, hideOnMobile: true },
          { key: "requestId", title: "Request Id", render: (r) => <span className="mono small muted">{r.requestId}</span>, hideOnMobile: true },
        ]}
        empty={{ icon: "list", title: "Kayıt yok" }} />
    </div>
  );
}
