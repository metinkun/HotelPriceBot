import React, { useEffect, useState } from "react";
import { api, fmtDate } from "../api.js";
import { PageHeader, DataTable, Select, Badge, Alert } from "@kit";

export const LOGIN_RESULT = {
  ok: ["ok", "başarılı"], bad_password: ["bad", "yanlış şifre"], inactive: ["neutral", "pasif"], expired: ["bad", "demo süresi doldu"],
  device_required: ["info", "cihaz kaydı"], device_limit: ["warn", "cihaz sınırı"], webauthn_failed: ["bad", "doğrulama hatası"], removed_device: ["warn", "kaldırılmış cihaz"],
};
export const resultBadge = (r) => { const [t, l] = LOGIN_RESULT[r] || ["neutral", r]; return <Badge tone={t}>{l}</Badge>; };
export const kindBadge = (k) => (k ? <Badge tone="neutral" dot={false}>{k === "mobile" ? "mobil" : "pc"}</Badge> : <span className="muted">—</span>);

// Giris logu sutunlari (musteri detayindaki sekmeyle paylasilir)
export const loginLogColumns = (extra = []) => [
  { key: "at", title: "Zaman", primary: true, render: (l) => fmtDate(l.at), sortable: true, sortValue: (l) => l.at },
  ...extra,
  { key: "email", title: "Kullanıcı", render: (l) => l.email || <span className="muted">—</span> },
  { key: "result", title: "Sonuç", render: (l) => resultBadge(l.result) },
  { key: "deviceKind", title: "Cihaz", render: (l) => kindBadge(l.deviceKind), hideOnMobile: true },
  { key: "ip", title: "IP", render: (l) => <span className="mono small">{l.ip || "—"}</span>, hideOnMobile: true },
  { key: "userAgent", title: "Tarayıcı", hideOnMobile: true, render: (l) => <span className="muted small truncate" style={{ maxWidth: 220, display: "inline-block" }} title={l.userAgent || ""}>{l.userAgent || "—"}</span> },
  { key: "detail", title: "Detay", hideOnMobile: true, render: (l) => <span className="muted small">{l.detail || ""}</span> },
];

export default function LoginLogs() {
  const [rows, setRows] = useState([]);
  const [clients, setClients] = useState([]);
  const [clientId, setClientId] = useState("");
  const [result, setResult] = useState("");
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");

  useEffect(() => { api("/api/admin/clients").then(setClients).catch(() => {}); }, []);
  const load = async () => { try { setRows(await api(`/api/admin/login-logs?limit=200${clientId ? "&clientId=" + clientId : ""}`)); setErr(""); } catch (e) { setErr(e.message); } finally { setLoading(false); } };
  useEffect(() => { load(); const t = setInterval(load, 30000); return () => clearInterval(t); }, [clientId]);

  const name = (id) => clients.find((c) => c.id === id)?.name || (id ? `#${id}` : "—");
  const visible = result ? rows.filter((l) => l.result === result) : rows;
  return (
    <div>
      <PageHeader title="Giriş logları" description="Müşteri portalına yapılan giriş denemeleri: sonuç, cihaz, IP ve tarayıcı. 30 saniyede bir yenilenir." />
      {err && <Alert tone="bad">{err}</Alert>}
      <DataTable rows={visible} loading={loading} pageSize={50}
        toolbar={<>
          <Select value={clientId} onChange={(e) => setClientId(e.target.value)} style={{ width: 220 }}><option value="">Tüm müşteriler</option>{clients.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}</Select>
          <Select value={result} onChange={(e) => setResult(e.target.value)} style={{ width: 200 }}><option value="">Tüm sonuçlar</option>{Object.entries(LOGIN_RESULT).map(([k, [, l]]) => <option key={k} value={k}>{l}</option>)}</Select>
        </>}
        columns={loginLogColumns([{ key: "clientId", title: "Müşteri", render: (l) => (l.clientId ? <a href={`#clientdetail/${l.clientId}`}>{name(l.clientId)}</a> : <span className="muted">—</span>) }])}
        empty={{ icon: "logIn", title: "Giriş kaydı yok", text: "Portal kullanıcıları giriş yaptıkça burada listelenir." }} />
    </div>
  );
}
