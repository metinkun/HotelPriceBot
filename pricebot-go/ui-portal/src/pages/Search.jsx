import React, { useEffect, useState } from "react";
import { api, addDays } from "../api.js";
import { PageHeader, Card, Button, Field, Input, FormGrid, FormSection, Segmented, ChipSelect, Badge, Progress, Alert, EmptyState, statusTone, statusLabel } from "@kit";
import ResultTable from "./ResultTable.jsx";

export default function Search({ me, param, navigate }) {
  const [hotels, setHotels] = useState([]);
  const [sel, setSel] = useState([]);
  const [kind, setKind] = useState("prices");
  const [checkIn, setCheckIn] = useState(addDays(30));
  const [checkOut, setCheckOut] = useState(addDays(33));
  const [adults, setAdults] = useState(2);
  const [children, setChildren] = useState(0);
  const [childAges, setChildAges] = useState("");
  const [airport, setAirport] = useState("SAW");
  const [provs, setProvs] = useState([]);
  const [search, setSearch] = useState(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const enabled = me.providers.filter((p) => p.enabled);

  useEffect(() => {
    api("/api/portal/hotels").then((h) => { setHotels(h); if (param) setSel([param]); else if (h.length) setSel([h[0].internalHotelId]); }).catch((e) => setErr(e.message));
  }, []);

  // Sonuc bitene kadar sorgula (1.5 sn)
  useEffect(() => {
    if (!search || search.status === "done" || search.status === "failed") return;
    const t = setTimeout(() => api("/api/portal/searches/" + search.id).then(setSearch).catch(() => {}), 1500);
    return () => clearTimeout(t);
  }, [search]);

  const run = async (e) => {
    e.preventDefault(); setBusy(true); setErr("");
    try {
      const body = { kind, hotelIds: sel, checkIn, checkOut, adults: Number(adults), children: Number(children), childAges: childAges.split(",").map((x) => Number(x.trim())).filter((x) => !Number.isNaN(x) && childAges.trim()), providers: provs, airportCode: kind === "packages" ? airport : undefined };
      setSearch(await api("/api/portal/searches", { method: "POST", body }));
    } catch (ex) { setErr(ex.message); } finally { setBusy(false); }
  };
  const result = search?.result;
  const running = search && search.status !== "done" && search.status !== "failed";

  return (
    <div>
      <PageHeader title="Fiyat araması" description="Seçtiğiniz oteller için tüm etkin sitelerden anlık fiyat karşılaştırması. Sonuçlar geldikçe tablo kendiliğinden dolar." />
      {err && <Alert tone="bad">{err}</Alert>}
      <Card title="Arama">
        <form onSubmit={run}>
          <FormSection>Oteller</FormSection>
          {hotels.length === 0 ? <EmptyState icon="building" title="Önce otel ekleyin" action={<Button variant="secondary" icon="plus" onClick={() => navigate("hotels")}>Otellerim</Button>} />
            : <ChipSelect value={sel} onChange={setSel} options={hotels.map((h) => ({ value: h.internalHotelId, label: h.hotelName || h.internalHotelId }))} />}
          <FormSection>Tarih ve kişi</FormSection>
          <div style={{ marginBottom: 12 }}>
            <Segmented value={kind} onChange={setKind} options={[{ value: "prices", label: "Sadece otel" }, { value: "packages", label: "Otel + uçak + transfer", disabled: !me.plan.allowPackages }]} />
            {!me.plan.allowPackages && <span className="muted small" style={{ marginLeft: 10 }}>Paket sorgusu planınızda kapalı.</span>}
          </div>
          <FormGrid style={{ gridTemplateColumns: "repeat(auto-fit, minmax(140px, 1fr))" }}>
            <Field label="Giriş"><Input type="date" value={checkIn} onChange={(e) => setCheckIn(e.target.value)} required /></Field>
            <Field label="Çıkış"><Input type="date" value={checkOut} onChange={(e) => setCheckOut(e.target.value)} required /></Field>
            <Field label="Yetişkin"><Input type="number" min="1" max="8" value={adults} onChange={(e) => setAdults(e.target.value)} /></Field>
            <Field label="Çocuk"><Input type="number" min="0" max="6" value={children} onChange={(e) => setChildren(e.target.value)} /></Field>
            {Number(children) > 0 && <Field label="Çocuk yaşları" hint="Virgülle: 5,8"><Input placeholder="5,8" value={childAges} onChange={(e) => setChildAges(e.target.value)} /></Field>}
            {kind === "packages" && <Field label="Kalkış (IATA)"><Input value={airport} onChange={(e) => setAirport(e.target.value.toUpperCase())} maxLength={3} placeholder="SAW" /></Field>}
          </FormGrid>
          <FormSection>Siteler <span className="muted" style={{ textTransform: "none", letterSpacing: 0, fontWeight: 500 }}>(boş = tüm etkin siteler)</span></FormSection>
          <ChipSelect value={provs} onChange={setProvs} options={enabled.map((p) => ({ value: p.name, label: p.label, hint: p.requiresBrowser ? "~10 sn" : undefined }))} />
          <div className="sr-row center" style={{ marginTop: 20 }}>
            <Button type="submit" size="lg" icon="search" loading={busy} disabled={sel.length === 0}>Fiyatları getir</Button>
            <span className="muted small">{sel.length} otel · {provs.length || enabled.length} site</span>
          </div>
        </form>
      </Card>
      {search && (
        <Card style={{ marginTop: 16 }} title="Sonuçlar" subtitle={result ? `${result.checkIn} → ${result.checkOut}` : ""} actions={<Badge tone={statusTone(search.status)}>{running ? "sorgulanıyor…" : statusLabel(search.status)}</Badge>}>
          {running && result && <Progress value={result.done || 0} max={result.total || 1} style={{ marginBottom: 14 }} />}
          {running && !result && <Progress indeterminate style={{ marginBottom: 14 }} />}
          {search.error && <Alert tone="bad">{search.error}</Alert>}
          {result && <ResultTable result={result} providers={me.providers} kind={kind} />}
        </Card>
      )}
    </div>
  );
}
