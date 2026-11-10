import React from "react";
import { api } from "../api.js";
import { HotelDetailView } from "@kit";

// Acente bazli statik bilgiler (mevcut detay ekrani); otel ozetinden "Statik bilgiler" ile gelinir.
export default function HotelStatic({ param, extra, navigate }) {
  return <HotelDetailView api={api} apiBase="/api/admin" hotelId={param} initialProvider={extra} onOverview={() => navigate("hoteldetail", param)} onBack={() => navigate("hotels")} />;
}
