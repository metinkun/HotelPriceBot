import React, { useMemo, useState } from "react";
import { Icon } from "./Icon.jsx";
import { Button, IconButton } from "./Button.jsx";
import { Input, Select } from "./Form.jsx";
import { EmptyState, Skeleton } from "./Card.jsx";
import { cx } from "./format.js";

/*
DataTable: duzenli liste tablosu.
  columns: [{ key, title, render?(row), align?: 'num'|'center', width?, primary?: bool (mobil kart basligi), sub?(row) (ikinci satir),
              hideOnMobile?: bool, sortable?: bool, sortValue?(row), filterValue?(row), filter?: false (sutun filtresi yok) }]
  rows, rowKey(row)
  actions(row) -> [{ icon, title, onClick, tone?: 'danger'|'primary', hidden?: bool, disabled?: bool }]
  onRowDoubleClick(row), onRowClick(row), selectedKey
  search: true (varsayilan; tum sutunlarda genel arama) | { placeholder, keys?: [..] } | false
  filters: true (varsayilan; "Filtreler" dugmesiyle sutun basina filtre satiri) | false
  toolbar: ek filtreler (sol), toolbarRight
  loading, empty: { icon, title, text, action }, dense, footer (node)
  Mobilde (<=860px) satirlar karta donusur, aksiyonlar kartin altinda.
*/

// React dugumunden aranabilir duz metin (render ciktisi icin).
export function nodeText(n) {
  if (n == null || n === false) return "";
  if (typeof n === "string" || typeof n === "number") return String(n);
  if (Array.isArray(n)) return n.map(nodeText).join(" ");
  if (typeof n === "object") {
    const p = n.props || {};
    return [p.title, p.value, nodeText(p.children)].filter((x) => x != null && x !== "").join(" ");
  }
  return "";
}
const norm = (s) => String(s ?? "").toLocaleLowerCase("tr").trim();

