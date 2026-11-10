import React from "react";
import { api } from "../api.js";
import { PageHeader, NotificationsView } from "@kit";

export default function Notifications({ me }) {
  return (
    <div>
      <PageHeader title="Bildirimler" description="Acentelerdeki otel bilgisi, yorum, puan, etkinlik ve kampanya değişiklikleri. Çift tıklayarak ilgili acentenin statik bilgilerine gidin." />
      <NotificationsView api={api} apiBase="/api/portal" labelOf={(n) => (me.providers.find((p) => p.name === n) || {}).label || n} onOpenHotel={(id, prov) => { location.hash = prov ? `hotelstatic/${id}/${prov}` : `hoteldetail/${id}`; }} />
    </div>
  );
}
