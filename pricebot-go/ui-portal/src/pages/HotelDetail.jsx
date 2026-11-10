import React from "react";
import { api } from "../api.js";
import { HotelOverviewView } from "@kit";

// Otel ozeti (bizim detay ekrani); "Statik bilgiler" ile acente bazli ekrana gecilir.
export default function HotelDetail({ me, param, navigate }) {
  return <HotelOverviewView api={api} apiBase="/api/portal" hotelId={param} onBack={() => navigate("hotels")} onRunSearch={() => navigate("search", param)}
    onOpenStatic={(prov) => { location.hash = `hotelstatic/${param}/${prov || ""}`; }} />;
}
