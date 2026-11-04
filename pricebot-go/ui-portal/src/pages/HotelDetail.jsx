import React from "react";
import { api } from "../api.js";
import { HotelDetailView } from "@kit/features/HotelDetail.jsx";

export default function HotelDetail({ me, param, extra, navigate }) {
  return <HotelDetailView api={api} apiBase="/api/portal" hotelId={param} initialProvider={extra} labelOf={(n) => (me.providers.find((p) => p.name === n) || {}).label || n} onBack={() => navigate("hotels")} />;
}
