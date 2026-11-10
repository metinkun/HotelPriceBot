import React, { useMemo, useState } from "react";
import { Input, Button, Badge, Icon } from "../index.js";
import { cx } from "../format.js";

/*
SitePicker: 50-100 siteye olcekli coklu secim.
  options: [{ value, label, hint?, disabled?, title?, badge? }]   value: [..]  onChange(list)
  max: secilebilecek en fazla site (0 = sinirsiz); emptyMeansAll: bos secim "tum siteler" anlamina gelir (bilgi satiri)
  compact: daha kisa karolar (modal icinde)
*/
export function SitePicker({ options = [], value = [], onChange, max = 0, emptyMeansAll = false, compact = false, readOnly = false }) {
  const [q, setQ] = useState("");
  const [onlySelected, setOnlySelected] = useState(false);
  const sel = new Set(value);
  const norm = (s) => String(s || "").toLocaleLowerCase("tr");
  const visible = useMemo(() => options.filter((o) => (!q || norm(o.label).includes(norm(q)) || norm(o.value).includes(norm(q)) || norm(o.hint).includes(norm(q))) && (!onlySelected || sel.has(o.value))), [options, q, onlySelected, value]);
  const full = max > 0 && value.length >= max;
  const toggle = (o) => {
    if (readOnly || o.disabled) return;
    if (sel.has(o.value)) return onChange(value.filter((v) => v !== o.value));
    if (full) return;
    onChange([...value, o.value]);
  };
  const selectAll = () => { const add = visible.filter((o) => !o.disabled && !sel.has(o.value)).map((o) => o.value); let next = [...value, ...add]; if (max > 0) next = next.slice(0, max); onChange(next); };
  const clear = () => onChange(value.filter((v) => !visible.some((o) => o.value === v)));
  return (
    <div className={cx("sr-sitepicker", compact && "compact")}>
      <div className="sr-sitepicker-bar">
        <Input icon="search" placeholder="Site ara…" value={q} onChange={(e) => setQ(e.target.value)} />
        <Badge tone={full ? "warn" : "info"} dot={false}>{value.length}{max > 0 ? ` / ${max}` : ""} seçili{emptyMeansAll && value.length === 0 ? " · tümü" : ""}</Badge>
        <Button size="sm" variant={onlySelected ? "soft" : "ghost"} onClick={() => setOnlySelected(!onlySelected)}>Seçililer</Button>
        {!readOnly && <><Button size="sm" variant="ghost" onClick={selectAll} disabled={full}>Tümünü seç</Button><Button size="sm" variant="ghost" onClick={clear} disabled={!value.length}>Temizle</Button></>}
      </div>
      <div className="sr-sitepicker-grid">
        {visible.map((o) => { const on = sel.has(o.value); return (
          <button type="button" key={o.value} className={cx("sr-sitetile", on && "on", (o.disabled || (!on && full) || readOnly) && "off")} onClick={() => toggle(o)} title={o.title || (full && !on ? "Site limiti dolu" : "")} aria-pressed={on}>
            <span className="sr-sitetile-check"><Icon name="check" size={13} /></span>
            <span className="sr-sitetile-text"><b>{o.label}</b>{o.hint && <small>{o.hint}</small>}</span>
            {o.badge}
          </button>); })}
        {visible.length === 0 && <div className="muted small" style={{ padding: 10 }}>Eşleşen site yok.</div>}
      </div>
    </div>
  );
}
