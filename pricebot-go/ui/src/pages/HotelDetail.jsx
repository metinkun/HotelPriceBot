import React, { useEffect, useState } from "react";
import { api } from "../api.js";
import { HotelOverviewView, FormModal, Field, Input, Select, Textarea, Switch, FormGrid, useToast } from "@kit";

// Otel detayi: once bizim ozet ekranimiz (kunye, siteler, puan/yorum/etkinlik/kampanya, degisiklikler, musteriler);
// "Statik bilgiler" dugmesi acente bazli statik detay ekranina (hotelstatic) gider.
export default function HotelDetail({ param, navigate }) {
  const [meta, setMeta] = useState(null);
  const [tick, setTick] = useState(0);
  const toast = useToast();
  const openMeta = async () => {
    const list = await api("/api/admin/hotels");
    const h = list.find((x) => x.internalHotelId === param); const m = h?.meta || {};
    setMeta({ id: param, name: h?.hotelName || "", city: m.city || "", region: m.region || "", country: m.country || "", stars: m.stars ?? "", chain: m.chain || "", externalId: m.externalId || "", isActive: m.isActive !== false, notes: m.notes || "" });
  };
  const saveMeta = async () => {
    const body = { name: meta.name, city: meta.city, region: meta.region, country: meta.country, chain: meta.chain, externalId: meta.externalId, isActive: !!meta.isActive, notes: meta.notes };
    if (meta.stars !== "") body.stars = Number(meta.stars);
    await api(`/api/admin/hotels/${meta.id}/meta`, { method: "PUT", body }); setMeta(null); toast.success("Otel künyesi kaydedildi."); setTick((t) => t + 1);
  };
  return (
    <>
      <HotelOverviewView key={tick} api={api} apiBase="/api/admin" hotelId={param} onBack={() => navigate("hotels")}
        onOpenStatic={(prov) => { location.hash = `hotelstatic/${param}/${prov || ""}`; }} onOpenClient={(id) => navigate("clientdetail", id)} onEditMeta={openMeta} />
      <FormModal open={!!meta} onClose={() => setMeta(null)} title="Otel künyesi" subtitle={meta ? meta.id : ""} size="md" onSubmit={saveMeta}>
        {meta && (<>
          <Field label="Ad" required><Input value={meta.name} onChange={(e) => setMeta({ ...meta, name: e.target.value })} required /></Field>
          <FormGrid>
            <Field label="Şehir"><Input value={meta.city} onChange={(e) => setMeta({ ...meta, city: e.target.value })} placeholder="Girne" /></Field>
            <Field label="Bölge"><Input value={meta.region} onChange={(e) => setMeta({ ...meta, region: e.target.value })} placeholder="Alsancak" /></Field>
            <Field label="Ülke"><Input value={meta.country} onChange={(e) => setMeta({ ...meta, country: e.target.value })} placeholder="TRNC" /></Field>
            <Field label="Yıldız"><Select value={meta.stars} onChange={(e) => setMeta({ ...meta, stars: e.target.value })}><option value="">—</option>{[0, 1, 2, 3, 4, 5, 6, 7].map((n) => <option key={n} value={n}>{n ? "★".repeat(n) : "yıldızsız"}</option>)}</Select></Field>
            <Field label="Zincir"><Input value={meta.chain} onChange={(e) => setMeta({ ...meta, chain: e.target.value })} placeholder="Merit" /></Field>
            <Field label="Dış kimlik" hint="Müşteri/Excel kimliği"><Input value={meta.externalId} onChange={(e) => setMeta({ ...meta, externalId: e.target.value })} /></Field>
          </FormGrid>
          <Switch label="Aktif (pasif oteller aramalarda atlanır)" checked={!!meta.isActive} onChange={(e) => setMeta({ ...meta, isActive: e.target.checked })} />
          <Field label="Notlar"><Textarea value={meta.notes} onChange={(e) => setMeta({ ...meta, notes: e.target.value })} rows={3} /></Field>
        </>)}
      </FormModal>
    </>
  );
}
