import React from "react";
import { Icon } from "./Icon.jsx";
import { cx } from "./format.js";

export function Card({ title, subtitle, actions, className, flat, pad0, children, style }) {
  return (
    <section className={cx("sr-card", flat && "flat", pad0 && "pad-0", className)} style={style}>
      {(title || actions) && (
        <div className="sr-card-head" style={pad0 ? { padding: "16px 20px 0" } : undefined}>
          <div>{title && <h3>{title}</h3>}{subtitle && <p>{subtitle}</p>}</div>
          {actions && <div className="sr-row center" style={{ gap: 8 }}>{actions}</div>}
        </div>
      )}
      {children}
    </section>
  );
}
export function Stat({ icon, label, value, unit, sub, soft, onClick }) {
  return (
    <div className="sr-stat" onClick={onClick} style={onClick ? { cursor: "pointer" } : undefined}>
      {icon && <div className={cx("sr-stat-ic", soft && "soft")}><Icon name={icon} size={20} /></div>}
      <div style={{ minWidth: 0 }}><div className="k">{label}</div><div className="v">{value}{unit && <small>{unit}</small>}</div>{sub && <div className="sub">{sub}</div>}</div>
    </div>
  );
}
export function PageHeader({ title, description, actions, children }) {
  return (
    <div className="sr-page-head">
      <div><h1>{title}</h1>{description && <p>{description}</p>}{children}</div>
      {actions && <div className="sr-actions">{actions}</div>}
    </div>
  );
}
export function EmptyState({ icon = "list", title, text, action }) {
  return (
    <div className="sr-empty">
      <div className="sr-empty-ic"><Icon name={icon} size={22} /></div>
      {title && <b>{title}</b>}{text && <div>{text}</div>}{action && <div style={{ marginTop: 12 }}>{action}</div>}
    </div>
  );
}
export function Alert({ tone = "info", children, className }) {
  const ic = { bad: "xCircle", ok: "checkCircle", warn: "alert", info: "info" }[tone];
  return <div className={cx("sr-alert", tone, className)}><Icon name={ic} size={16} /><div>{children}</div></div>;
}
export function Badge({ tone = "neutral", dot = true, children, className, title }) {
  return <span className={cx("sr-badge", tone, !dot && "nodot", className)} title={title}>{children}</span>;
}
export function Progress({ value, max = 100, indeterminate, style }) {
  return <div className={cx("sr-progress", indeterminate && "indeterminate")} style={style}><i style={{ width: indeterminate ? undefined : Math.min(100, (100 * (value || 0)) / (max || 1)) + "%" }} /></div>;
}
export function Skeleton({ w = "100%", h = 14, style }) { return <div className="sr-skel" style={{ width: w, height: h, ...style }} />; }
export function Avatar({ name, size = 34 }) {
  const ini = (name || "?").split(/\s+/).filter(Boolean).slice(0, 2).map((w) => w[0].toUpperCase()).join("");
  return <div className="sr-avatar" style={{ width: size, height: size, fontSize: Math.round(size * 0.38) }}>{ini || "?"}</div>;
}
export function KeyBox({ label, value }) { return <div className="sr-keybox">{label && <b>{label}</b>}{value}</div>; }
export function Tabs({ value, onChange, items, className }) {
  return (
    <div className={cx("sr-tabs", className)}>
      {items.map((t) => <button key={t.value} type="button" className={t.value === value ? "on" : ""} onClick={() => onChange(t.value)}>{t.icon && <Icon name={t.icon} size={15} />}{t.label}{t.count != null && <span className="sr-count">{t.count}</span>}</button>)}
    </div>
  );
}
export function Chip({ on, off, dot, onClick, children, title, className }) {
  return <span className={cx("sr-chip", on && "on", off && "off", !onClick && "static", className)} onClick={onClick} title={title}>{dot && <span className="sr-dot" />}{children}</span>;
}
export function Divider() { return <div className="sr-divider" />; }
export function KV({ items }) {
  return <dl className="sr-kv">{items.filter((i) => i && i[1] != null && i[1] !== "").map(([k, v]) => <React.Fragment key={k}><dt>{k}</dt><dd>{v}</dd></React.Fragment>)}</dl>;
}