export function DataTable({ columns, rows = [], rowKey, actions, onRowDoubleClick, onRowClick, selectedKey, search = true, filters = true, toolbar, toolbarRight, loading, empty, dense, footer, maxHeight, className, pageSize = 25 }) {
  const [q, setQ] = useState("");
  const [sort, setSort] = useState(null); // {key, dir}
  const [page, setPage] = useState(0);
  const [showFilters, setShowFilters] = useState(false);
  const [colFilters, setColFilters] = useState({});
  const key = rowKey || ((r, i) => r.id ?? i);
  const cell = (c, r) => (c.render ? c.render(r) : r[c.key]);
  // Sutunun aranabilir metni: filterValue > sortValue > render ciktisi > ham deger
  const colText = (c, r) => {
    if (c.filterValue) return norm(c.filterValue(r));
    if (c.sortValue) { const v = c.sortValue(r); if (typeof v === "string" || typeof v === "number") return norm(v); }
    if (c.render) return norm(nodeText(c.render(r)) + (c.sub ? " " + nodeText(c.sub(r)) : ""));
    return norm(r[c.key]) + (c.sub ? " " + norm(nodeText(c.sub(r))) : "");
  };
  const searchOn = search !== false && search != null;
  const searchKeys = searchOn && typeof search === "object" ? search.keys : null;

  // Sutun basina ayirt edici degerler (az sayida ise secim kutusu)
  const distinct = useMemo(() => {
    if (!showFilters) return {};
    const out = {};
    for (const c of columns) {
      if (c.filter === false) continue;
      const set = new Map();
      for (const r of rows) { const t = colText(c, r); if (t) set.set(t, (set.get(t) || 0) + 1); if (set.size > 14) break; }
      if (set.size >= 2 && set.size <= 14 && rows.length > 3) out[c.key] = [...set.keys()].sort();
    }
    return out;
  }, [showFilters, rows, columns]);

  const filtered = useMemo(() => {
    let out = rows;
    if (searchOn && q.trim()) {
      const needle = norm(q);
      out = out.filter((r) => {
        if (searchKeys) return searchKeys.some((k) => { const v = typeof k === "function" ? k(r) : r[k]; return v != null && norm(v).includes(needle); });
        return columns.some((c) => colText(c, r).includes(needle));
      });
    }
    const active = Object.entries(colFilters).filter(([, v]) => v);
    if (active.length) {
      out = out.filter((r) => active.every(([k, v]) => { const c = columns.find((x) => x.key === k); if (!c) return true; const t = colText(c, r); return distinct[k] ? t === v : t.includes(norm(v)); }));
    }
    if (sort) {
      const col = columns.find((c) => c.key === sort.key);
      const val = (r) => (col?.sortValue ? col.sortValue(r) : col?.render && typeof r[sort.key] === "undefined" ? nodeText(col.render(r)) : r[sort.key]);
      out = [...out].sort((a, b) => { const x = val(a), y = val(b); if (x == null) return 1; if (y == null) return -1; return (x < y ? -1 : x > y ? 1 : 0) * (sort.dir === "asc" ? 1 : -1); });
    }
    return out;
  }, [rows, q, sort, colFilters, columns, distinct]);

  const pages = pageSize ? Math.max(1, Math.ceil(filtered.length / pageSize)) : 1;
  const cur = Math.min(page, pages - 1);
  const visible = pageSize ? filtered.slice(cur * pageSize, (cur + 1) * pageSize) : filtered;
  const onSearch = (v) => { setQ(v); setPage(0); };
  const setColFilter = (k, v) => { setColFilters((f) => ({ ...f, [k]: v })); setPage(0); };
  const activeFilters = Object.values(colFilters).filter(Boolean).length;
  const clearFilters = () => { setColFilters({}); setQ(""); setPage(0); };
  const toggleSort = (c) => { if (!c.sortable) return; setSort((s) => (s?.key === c.key ? (s.dir === "asc" ? { key: c.key, dir: "desc" } : null) : { key: c.key, dir: "asc" })); };
  const acts = (r) => (actions ? actions(r).filter((a) => a && !a.hidden) : []);
  const primaryCol = columns.find((c) => c.primary) || columns[0];
  const hasToolbar = searchOn || filters || toolbar || toolbarRight;
  const filtering = q.trim() || activeFilters > 0;

  return (
    <div className={cx("sr-table-wrap", className)}>
      {hasToolbar && (
        <div className="sr-table-toolbar">
          {searchOn && <Input icon="search" placeholder={(typeof search === "object" && search.placeholder) || "Tabloda ara…"} value={q} onChange={(e) => onSearch(e.target.value)} />}
          {filters && <Button size="sm" variant={showFilters || activeFilters ? "soft" : "secondary"} icon="filter" onClick={() => setShowFilters(!showFilters)}>Filtreler{activeFilters ? ` (${activeFilters})` : ""}</Button>}
          {filtering && <Button size="sm" variant="ghost" icon="x" onClick={clearFilters}>Temizle</Button>}
          {toolbar}
          <div className="sr-spacer" />
          {toolbarRight}
        </div>
      )}
      <div className="sr-table-scroll cardify" style={maxHeight ? { maxHeight, overflowY: "auto" } : undefined}>
        <table className={cx("sr-table", dense && "dense")}>
          <thead>
            <tr>
              {columns.map((c) => <th key={c.key} className={cx(c.align, c.sortable && "sortable")} style={{ width: c.width, cursor: c.sortable ? "pointer" : undefined }} onClick={() => toggleSort(c)}>
                <span style={{ display: "inline-flex", alignItems: "center", gap: 4 }}>{c.title}{sort?.key === c.key && <Icon name="chevronDown" size={12} style={{ transform: sort.dir === "asc" ? "rotate(180deg)" : "none" }} />}</span>
              </th>)}
              {actions && <th className="actions"><span className="sr-only">İşlemler</span></th>}
            </tr>
            {showFilters && (
              <tr className="sr-filter-row">
                {columns.map((c) => <th key={c.key} className={cx(c.align)}>
                  {c.filter === false ? null : distinct[c.key]
                    ? <Select value={colFilters[c.key] || ""} onChange={(e) => setColFilter(c.key, e.target.value)}><option value="">Tümü</option>{distinct[c.key].map((v) => <option key={v} value={v}>{v}</option>)}</Select>
                    : <Input value={colFilters[c.key] || ""} onChange={(e) => setColFilter(c.key, e.target.value)} placeholder="Filtrele…" />}
                </th>)}
                {actions && <th className="actions" />}
              </tr>
            )}
          </thead>
          <tbody>
            {loading && rows.length === 0 && [0, 1, 2].map((i) => <tr key={"sk" + i}>{columns.map((c) => <td key={c.key}><Skeleton w={c.key === primaryCol.key ? "70%" : "45%"} /></td>)}{actions && <td />}</tr>)}
            {!loading && filtered.length === 0 && <tr><td colSpan={columns.length + (actions ? 1 : 0)}><EmptyState icon={empty?.icon} title={filtering ? "Sonuç bulunamadı" : (empty?.title || "Kayıt yok")} text={filtering ? "Arama veya filtreleri değiştirin." : empty?.text} action={filtering ? <Button size="sm" variant="secondary" onClick={clearFilters}>Filtreleri temizle</Button> : empty?.action} /></td></tr>}
            {visible.map((r, i) => {
              const k = key(r, i);
              return (
                <tr key={k} className={cx((onRowDoubleClick || onRowClick) && "clickable", selectedKey != null && selectedKey === k && "selected")} onDoubleClick={onRowDoubleClick ? () => onRowDoubleClick(r) : undefined} onClick={onRowClick ? () => onRowClick(r) : undefined} title={onRowDoubleClick ? "Detay için çift tıklayın" : undefined}>
                  {columns.map((c) => <td key={c.key} className={cx(c.align)}>{c.primary || c.sub ? <div><div className={c.primary ? "sr-cell-primary" : ""}>{cell(c, r)}</div>{c.sub && <div className="sr-cell-sub">{c.sub(r)}</div>}</div> : cell(c, r)}</td>)}
                  {actions && <td className="actions" onClick={(e) => e.stopPropagation()} onDoubleClick={(e) => e.stopPropagation()}><div className="sr-actions">{acts(r).map((a) => <IconButton key={a.title} icon={a.icon} title={a.title} tone={a.tone} disabled={a.disabled} onClick={() => a.onClick(r)} />)}</div></td>}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      <div className="sr-cards-mobile">
        {!loading && filtered.length === 0 && <EmptyState icon={empty?.icon} title={filtering ? "Sonuç bulunamadı" : (empty?.title || "Kayıt yok")} text={empty?.text} action={empty?.action} />}
        {visible.map((r, i) => {
          const k = key(r, i);
          return (
            <div key={k} className={cx("sr-mcard", selectedKey != null && selectedKey === k && "selected")} onClick={onRowClick ? () => onRowClick(r) : onRowDoubleClick ? () => onRowDoubleClick(r) : undefined}>
              <div className="sr-mcard-top"><div><div className="sr-cell-primary">{cell(primaryCol, r)}</div>{primaryCol.sub && <div className="sr-cell-sub">{primaryCol.sub(r)}</div>}</div></div>
              <div className="sr-mcard-grid">
                {columns.filter((c) => c !== primaryCol && !c.hideOnMobile).map((c) => <div key={c.key}><div className="k">{c.title}</div><div>{cell(c, r) ?? "—"}</div></div>)}
              </div>
              {actions && acts(r).length > 0 && <div className="sr-actions" onClick={(e) => e.stopPropagation()}>{acts(r).map((a) => <IconButton key={a.title} icon={a.icon} title={a.title} label={a.title} tone={a.tone} disabled={a.disabled} onClick={() => a.onClick(r)} />)}</div>}
            </div>
          );
        })}
      </div>
      {(footer || pages > 1 || rows.length > 0) && (
        <div className="sr-table-foot">
          <span>{filtered.length === rows.length ? `${rows.length} kayıt` : `${filtered.length} / ${rows.length} kayıt`}{pages > 1 && ` · sayfa ${cur + 1}/${pages}`}</span>
          <span className="sr-row center" style={{ gap: 6 }}>{footer}{pages > 1 && <><IconButton icon="chevronLeft" title="Önceki sayfa" disabled={cur === 0} onClick={() => setPage(cur - 1)} /><IconButton icon="chevronRight" title="Sonraki sayfa" disabled={cur >= pages - 1} onClick={() => setPage(cur + 1)} /></>}</span>
        </div>)}
    </div>
  );
}
