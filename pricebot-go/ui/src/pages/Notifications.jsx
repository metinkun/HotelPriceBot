import React from "react";
import { api } from "../api.js";
import { PageHeader } from "@kit";
import { NotificationsView } from "@kit/features/Notifications.jsx";

export default function Notifications({ navigate }) {
  return (
    <div>
      <PageHeader title="Bildirimler" description="Acentelerde otel bilgisi değiştiğinde üretilen uyarılar. Çift tıklayarak ilgili acentenin detayına gidin." />
      <NotificationsView api={api} apiBase="/api/admin" onOpenHotel={(id, prov) => { location.hash = `hoteldetail/${id}/${prov}`; }} />
    </div>
  );
}
