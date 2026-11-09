import React, { useEffect, useState } from "react";
import { api, fmtDate, fmtRel } from "../api.js";
import { PageHeader, DataTable, Tabs, Badge, Alert, useConfirm, usePrompt, useToast } from "@kit";

const TYPE = { device_remove: "Cihaz kaldırma", device_replace: "Cihaz değiştirme" };
const STATUS = { pending: ["warn", "bekliyor"], approved: ["ok", "onaylandı"], rejected: ["bad", "reddedildi"] };
const kindLabel = (k) => (k === "mobile" ? "mobil" : k === "pc" ? "pc" : k || "");

export default function Approvals({ navigate }) {
  const [tab, setTab] = useState("pending");
  const [data, setData] = useState({ pending: 0, items: [] });
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");
  const confirm = useConfirm(); const prompt = usePrompt(); const toast = useToast();

  const load = async () => { setLoading(true); try { setData(await api(`/api/admin/approvals?status=${tab === "pending" ? "pending" : "all"}&limit=200`)); setErr(""); } catch (e) { setErr(e.message); } finally { setLoading(false); } };
  useEffect(() => { load(); }, [tab]);

  const payload = (a) => { try { return typeof a.payload === "string" ? JSON.parse(a.payload) : a.payload || {}; } catch { return {}; } };
  const detail = (a) => {
    const p = payload(a);
    const rm = p.removeDeviceName ? `${p.removeDeviceName} (${kindLabel(p.removeDeviceKind)})` : "";
    const nw = p.newDeviceName || p.newDeviceKind ? `${p.newDeviceName || "yeni cihaz"} (${kindLabel(p.newDeviceKind)})` : "";
    if (rm && nw) return <span><b>{rm}</b> kaldırılacak → <b>{nw}</b> eklenecek</span>;
    if (rm) return <span><b>{rm}</b> kaldırılacak</span>;
    return <span className="muted">—</span>;
  };
  const approve = async (a) => {
    if (!(await confirm({ title: "Talep onaylansın mı?", message: "Belirtilen cihaz kaldırılacak; kullanıcı bir sonraki girişte yeni cihazını kaydedebilecek.", confirmText: "Onayla" }))) return;
    try { await api(`/api/admin/approvals/${a.id}/approve`, { method: "POST", body: {} }); toast.success("Talep onaylandı, cihaz kaldırıldı."); await load(); } catch (e) { toast.error(e.message); }
  };
  const reject = async (a) => {
    const note = await prompt({ title: "Talebi reddet", label: "Ret nedeni (kullanıcıya gösterilir)", placeholder: "örn. hesap sahibiyle teyit edilemedi" });
    if (note === null) return;
    try { await api(`/api/admin/approvals/${a.id}/reject`, { method: "POST", body: { note: note || null } }); toast.success("Talep reddedildi."); await load(); } catch (e) { toast.error(e.message); }
  };

  return (
    <div>
      <PageHeader title="Onay bekleyen işlemler" description="Portal kullanıcılarının günlük cihaz değişiklik hakkını aşan veya cihaz sınırı dolduğunda gönderdiği talepler. Admin tarafında sınır yoktur." />
      {err && <Alert tone="bad">{err}</Alert>}
      <Tabs value={tab} onChange={setTab} items={[{ value: "pending", label: "Bekleyen", icon: "clock", count: data.pending }, { value: "all", label: "Tümü", icon: "history" }]} />
      <DataTable rows={data.items} loading={loading} pageSize={50}
        columns={[
          { key: "createdAt", title: "Zaman", primary: true, render: (a) => <span title={fmtDate(a.createdAt)}>{fmtRel(a.createdAt)}</span>, sub: (a) => fmtDate(a.createdAt), sortable: true, sortValue: (a) => a.createdAt },
          { key: "clientName", title: "Müşteri", render: (a) => <a href={`#clientdetail/${a.clientId}`}>{a.clientName || `#${a.clientId}`}</a> },
          { key: "userEmail", title: "Kullanıcı", render: (a) => a.userEmail || <span className="muted">—</span>, hideOnMobile: true },
          { key: "type", title: "Tür", render: (a) => <Badge tone="info" dot={false}>{TYPE[a.type] || a.type}</Badge> },
          { key: "detail", title: "Detay", render: detail },
          { key: "status", title: "Durum", render: (a) => { const [t, l] = STATUS[a.status] || ["neutral", a.status]; return <Badge tone={t}>{l}</Badge>; } },
          { key: "decided", title: "Karar", hideOnMobile: true, render: (a) => (a.decidedAt ? <span className="muted small">{a.decidedBy || "admin"} · {fmtDate(a.decidedAt)}{a.note ? ` · ${a.note}` : ""}</span> : <span className="muted">—</span>) },
        ]}
        actions={(a) => [
          { icon: "check", title: "Onayla", tone: "primary", hidden: a.status !== "pending", onClick: approve },
          { icon: "x", title: "Reddet", tone: "danger", hidden: a.status !== "pending", onClick: reject },
          { icon: "users", title: "Müşteri detayı", onClick: (x) => navigate("clientdetail", x.clientId) },
        ]}
        empty={{ icon: "clipboardCheck", title: tab === "pending" ? "Bekleyen talep yok" : "Kayıt yok", text: "Kullanıcılar cihaz sınırını aştığında talepler burada görünür." }} />
    </div>
  );
}
