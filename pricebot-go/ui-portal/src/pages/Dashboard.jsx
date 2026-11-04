import React, { useEffect, useState } from "react";
import { api, fmtDate, parseReq } from "../api.js";
import { PageHeader, Stat, Card, DataTable, Badge, Chip, Button, EmptyState, statusTone, statusLabel } from "@kit";

export default function Dashboard({ me, navigate }) {
  const [hotels, setHotels] = useState([]);
  const [searches, setSearches] = useState([]);
  const [schedules, setSchedules] = useState([]);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    Promise.all([
      api("/api/portal/hotels").then(setHotels).catch(() => {}),
      api("/api/portal/searches?limit=8").then(setSearches).catch(() => {}),
      api("/api/portal/schedules").then(setSchedules).catch(() => {}),
    ]).finally(() => setLoading(false));
  }, []);
  const enabled = me.providers.filter((p) => p.enabled);
  const next = schedules.filter((s) => s.isActive && s.nextRunAt).sort((a, b) => new Date(a.nextRunAt) - new Date(b.nextRunAt))[0];
  const open = (s) => navigate("history", s.id);

  return (
    <div>
      <PageHeader title={`Hoş geldiniz, ${me.client.name}`} description="Otellerinizin güncel fiyatlarını tek ekrandan takip edin." actions={<Button icon="search" onClick={() => navigate("search")}>Fiyat ara</Button>} />
      <div className="sr-grid">
        <Stat icon="building" label="Takip edilen otel" value={hotels.length} onClick={() => navigate("hotels")} />
        <Stat icon="globe" label="Etkin site" value={enabled.length} unit={`/ ${me.providers.length}`} soft onClick={() => navigate("sites")} />
        <Stat icon="calendar" label="Aktif plan" value={schedules.filter((s) => s.isActive).length} soft onClick={() => navigate("schedules")} />
        <Stat icon="clock" label="Sonraki planlı arama" value={<span style={{ fontSize: 15 }}>{next ? fmtDate(next.nextRunAt) : "—"}</span>} sub={next ? next.name : "planlı arama yok"} soft />
      </div>
      <div className="sr-grid cols-2" style={{ marginTop: 16, alignItems: "start" }}>
        <Card title="Son aramalar" subtitle="Anlık ve planlı aramaların sonuçları" actions={<Button size="sm" variant="ghost" iconRight="arrowRight" onClick={() => navigate("history")}>Tümü</Button>} pad0>
          <div style={{ padding: "0 0 4px" }}>
            {searches.length === 0 && !loading ? (
              <EmptyState icon="search" title="Henüz arama yok" text="İlk aramanızı yapın, sonuçlar burada görünsün." action={<Button icon="search" onClick={() => navigate("search")}>İlk aramanızı yapın</Button>} />
            ) : (
              <DataTable dense rows={searches} loading={loading} className="sr-card-flush" pageSize={0}
                columns={[
                  { key: "createdAt", title: "Zaman", primary: true, render: (s) => fmtDate(s.createdAt) },
                  { key: "kind", title: "Arama", render: (s) => `${s.kind === "packages" ? "Paket" : "Otel"} · ${(parseReq(s.request).hotelIds || []).length} otel` },
                  { key: "status", title: "Durum", render: (s) => <Badge tone={statusTone(s.status)}>{statusLabel(s.status)}</Badge> },
                  { key: "scheduleId", title: "Kaynak", render: (s) => (s.scheduleId ? <Badge tone="info" dot={false}>planlı</Badge> : <Badge tone="neutral" dot={false}>anlık</Badge>), hideOnMobile: true },
                ]}
                actions={(s) => [{ icon: "eye", title: "Sonucu aç", onClick: open }]} onRowDoubleClick={open} />
            )}
          </div>
        </Card>
        <Card title="Etkin siteler" subtitle={`${enabled.length} site fiyat kaynağı olarak açık`}>
          <div className="sr-chips">
            {enabled.map((p) => <Chip key={p.name} on>{p.label}</Chip>)}
            {me.providers.filter((p) => !p.enabled).map((p) => <Chip key={p.name} off>{p.label}</Chip>)}
          </div>
          <p className="muted small" style={{ marginTop: 14 }}>Site eklemek veya kapatmak için hesap yöneticinizle iletişime geçin.</p>
        </Card>
      </div>
    </div>
  );
}
