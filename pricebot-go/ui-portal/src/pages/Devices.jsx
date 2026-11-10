import React, { useEffect, useState } from "react";
import { api, fmtDate, fmtRel, setToken } from "../api.js";
import { PageHeader, Card, DataTable, Badge, Alert, Icon, Stat, useConfirm, useToast } from "@kit";

const KIND = { pc: ["monitor", "Bilgisayar"], mobile: ["smartphone", "Mobil"] };
const TYPE = { device_remove: "Cihaz kaldırma", device_replace: "Cihaz değiştirme" };
const STATUS = { pending: ["warn", "onay bekliyor"], approved: ["ok", "onaylandı"], rejected: ["bad", "reddedildi"] };

// Cihazlarim: kayitli cihazlar (Face ID / Touch ID / Windows Hello), sinirlar, bekleyen talepler
export default function Devices({ me }) {
  const [data, setData] = useState(null);
  const [err, setErr] = useState("");
  const confirm = useConfirm(); const toast = useToast();
  const load = () => api("/api/portal/devices").then(setData).catch((e) => setErr(e.message));
  useEffect(() => { load(); }, []);

  const remove = async (d) => {
    const isCurrent = d.id === data.currentDeviceId;
    const ok = await confirm({ title: "Cihaz kaldırılsın mı?", message: `${d.name}: bu cihaz bir daha giriş yapamaz.${isCurrent ? " Bu cihaz şu an kullandığınız cihaz; oturumunuz kapanacak." : ""} Günde 1 kaldırma hakkınız var; fazlası yönetici onayına gider.`, confirmText: "Kaldır", danger: true });
    if (!ok) return;
    try {
      const r = await api("/api/portal/devices/" + d.id, { method: "DELETE" });
      if (r.status === "pending") { toast.info(r.message); await load(); return; }
      toast.success("Cihaz kaldırıldı.");
      if (isCurrent) { setToken(""); window.dispatchEvent(new Event("pricebot:logout")); return; }
      await load();
    } catch (e) { toast.error(e.message); }
  };

  if (err) return <Alert tone="bad">{err}</Alert>;
  if (!data) return <div className="muted">Yükleniyor…</div>;
  const lim = data.limits || {};
  const fmtLim = (k) => `${lim[k]?.used ?? 0}/${lim[k]?.max ? lim[k].max : "∞"}`;

  return (
    <div>
      <PageHeader title="Cihazlarım" description="Portala yalnızca kayıtlı cihazlardan girilebilir. Her cihaz Face ID, Touch ID veya Windows Hello ile doğrulanır." />
      {data.requireDeviceAuth === false && <Alert tone="info">Bu hesapta cihaz doğrulaması kapalı; cihaz kaydı gerekmez.</Alert>}
      <div className="sr-grid">
        <Stat icon="monitor" label="Bilgisayar" value={fmtLim("pc")} sub="kayıtlı / izin" />
        <Stat icon="smartphone" label="Mobil" value={fmtLim("mobile")} sub="kayıtlı / izin" soft />
        <Stat icon="refresh" label="Günlük kaldırma hakkı" value={`${data.removalsToday}/${data.dailyRemovalLimit}`} sub="kullanılan / günlük" soft />
      </div>
      <Card style={{ marginTop: 16 }} pad0 title="Kayıtlı cihazlar">
        <DataTable rows={data.devices} className="sr-card-flush" pageSize={0}
          columns={[
            { key: "name", title: "Cihaz", primary: true, render: (d) => <span className="sr-row center" style={{ gap: 8 }}>{d.name}{d.id === data.currentDeviceId && <Badge tone="info" dot={false}>bu cihaz</Badge>}</span>, sub: (d) => d.userAgent ? d.userAgent.slice(0, 70) : "" },
            { key: "kind", title: "Tür", render: (d) => <Badge tone="neutral" dot={false}><Icon name={KIND[d.kind]?.[0] || "monitor"} size={13} /> {KIND[d.kind]?.[1] || d.kind}</Badge> },
            { key: "createdAt", title: "Eklendi", render: (d) => fmtDate(d.createdAt), hideOnMobile: true },
            { key: "lastUsedAt", title: "Son kullanım", render: (d) => fmtRel(d.lastUsedAt) },
          ]}
          actions={(d) => [{ icon: "ban", title: "Kaldır (cihaz kaydı pasife alınır)", tone: "danger", onClick: remove }]}
          empty={{ icon: "fingerprint", title: "Kayıtlı cihaz yok", text: data.requireDeviceAuth === false ? "Cihaz doğrulaması kapalı olduğu için kayıt gerekmiyor." : "Giriş sırasında cihazınız kaydedilir." }} />
      </Card>
      <Card style={{ marginTop: 16 }} pad0 title="Bekleyen talepler" subtitle="Günlük hakkı aşan kaldırma/değiştirme talepleri yönetici onayına gider.">
        <DataTable rows={data.pendingApprovals || []} className="sr-card-flush" pageSize={0}
          columns={[
            { key: "type", title: "Talep", primary: true, render: (a) => TYPE[a.type] || a.type, sub: (a) => { const p = a.payload || {}; return [p.removeDeviceName && `Kaldırılacak: ${p.removeDeviceName}`, p.newDeviceName && `Yeni: ${p.newDeviceName}`].filter(Boolean).join(" · "); } },
            { key: "createdAt", title: "Oluşturma", render: (a) => fmtDate(a.createdAt) },
            { key: "status", title: "Durum", render: (a) => <Badge tone={STATUS[a.status]?.[0] || "neutral"}>{STATUS[a.status]?.[1] || a.status}</Badge> },
          ]}
          empty={{ icon: "clipboardCheck", title: "Bekleyen talep yok" }} />
      </Card>
    </div>
  );
}
