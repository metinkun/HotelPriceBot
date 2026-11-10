import React from "react";
import { Badge } from "../index.js";
import { fmtTL } from "../format.js";

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
    if (p?.price == null) return { text: "", avail: false, note: p?.note };
    return { text: fmtTL(p.price), sub: (p.boardType || "") + (p.listPrice && p.listPrice > p.price ? ` · liste ${fmtTL(p.listPrice)} (%${p.discountRate})` : ""), val: avail ? p.price : null, shown: p.price, cached: r.cached, avail, note: p.note };
  };
  let hotels = result.hotels || [];
  if (hotels.length === 0) return <div className="muted" style={{ padding: 12 }}>Sonuç yok.</div>;
  const own = ownHotelId ? hotels.find((h) => h.hotelId === ownHotelId) : null;
  if (own) hotels = [own, ...hotels.filter((h) => h !== own)];
  const ownCells = own ? Object.fromEntries(cols.map((c) => [c, cell(own, c)])) : {};

  return (
    <div className="sr-table-wrap">
      <div className="sr-table-scroll">
        <table className="sr-table">
          <thead><tr><th style={{ minWidth: 200 }}>Otel</th>{cols.map((c) => <th key={c} className="num">{labelOf(c)}</th>)}</tr></thead>
          <tbody>
            {hotels.map((h) => {
              const isOwn = own && h === own;
              const cells = cols.map((c) => cell(h, c));
              const vals = cells.map((c) => c.val).filter((v) => v != null);
              const best = vals.length ? Math.min(...vals) : null;
              return (
                <tr key={h.hotelId} style={isOwn ? { boxShadow: "inset 3px 0 0 var(--primary)", background: "var(--primary-soft)" } : undefined}>
                  <td style={isOwn ? { borderLeft: "3px solid var(--primary)" } : undefined}>
                    <div className="sr-cell-primary sr-row center" style={{ gap: 8 }}>{isOwn && <Badge tone="info" dot={false}>Otelim</Badge>}{h.hotelName || h.hotelId}</div>
                    <div className="sr-cell-sub">{h.status === "done" ? "tamamlandı" : h.status === "failed" ? "hata: " + (h.error || "") : "sorgulanıyor…"}</div>
                  </td>
                  {cells.map((c, i) => {
                    const oc = own && !isOwn ? ownCells[cols[i]] : null;
                    const delta = oc && oc.val != null && c.val != null ? c.val - oc.val : null;
                    const hasAvail = c.avail !== undefined;
                    return (
                      <td key={i} className={"num sr-price-cell " + (c.val != null && c.val === best ? "sr-best" : "")} title={c.title || c.note || (delta != null ? "Otelinize göre fark" : "")}
                        style={c.cls === "bad" ? { color: "var(--bad)" } : c.cls === "wait" ? { color: "var(--muted)" } : c.avail === false ? { color: "var(--muted)" } : undefined}>
                        {c.text}{c.sub && <small>{c.sub}</small>}{c.cached && <small>önbellek</small>}
                        {hasAvail && <small><Badge tone={c.avail ? "ok" : "bad"} dot={false} className="sr-avail">{c.avail ? "müsait" : "müsait değil"}</Badge></small>}
                        {delta != null && delta !== 0 && <small style={{ color: delta > 0 ? "var(--ok)" : "var(--bad)", fontWeight: 600 }}>{delta > 0 ? "+" : "−"}{fmtTL(Math.abs(delta))}</small>}
                        {delta === 0 && <small style={{ color: "var(--muted)" }}>aynı fiyat</small>}
                      </td>
                    );
                  })}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      <div className="muted small" style={{ padding: "8px 14px", borderTop: "1px solid var(--line)" }}>
        Her hücrede sitenin verdiği fiyat ve müsaitlik durumu gösterilir; <span style={{ color: "var(--ok)" }}>yeşil</span> müsait, <span style={{ color: "var(--bad)" }}>kırmızı</span> müsait değil (fiyat yine de sitede yazıyorsa gösterilir). En ucuz müsait fiyat vurgulanır.
        {own && <> Rakip hücrelerindeki fark otelinizin aynı sitedeki fiyatına göredir: <span style={{ color: "var(--ok)" }}>yeşil</span> rakip daha pahalı, <span style={{ color: "var(--bad)" }}>kırmızı</span> rakip daha ucuz.</>}
      </div>
    </div>
  );
}
