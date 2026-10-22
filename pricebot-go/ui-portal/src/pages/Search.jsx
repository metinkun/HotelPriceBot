import React, { useEffect, useState } from "react";
import { api, addDays } from "../api.js";
import ResultTable from "./ResultTable.jsx";

export default function Search({ me }) {
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
    api("/api/portal/hotels").then((h) => { setHotels(h); const pre = (location.hash.split("/")[1] || ""); if (pre) setSel([pre]); else if (h.length) setSel([h[0].internalHotelId]); }).catch((e) => setErr(e.message));
  }, []);

  // Sonuc bitene kadar sorgula
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
  const toggle = (arr, set, v) => set(arr.includes(v) ? arr.filter((x) => x !== v) : [...arr, v]);
  const result = search?.result;

  return (
    <div>
      {err && <div className="err">{err}</div>}
      <div className="card">
        <form onSubmit={run}>
          <h3>Oteller</h3>
          {hotels.length === 0 && <div className="muted">Önce <a href="#hotels">otel ekleyin</a>.</div>}
          <div className="chips">{hotels.map((h) => <span key={h.internalHotelId} className={"chip " + (sel.includes(h.internalHotelId) ? "on" : "")} onClick={() => toggle(sel, setSel, h.internalHotelId)}>{h.hotelName || h.internalHotelId}</span>)}</div>
          <h3 style={{ marginTop: 16 }}>Tarih ve kişi</h3>
          <div className="row">
            <div><label>Tür</label><select value={kind} onChange={(e) => setKind(e.target.value)}><option value="prices">Sadece otel</option><option value="packages" disabled={!me.plan.allowPackages}>Otel + uçak + transfer</option></select></div>
            <div><label>Giriş</label><input type="date" value={checkIn} onChange={(e) => setCheckIn(e.target.value)} /></div>
            <div><label>Çıkış</label><input type="date" value={checkOut} onChange={(e) => setCheckOut(e.target.value)} /></div>
            <div><label>Yetişkin</label><input type="number" min="1" max="8" value={adults} onChange={(e) => setAdults(e.target.value)} style={{ width: 90 }} /></div>
            <div><label>Çocuk</label><input type="number" min="0" max="6" value={children} onChange={(e) => setChildren(e.target.value)} style={{ width: 90 }} /></div>
            {Number(children) > 0 && <div><label>Çocuk yaşları</label><input placeholder="5,8" value={childAges} onChange={(e) => setChildAges(e.target.value)} style={{ width: 110 }} /></div>}
            {kind === "packages" && <div><label>Kalkış (IATA)</label><input value={airport} onChange={(e) => setAirport(e.target.value.toUpperCase())} maxLength={3} style={{ width: 90 }} /></div>}
          </div>
          <h3 style={{ marginTop: 16 }}>Siteler <span className="muted" style={{ fontWeight: 400 }}>(boş = tüm etkin siteler)</span></h3>
          <div className="chips">{enabled.map((p) => <span key={p.name} className={"chip " + (provs.includes(p.name) ? "on" : "")} onClick={() => toggle(provs, setProvs, p.name)}>{p.label}{p.requiresBrowser && <small className="muted"> ~10 sn</small>}</span>)}</div>
          <button className="btn" style={{ marginTop: 16 }} disabled={busy || sel.length === 0}>{busy ? "Başlatılıyor…" : "Fiyatları getir"}</button>
        </form>
      </div>
      {search && (
        <div className="card">
          <div className="row" style={{ justifyContent: "space-between" }}><h3>Sonuçlar <span className="muted" style={{ fontWeight: 400 }}>{result?.checkIn} → {result?.checkOut}</span></h3>
            <span className={"badge " + (search.status === "done" ? "ok" : search.status === "failed" ? "bad" : "warn")}>{search.status === "done" ? "tamamlandı" : search.status === "failed" ? "hata" : "sorgulanıyor…"}</span></div>
          {result && <div className="progress" style={{ margin: "8px 0 14px" }}><i style={{ width: (100 * (result.done || 0) / (result.total || 1)) + "%" }} /></div>}
          {search.error && <div className="err">{search.error}</div>}
          {result && <ResultTable result={result} providers={me.providers} kind={kind} />}
        </div>
      )}
    </div>
  );
}
