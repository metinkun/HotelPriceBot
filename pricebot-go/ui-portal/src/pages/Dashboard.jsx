import React, { useEffect, useState } from "react";
import { api, fmtDate, parseReq } from "../api.js";
import { PageHeader, Stat, Card, DataTable, Badge, Chip, Button, EmptyState, Alert, Progress, statusTone, statusLabel } from "@kit";

const lim = (v) => (v ? v : "∞");

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
  const L = me.limits || {}; const U = me.usage || {};
  const isOtel = me.client?.kind === "otel";
  const own = isOtel && me.client?.ownHotelId ? hotels.find((h) => h.internalHotelId === me.client.ownHotelId) : null;
  const competitors = isOtel ? hotels.filter((h) => h.internalHotelId !== me.client?.ownHotelId).length : 0;

  return (
    <div>
      <PageHeader title={`Hoş geldiniz, ${me.client.name}`} description={isOtel ? "Otelinizin fiyatını rakiplerinizle tüm acentelerde karşılaştırın." : "Otellerinizin güncel fiyatlarını tek ekrandan takip edin."} actions={<Button icon="search" onClick={() => navigate("search")}>Fiyat ara</Button>} />
      {me.client?.limitsCustomized && <Alert tone="info">Paketinizde size özel düzenlemeler var. Değişiklik için yetkilinizle iletişime geçin.</Alert>}
      {me.client?.demoExpiresAt && <Alert tone="warn">Deneme süreniz {fmtDate(me.client.demoExpiresAt)} tarihinde doluyor.</Alert>}
      <div className="sr-grid">
        <div className="sr-stat" style={{ flexDirection: "column", alignItems: "stretch", gap: 8 }}>
          <div className="sr-row center" style={{ gap: 14 }}><div className="sr-stat-ic"><span style={{ fontWeight: 800 }}>{U.searches ?? 0}</span></div><div><div className="k">Bugünkü arama</div><div className="v">{U.searches ?? 0}<small>/ {lim(L.dailySearches)}</small></div></div></div>
          {L.dailySearches > 0 && <Progress value={U.searches || 0} max={L.dailySearches} />}
        </div>
        <Stat icon="building" label={isOtel ? "Takip edilen otel (otelim + rakipler)" : "Takip edilen otel"} value={hotels.length} unit={`/ ${lim(L.maxHotels)}`} onClick={() => navigate("hotels")} soft />
        <Stat icon="calendar" label="Planlı arama bugün" value={U.scheduledRuns ?? 0} unit={`/ ${lim(L.dailyScheduledRuns)}`} sub={`${schedules.filter((s) => s.isActive).length} aktif planlı arama`} soft onClick={() => navigate("schedules")} />
        <Stat icon="globe" label="Etkin site" value={enabled.length} unit={`/ ${me.providers.length}`} soft onClick={() => navigate("sites")} />
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
        {isOtel ? (
          <Card title="Rakip takibi" subtitle={own ? `Otelim: ${own.hotelName || own.internalHotelId} · ${competitors} rakip` : "Oteliniz henüz tanımlı değil; hesap yöneticinize başvurun."}>
            {own && <div className="sr-row center" style={{ gap: 8, marginBottom: 10 }}><Badge tone="info" dot={false}>Otelim</Badge><b>{own.hotelName || own.internalHotelId}</b></div>}
            <div className="sr-chips">{hotels.filter((h) => h.internalHotelId !== me.client?.ownHotelId).map((h) => <Chip key={h.internalHotelId} onClick={() => navigate("hoteldetail", h.internalHotelId)}>{h.hotelName || h.internalHotelId}</Chip>)}</div>
            <p className="muted small" style={{ marginTop: 14 }}>Fiyat aramalarında oteliniz en üstte sabitlenir, rakip hücrelerinde otelinize göre fark gösterilir. Etkin siteler: {enabled.length}/{me.providers.length}.</p>
          </Card>
        ) : (
          <Card title="Etkin siteler" subtitle={`${enabled.length} site fiyat kaynağı olarak açık`}>
            <div className="sr-chips">
              {enabled.map((p) => <Chip key={p.name} on>{p.label}</Chip>)}
              {me.providers.filter((p) => !p.enabled).map((p) => <Chip key={p.name} off title={p.maintenance ? "bakımda" : "kapalı"}>{p.label}{p.maintenance && <small className="muted">bakımda</small>}</Chip>)}
            </div>
            <p className="muted small" style={{ marginTop: 14 }}>Site eklemek veya kapatmak için hesap yöneticinizle iletişime geçin.</p>
          </Card>
        )}
      </div>
    </div>
  );
}
