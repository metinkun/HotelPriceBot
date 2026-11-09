import React from "react";
import { PageHeader, DataTable, Badge } from "@kit";

export default function Sites({ me }) {
  const rows = me.providers.map((p) => ({ ...p, id: p.name }));
  const status = (p) => (p.maintenance ? <Badge tone="warn">bakımda</Badge> : p.enabled ? <Badge tone="ok">etkin</Badge> : <Badge tone="neutral">kapalı</Badge>);
  return (
    <div>
      <PageHeader title="Siteler" description="Hesabınız için etkin olan fiyat kaynakları. Site eklemek/kapatmak için hesap yöneticinizle görüşün. 'Bakımda' siteler geçici olarak tüm müşterilere kapalıdır." />
      <DataTable rows={rows} rowKey={(p) => p.name} pageSize={0}
        columns={[
          { key: "label", title: "Site", primary: true, sub: (p) => p.domain },
          { key: "enabled", title: "Durum", render: status },
          { key: "supportsPackages", title: "Paket", render: (p) => (p.supportsPackages ? <Badge tone="info" dot={false}>otel + uçak</Badge> : <span className="muted">—</span>), hideOnMobile: true },
          { key: "requiresBrowser", title: "Not", render: (p) => (p.maintenance ? <span className="muted">Geçici olarak kapalı; aramalarda atlanır.</span> : p.requiresBrowser ? <span className="muted">Tarayıcı tabanlı; sorgu birkaç saniye daha uzun sürebilir.</span> : <span className="muted">—</span>), hideOnMobile: true },
        ]} />
    </div>
  );
}
