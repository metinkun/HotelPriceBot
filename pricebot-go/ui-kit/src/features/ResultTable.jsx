import React, { useEffect, useRef, useState } from "react";
import { Badge, Icon } from "../index.js";
import { fmtTL } from "../format.js";

const ROOM_STATUS = { available: ["ok", "müsait"], on_request: ["warn", "sorulacak"], sold_out: ["bad", "satışta değil"] };

// RoomsPanel: bir otelin tum sitelerdeki oda + pansiyon teklifleri (satir basina bir teklif).
function RoomsPanel({ hotel, cols, labelOf }) {
  const rows = [];
  for (const prov of cols) {
    const r = (hotel.result?.results || []).find((x) => x.provider === prov);
    const rooms = r?.price?.rooms || [];
    if (!rooms.length) { if (r?.price?.price != null) rows.push({ prov, lead: true, roomName: r.price.roomName || "—", boardType: r.price.boardType || "", price: r.price.price, listPrice: r.price.listPrice, discountRate: r.price.discountRate, status: r.price.available ? "available" : "sold_out", only: true }); continue; }
    rooms.forEach((rm, i) => rows.push({ prov, roomName: rm.roomName, boardType: rm.boardType, price: rm.price, listPrice: rm.listPrice, discountRate: rm.discountRate, status: rm.status, campaignName: rm.campaignName, minStayNights: rm.minStayNights, lead: i === 0 && rm.available, count: rooms.length }));
  }
  if (!rows.length) return <div className="muted small" style={{ padding: "8px 14px" }}>Bu otel için oda listesi yok.</div>;
  const best = Math.min(...rows.filter((x) => x.status === "available" && x.price != null).map((x) => x.price));
  return (
    <table className="sr-table sr-rooms">
      <thead><tr><th>Site</th><th>Oda</th><th>Pansiyon</th><th className="num">Fiyat</th><th className="num">Liste</th><th>Durum</th></tr></thead>
      <tbody>
        {rows.map((x, i) => {
          const [tone, label] = ROOM_STATUS[x.status] || ["neutral", x.status];
          return (
            <tr key={i} className={x.price === best && x.status === "available" ? "sr-best-row" : ""} style={x.status === "sold_out" ? { opacity: 0.6 } : undefined}>
              <td><b>{labelOf(x.prov)}</b>{x.only && <small className="muted"> yalnızca en uygun oda</small>}</td>
              <td>{x.roomName}{x.campaignName && <div className="sr-cell-sub" title={x.campaignName}>{x.campaignName}</div>}</td>
              <td>{x.boardType}{x.minStayNights ? <div className="sr-cell-sub">min {x.minStayNights} gece</div> : null}</td>
              <td className="num"><b>{x.price != null ? fmtTL(x.price) : "—"}</b></td>
              <td className="num muted">{x.listPrice && x.listPrice > x.price ? <>{fmtTL(x.listPrice)}{x.discountRate ? <small> %{Math.round(x.discountRate)}</small> : null}</> : "—"}</td>
              <td><Badge tone={tone} dot={false}>{label}</Badge></td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

/*
ResultTable: satir = otel, sutun = site. Fiyat aramasinda lead fiyat; pakette en ucuz oda/pansiyon. Matris oldugu icin DataTable degil.
  result: {checkIn, checkOut, hotels:[{hotelId, hotelName, status, error, result:{results:[], errors:[]}}]}
  labelOf(provider), kind: prices|packages, ownHotelId (OTEL turu: kendi oteli ustte, farklar)
  Her hucre: fiyat + musaitlik etiketi (yesil "musait" / kirmizi "musait degil"); fiyat olmayan musait-degil hucreler sadece etiket.
*/
export function ResultTable({ result, labelOf = (n) => n, kind = "prices", ownHotelId }) {
  const cols = [];
  for (const h of result.hotels || []) for (const r of (h.result?.results || [])) if (!cols.includes(r.provider)) cols.push(r.provider);
  for (const h of result.hotels || []) for (const e of (h.result?.errors || [])) if (!cols.includes(e.provider)) cols.push(e.provider);

  const cell = (h, prov) => {
    const r = (h.result?.results || []).find((x) => x.provider === prov);
    const e = (h.result?.errors || []).find((x) => x.provider === prov);
    if (e) return { text: "hata", title: e.error, cls: "bad" };
    if (!r) return h.status === "done" || h.status === "failed" ? { text: "—" } : { text: "…", cls: "wait" };
    if (kind === "packages") {
      const prices = (r.rooms || []).flatMap((rm) => rm.boards.map((b) => b.price)).filter(Boolean);
      if (!prices.length) return { text: "", avail: false, note: r.note };
      const min = Math.min(...prices);
      return { text: fmtTL(min), sub: r.roomCount + " oda", val: min, note: r.note, avail: true };
    }
    const p = r.price;
    const avail = !!(r.found && p?.available);
    const rooms = (p?.rooms || []).length;
    if (p?.price == null) return { text: "", avail: false, note: p?.note, rooms };
    return { text: fmtTL(p.price), sub: (p.boardType || "") + (p.listPrice && p.listPrice > p.price ? ` · liste ${fmtTL(p.listPrice)} (%${p.discountRate})` : ""), val: avail ? p.price : null, shown: p.price, cached: r.cached, avail, note: p.note, rooms };
  };
  const [open, setOpen] = useState({});
  const toggle = (id) => setOpen((o) => ({ ...o, [id]: !o[id] }));
  // Oda paneli yatay kaydirmada ekran disina tasmasin: kaydirma alaninin genisligine sabitlenir (sticky).
  const scrollRef = useRef(null);
  const [panelW, setPanelW] = useState(0);
  useEffect(() => {
    const el = scrollRef.current; if (!el) return;
    const upd = () => setPanelW(el.clientWidth);
    upd(); const ro = typeof ResizeObserver !== "undefined" ? new ResizeObserver(upd) : null; ro?.observe(el);
    return () => ro?.disconnect();
  }, []);
  let hotels = result.hotels || [];
  if (hotels.length === 0) return <div className="muted" style={{ padding: 12 }}>Sonuç yok.</div>;
  const own = ownHotelId ? hotels.find((h) => h.hotelId === ownHotelId) : null;
  if (own) hotels = [own, ...hotels.filter((h) => h !== own)];
  const ownCells = own ? Object.fromEntries(cols.map((c) => [c, cell(own, c)])) : {};

  return (
    <div className="sr-table-wrap">
      <div className="sr-table-scroll" ref={scrollRef}>
        <table className="sr-table">
          <thead><tr><th style={{ minWidth: 200 }}>Otel</th>{cols.map((c) => <th key={c} className="num">{labelOf(c)}</th>)}</tr></thead>
          <tbody>
            {hotels.map((h) => {
              const isOwn = own && h === own;
              const cells = cols.map((c) => cell(h, c));
              const vals = cells.map((c) => c.val).filter((v) => v != null);
              const best = vals.length ? Math.min(...vals) : null;
              const roomTotal = cells.reduce((n, c) => n + (c.rooms || 0), 0);
              const isOpen = !!open[h.hotelId];
              return (
                <React.Fragment key={h.hotelId}>
                <tr style={isOwn ? { boxShadow: "inset 3px 0 0 var(--primary)", background: "var(--primary-soft)" } : undefined} className={roomTotal ? "sr-clickable" : ""} onClick={roomTotal ? () => toggle(h.hotelId) : undefined} title={roomTotal ? "Tüm oda fiyatlarını " + (isOpen ? "gizle" : "göster") : ""}>
                  <td style={isOwn ? { borderLeft: "3px solid var(--primary)" } : undefined}>
                    <div className="sr-cell-primary sr-row center" style={{ gap: 8 }}>{roomTotal > 0 && <Icon name={isOpen ? "chevronDown" : "chevronRight"} size={16} />}{isOwn && <Badge tone="info" dot={false}>Otelim</Badge>}{h.hotelName || h.hotelId}</div>
                    <div className="sr-cell-sub">{h.status === "done" ? "tamamlandı" : h.status === "failed" ? "hata: " + (h.error || "") : "sorgulanıyor…"}{roomTotal > 0 && <> · <span className="sr-link">{roomTotal} oda teklifi</span></>}</div>
                  </td>
                  {cells.map((c, i) => {
                    const oc = own && !isOwn ? ownCells[cols[i]] : null;
                    const delta = oc && oc.val != null && c.val != null ? c.val - oc.val : null;
                    const hasAvail = c.avail !== undefined;
                    return (
                      <td key={i} className={"num sr-price-cell " + (c.val != null && c.val === best ? "sr-best" : "")} title={c.title || c.note || (delta != null ? "Otelinize göre fark" : "")}
                        style={c.cls === "bad" ? { color: "var(--bad)" } : c.cls === "wait" ? { color: "var(--muted)" } : c.avail === false ? { color: "var(--muted)" } : undefined}>
                        {c.text}{c.sub && <small>{c.sub}</small>}{c.rooms > 1 && <small className="muted">+{c.rooms - 1} oda daha</small>}{c.cached && <small>önbellek</small>}
                        {hasAvail && <small><Badge tone={c.avail ? "ok" : "bad"} dot={false} className="sr-avail">{c.avail ? "müsait" : "müsait değil"}</Badge></small>}
                        {delta != null && delta !== 0 && <small style={{ color: delta > 0 ? "var(--ok)" : "var(--bad)", fontWeight: 600 }}>{delta > 0 ? "+" : "−"}{fmtTL(Math.abs(delta))}</small>}
                        {delta === 0 && <small style={{ color: "var(--muted)" }}>aynı fiyat</small>}
                      </td>
                    );
                  })}
                </tr>
                {isOpen && <tr className="sr-rooms-row"><td colSpan={cols.length + 1} style={{ padding: 0, background: "var(--surface-2)" }}><div className="sr-rooms-wrap" style={panelW ? { position: "sticky", left: 0, width: panelW } : undefined}><RoomsPanel hotel={h} cols={cols} labelOf={labelOf} /></div></td></tr>}
                </React.Fragment>
              );
            })}
          </tbody>
        </table>
      </div>
      <div className="muted small" style={{ padding: "8px 14px", borderTop: "1px solid var(--line)" }}>
        Hücrede en uygun müsait oda gösterilir; otel satırına tıklayınca her sitenin listelediği tüm oda + pansiyon fiyatları açılır. Her hücrede sitenin verdiği fiyat ve müsaitlik durumu gösterilir; <span style={{ color: "var(--ok)" }}>yeşil</span> müsait, <span style={{ color: "var(--bad)" }}>kırmızı</span> müsait değil (fiyat yine de sitede yazıyorsa gösterilir). En ucuz müsait fiyat vurgulanır.
        {own && <> Rakip hücrelerindeki fark otelinizin aynı sitedeki fiyatına göredir: <span style={{ color: "var(--ok)" }}>yeşil</span> rakip daha pahalı, <span style={{ color: "var(--bad)" }}>kırmızı</span> rakip daha ucuz.</>}
      </div>
    </div>
  );
}
