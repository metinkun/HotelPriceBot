import React from "react";
import { api } from "../api.js";
import { HotelDetailView } from "@kit";

export default function HotelStatic({ me, param, extra, navigate }) {
  return <HotelDetailView api={api} apiBase="/api/portal" hotelId={param} initialProvider={extra} labelOf={(n) => (me.providers.find((p) => p.name === n) || {}).label || n} onOverview={() => navigate("hoteldetail", param)} onBack={() => navigate("hotels")} />;
}
