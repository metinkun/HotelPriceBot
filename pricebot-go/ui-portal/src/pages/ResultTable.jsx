import React from "react";
import { fmtTL } from "../api.js";

// Satir = otel, sutun = site. Fiyat aramasinda lead fiyat; pakette en ucuz oda/pansiyon.
export default function ResultTable({ result, providers, kind }) {
  const label = (n) => (providers.find((p) => p.name === n) || {}).label || n;
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
      if (!prices.length) return { text: "paket yok" };
      const min = Math.min(...prices);
      return { text: fmtTL(min), sub: r.roomCount + " oda", val: min, note: r.note };
    }
    const p = r.price;
    if (!r.found || !p?.available || p.price == null) return { text: "müsait değil" };
    return { text: fmtTL(p.price), sub: (p.boardType || "") + (p.listPrice && p.listPrice > p.price ? ` · liste ${fmtTL(p.listPrice)} (%${p.discountRate})` : ""), val: p.price, cached: r.cached };
  };

  return (
    <table>
      <thead><tr><th>Otel</th>{cols.map((c) => <th key={c}>{label(c)}</th>)}</tr></thead>
      <tbody>
        {(result.hotels || []).map((h) => {
          const cells = cols.map((c) => cell(h, c));
          const vals = cells.map((c) => c.val).filter((v) => v != null);
          const best = vals.length ? Math.min(...vals) : null;
          return (
            <tr key={h.hotelId}>
              <td><b>{h.hotelName || h.hotelId}</b><div className="muted" style={{ fontSize: 12 }}>{h.status === "done" ? "tamam" : h.status === "failed" ? "hata: " + (h.error || "") : "sorgulanıyor…"}</div></td>
              {cells.map((c, i) => <td key={i} className={"price-cell " + (c.val != null && c.val === best ? "best" : "")} title={c.title || c.note || ""}>{c.text}{c.sub && <small>{c.sub}</small>}{c.cached && <small>önbellek</small>}</td>)}
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}
