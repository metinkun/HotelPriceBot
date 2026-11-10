import React from "react";
import { api } from "../api.js";
import { PageHeader } from "@kit";
import { NotificationsView } from "@kit/features/Notifications.jsx";

export default function Notifications({ navigate }) {
  return (
    <div>
      <PageHeader title="Bildirimler" description="Acentelerde otel bilgisi, yorum, puan, etkinlik veya kampanya değiştiğinde üretilen uyarılar. Çift tıklayarak ilgili acentenin statik bilgilerine gidin." />
      <NotificationsView api={api} apiBase="/api/admin" onOpenHotel={(id, prov) => { location.hash = prov ? `hotelstatic/${id}/${prov}` : `hoteldetail/${id}`; }} />
    </div>
  );
}
