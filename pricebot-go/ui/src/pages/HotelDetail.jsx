import React from "react";
import { api } from "../api.js";
import { HotelDetailView } from "@kit/features/HotelDetail.jsx";

export default function HotelDetail({ param, extra, navigate }) {
  return <HotelDetailView api={api} apiBase="/api/admin" hotelId={param} initialProvider={extra} onBack={() => navigate("hotels")} />;
}
