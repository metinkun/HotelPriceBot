import React from "react";
import { api } from "../api.js";
import { PageHeader } from "@kit";
import { NotificationsView } from "@kit/features/Notifications.jsx";

export default function Notifications({ me }) {
  return (
    <div>
      <PageHeader title="Bildirimler" description="Acentelerdeki otel bilgisi değişiklikleri. Çift tıklayarak ilgili acentenin detayına gidin." />
      <NotificationsView api={api} apiBase="/api/portal" labelOf={(n) => (me.providers.find((p) => p.name === n) || {}).label || n} onOpenHotel={(id, prov) => { location.hash = `hoteldetail/${id}/${prov}`; }} />
    </div>
  );
}
