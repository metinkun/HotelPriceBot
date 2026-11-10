import React, { useEffect, useState } from "react";
import { api, addDays } from "../api.js";
import { PageHeader, Card, Button, Field, Input, FormGrid, FormSection, Segmented, ChipSelect, SitePicker, Badge, Progress, Alert, EmptyState, Icon, statusTone, statusLabel } from "@kit";
import ResultTable from "./ResultTable.jsx";

export default function Search({ me, param, navigate, refreshMe }) {
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
  const isOtel = me.client?.kind === "otel" && !!me.client?.ownHotelId;
  const ownId = isOtel ? me.client.ownHotelId : null;
  const L = me.limits || {}; const U = me.usage || {};
  const quotaFull = L.dailySearches > 0 && (U.searches || 0) >= L.dailySearches;

  useEffect(() => {
    api("/api/portal/hotels").then((h) => {
      setHotels(h);
      let init = param ? [param] : h.length ? [h[0].internalHotelId] : [];
      if (ownId && h.some((x) => x.internalHotelId === ownId)) init = [ownId, ...init.filter((x) => x !== ownId)];
      setSel(init);
    }).catch((e) => setErr(e.message));
  }, []);
  // OTEL turu: kendi oteli her zaman secili kalir
  const setSelLocked = (v) => setSel(ownId ? [ownId, ...v.filter((x) => x !== ownId)] : v);

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
      refreshMe && refreshMe();
    } catch (ex) { setErr(ex.data?.error || ex.message); } finally { setBusy(false); }
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
            : <ChipSelect value={sel} onChange={setSelLocked} options={hotels.map((h) => (h.internalHotelId === ownId
                ? { value: h.internalHotelId, label: <span style={{ display: "inline-flex", alignItems: "center", gap: 6, whiteSpace: "nowrap" }}><Icon name="lock" size={13} /> Otelim · {h.hotelName || h.internalHotelId}</span>, disabled: true, title: "Oteliniz her aramada yer alır" }
                : { value: h.internalHotelId, label: h.hotelName || h.internalHotelId }))} />}
          {isOtel && <p className="muted small" style={{ marginTop: 8 }}>Oteliniz her aramaya dahildir; karşılaştırmak istediğiniz rakipleri seçin.</p>}
          <FormSection>Tarih ve kişi</FormSection>
          <div style={{ marginBottom: 12 }}>
            <Segmented value={kind} onChange={setKind} options={[{ value: "prices", label: "Sadece otel" }, { value: "packages", label: "Otel + uçak + transfer", disabled: !me.plan.allowPackages }]} />
            {!me.plan.allowPackages && <span className="muted small" style={{ marginLeft: 10 }}>Otel + uçak sorgusu paketinizde kapalı.</span>}
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
          <SitePicker compact value={provs} onChange={setProvs} emptyMeansAll options={[...enabled.map((p) => ({ value: p.name, label: p.label, hint: p.domain + (p.requiresBrowser ? " · ~10 sn" : "") })), ...me.providers.filter((p) => p.maintenance).map((p) => ({ value: p.name, label: p.label, hint: p.domain, disabled: true, title: "Site geçici olarak kapalı (bakımda)", badge: <Badge tone="warn" dot={false}>bakımda</Badge> }))]} />
          <div className="sr-row center" style={{ marginTop: 20 }}>
            <Button type="submit" size="lg" icon="search" loading={busy} disabled={sel.length === 0 || quotaFull}>Fiyatları getir</Button>
            <span className="muted small">{sel.length} otel · {provs.length || enabled.length} site{L.dailySearches > 0 && ` · bugün ${U.searches || 0}/${L.dailySearches} arama`}</span>
          </div>
          {quotaFull && <div style={{ marginTop: 12 }} />}
          {quotaFull && <Alert tone="warn">Günlük arama hakkınız doldu ({L.dailySearches}). Hak her gün 00:00'da yenilenir; daha fazlası için yetkilinizle iletişime geçin.</Alert>}
        </form>
      </Card>
      {search && (
        <Card style={{ marginTop: 16 }} title="Sonuçlar" subtitle={result ? `${result.checkIn} → ${result.checkOut}` : ""} actions={<Badge tone={statusTone(search.status)}>{running ? "sorgulanıyor…" : statusLabel(search.status)}</Badge>}>
          {running && result && <Progress value={result.done || 0} max={result.total || 1} style={{ marginBottom: 14 }} />}
          {running && !result && <Progress indeterminate style={{ marginBottom: 14 }} />}
          {search.error && <Alert tone="bad">{search.error}</Alert>}
          {result && <ResultTable result={result} providers={me.providers} kind={kind} ownHotelId={ownId} />}
        </Card>
      )}
    </div>
  );
}
