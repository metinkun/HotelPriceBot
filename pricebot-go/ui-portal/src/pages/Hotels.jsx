import React, { useEffect, useState } from "react";
import { api } from "../api.js";
import { PageHeader, DataTable, Button, Badge, Alert } from "@kit";

// Otellerim: otel listesi hesap yoneticisi tarafindan yonetilir (portaldan ekleme/cikarma yoktur).
export default function Hotels({ me, navigate }) {
  const [hotels, setHotels] = useState([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");
  const label = (n) => (me.providers.find((x) => x.name === n) || {}).label || n;
  const L = me.limits || {};
  const ownId = me.client?.kind === "otel" ? me.client?.ownHotelId : null;
  useEffect(() => { api("/api/portal/hotels").then(setHotels).catch((e) => setErr(e.message)).finally(() => setLoading(false)); }, []);
  const open = (h) => navigate("hoteldetail", h.internalHotelId);
  return (
    <div>
      <PageHeader title="Otellerim" description={`Takip ettiğiniz oteller ve her sitedeki eşleştirmeleri. Satıra çift tıklayarak otel özetine gidin.${L.maxHotels > 0 ? ` Paket limiti: ${hotels.length}/${L.maxHotels} otel.` : ""} Otel eklemek veya çıkarmak için hesap yetkilinizle iletişime geçin.`} actions={<Button icon="search" onClick={() => navigate("search")}>Fiyat ara</Button>} />
      {err && <Alert tone="bad">{err}</Alert>}
      <DataTable rows={hotels} rowKey={(h) => h.internalHotelId} loading={loading} search={{ placeholder: "Otel ara…", keys: ["hotelName", "internalHotelId", (h) => h.providers.map((p) => label(p.provider)).join(" ")] }}
        columns={[
          { key: "hotelName", title: "Otel", primary: true, sortable: true, render: (h) => <span className="sr-row center" style={{ gap: 8 }}>{h.internalHotelId === ownId && <Badge tone="info" dot={false}>Otelim</Badge>}{h.hotelName || h.internalHotelId}</span>, sub: (h) => <code>{h.internalHotelId}</code>, filterValue: (h) => (h.hotelName || "") + " " + h.internalHotelId },
          { key: "providers", title: "Siteler", render: (h) => <div className="sr-chips" style={{ gap: 4 }}>{h.providers.map((p) => <Badge key={p.provider} tone={p.enabled ? "info" : "neutral"} dot={false} title={p.sourceUrl || ""}>{label(p.provider)}</Badge>)}</div>, hideOnMobile: true, filterValue: (h) => h.providers.map((p) => label(p.provider)).join(" ") },
          { key: "count", title: "Site", align: "num", width: 70, sortable: true, sortValue: (h) => h.providers.length, render: (h) => h.providers.length },
        ]}
        actions={(h) => [{ icon: "eye", title: "Otel özeti", onClick: open }, { icon: "file", title: "Statik bilgiler", onClick: (x) => { location.hash = `hotelstatic/${x.internalHotelId}`; } }, { icon: "search", title: "Fiyat ara", tone: "primary", onClick: (x) => navigate("search", x.internalHotelId) }]}
        onRowDoubleClick={open} empty={{ icon: "building", title: "Henüz otel tanımlı değil", text: "Takip edilecek oteller hesap yetkiliniz tarafından tanımlanır." }} />
    </div>
  );
}
