import React from "react";
import { ResultTable as KitResultTable } from "@kit";

// Ortak sonuc matrisi (ui-kit); site etiketleri me.providers'tan.
export default function ResultTable({ result, providers = [], kind, ownHotelId }) {
  const labelOf = (n) => (providers.find((p) => p.name === n) || {}).label || n;
  return <KitResultTable result={result} labelOf={labelOf} kind={kind} ownHotelId={ownHotelId} />;
}
