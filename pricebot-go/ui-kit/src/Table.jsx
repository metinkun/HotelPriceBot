import React, { useMemo, useState } from "react";
import { Icon } from "./Icon.jsx";
import { IconButton } from "./Button.jsx";
import { Input } from "./Form.jsx";
import { EmptyState, Skeleton } from "./Card.jsx";
import { cx } from "./format.js";

/*
DataTable: duzenli liste tablosu.
  columns: [{ key, title, render?(row), align?: 'num'|'center', width?, primary?: bool (mobil kart basligi), sub?(row) (ikinci satir), hideOnMobile?: bool, sortable?: bool, sortValue?(row) }]
  rows, rowKey(row)
  actions(row) -> [{ icon, title, onClick, tone?: 'danger'|'primary', hidden?: bool, disabled?: bool }]
  onRowDoubleClick(row), onRowClick(row), selectedKey
  search: { placeholder, keys?: [..] } -> yerel arama kutusu; toolbar: ek filtreler (sol), toolbarRight
  loading, empty: { icon, title, text, action }, dense, footer (node) 
  Mobilde (<=860px) satirlar karta donusur, aksiyonlar kartin altinda.
*/
export function DataTable({ columns, rows = [], rowKey, actions, onRowDoubleClick, onRowClick, selectedKey, search, toolbar, toolbarRight, loading, empty, dense, footer, maxHeight, className, pageSize = 25 }) {
  const [q, setQ] = useState("");
  const [sort, setSort] = useState(null); // {key, dir}
  const [page, setPage] = useState(0);
  const key = rowKey || ((r, i) => r.id ?? i);

  const filtered = useMemo(() => {
    let out = rows;
    if (search && q.trim()) {
      const needle = q.trim().toLocaleLowerCase("tr");
      const keys = search.keys;
      out = rows.filter((r) => {
        const hay = keys ? keys.map((k) => (typeof k === "function" ? k(r) : r[k])) : Object.values(r);
        return hay.some((v) => v != null && String(v).toLocaleLowerCase("tr").includes(needle));
      });
    }
    if (sort) {
      const col = columns.find((c) => c.key === sort.key);
      const val = (r) => (col?.sortValue ? col.sortValue(r) : r[sort.key]);
      out = [...out].sort((a, b) => { const x = val(a), y = val(b); if (x == null) return 1; if (y == null) return -1; return (x < y ? -1 : x > y ? 1 : 0) * (sort.dir === "asc" ? 1 : -1); });
    }
    return out;
  }, [rows, q, sort, search, columns]);

  const pages = pageSize ? Math.max(1, Math.ceil(filtered.length / pageSize)) : 1;
  const cur = Math.min(page, pages - 1);
  const visible = pageSize ? filtered.slice(cur * pageSize, (cur + 1) * pageSize) : filtered;
  const onSearch = (v) => { setQ(v); setPage(0); };
  const toggleSort = (c) => { if (!c.sortable) return; setSort((s) => (s?.key === c.key ? (s.dir === "asc" ? { key: c.key, dir: "desc" } : null) : { key: c.key, dir: "asc" })); };
  const cell = (c, r) => (c.render ? c.render(r) : r[c.key]);
  const acts = (r) => (actions ? actions(r).filter((a) => a && !a.hidden) : []);
  const primaryCol = columns.find((c) => c.primary) || columns[0];
  const hasToolbar = search || toolbar || toolbarRight;

  return (
    <div className={cx("sr-table-wrap", className)}>
      {hasToolbar && (
        <div className="sr-table-toolbar">
          {search && <Input icon="search" placeholder={search.placeholder || "Ara…"} value={q} onChange={(e) => onSearch(e.target.value)} />}
          {toolbar}
          <div className="sr-spacer" />
          {toolbarRight}
        </div>
      )}
      <div className="sr-table-scroll cardify" style={maxHeight ? { maxHeight, overflowY: "auto" } : undefined}>
        <table className={cx("sr-table", dense && "dense")}>
          <thead><tr>
            {columns.map((c) => <th key={c.key} className={cx(c.align, c.sortable && "sortable")} style={{ width: c.width, cursor: c.sortable ? "pointer" : undefined }} onClick={() => toggleSort(c)}>
              <span style={{ display: "inline-flex", alignItems: "center", gap: 4 }}>{c.title}{sort?.key === c.key && <Icon name={sort.dir === "asc" ? "chevronDown" : "chevronRight"} size={12} style={{ transform: sort.dir === "asc" ? "rotate(180deg)" : "rotate(90deg)" }} />}</span>
            </th>)}
            {actions && <th className="actions"><span className="sr-only">İşlemler</span></th>}
          </tr></thead>
          <tbody>
            {loading && rows.length === 0 && [0, 1, 2].map((i) => <tr key={"sk" + i}>{columns.map((c) => <td key={c.key}><Skeleton w={c.key === primaryCol.key ? "70%" : "45%"} /></td>)}{actions && <td />}</tr>)}
            {!loading && filtered.length === 0 && <tr><td colSpan={columns.length + (actions ? 1 : 0)}><EmptyState icon={empty?.icon} title={empty?.title || (q ? "Sonuç bulunamadı" : "Kayıt yok")} text={empty?.text} action={!q && empty?.action} /></td></tr>}
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
        {!loading && filtered.length === 0 && <EmptyState icon={empty?.icon} title={empty?.title || "Kayıt yok"} text={empty?.text} action={empty?.action} />}
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
      {(footer || pages > 1 || (search && rows.length > 0)) && (
        <div className="sr-table-foot">
          <span>{filtered.length === rows.length ? `${rows.length} kayıt` : `${filtered.length} / ${rows.length} kayıt`}{pages > 1 && ` · sayfa ${cur + 1}/${pages}`}</span>
          <span className="sr-row center" style={{ gap: 6 }}>{footer}{pages > 1 && <><IconButton icon="chevronLeft" title="Önceki sayfa" disabled={cur === 0} onClick={() => setPage(cur - 1)} /><IconButton icon="chevronRight" title="Sonraki sayfa" disabled={cur >= pages - 1} onClick={() => setPage(cur + 1)} /></>}</span>
        </div>)}
    </div>
  );
}
