import React from "react";
import { Icon } from "./Icon.jsx";
import { cx } from "./format.js";

// Field: etiket + kontrol + ipucu/hata. Kontrol child olarak verilir.
export function Field({ label, hint, error, required, className, inline, children, style }) {
  return (
    <div className={cx("sr-field", inline && "inline", className)} style={style}>
      {label && <label>{label}{required && <span style={{ color: "var(--bad)" }}> *</span>}</label>}
      {children}
      {error ? <span className="sr-err">{error}</span> : hint ? <span className="sr-hint">{hint}</span> : null}
    </div>
  );
}
export const Input = React.forwardRef(function Input({ className, invalid, icon, ...rest }, ref) {
  const el = <input ref={ref} className={cx("sr-input", invalid && "invalid", className)} {...rest} />;
  if (!icon) return el;
  return <div className="sr-input-wrap"><Icon name={icon} />{el}</div>;
});
export const Select = React.forwardRef(function Select({ className, invalid, children, ...rest }, ref) {
  return <select ref={ref} className={cx("sr-select", invalid && "invalid", className)} {...rest}>{children}</select>;
});
export const Textarea = React.forwardRef(function Textarea({ className, ...rest }, ref) {
  return <textarea ref={ref} className={cx("sr-textarea", className)} {...rest} />;
});
export function Checkbox({ label, className, ...rest }) {
  return <label className={cx("sr-check", className)}><input type="checkbox" {...rest} />{label && <span>{label}</span>}</label>;
}
export function Switch({ label, className, ...rest }) {
  return <label className={cx("sr-switch", className)}><input type="checkbox" {...rest} />{label && <span>{label}</span>}</label>;
}
// Tek secimli bolumlu dugme: options [{value,label}]
export function Segmented({ value, onChange, options, className }) {
  return (
    <div className={cx("sr-segment", className)}>
      {options.map((o) => <button type="button" key={o.value} className={o.value === value ? "on" : ""} disabled={o.disabled} onClick={() => onChange(o.value)}>{o.label}</button>)}
    </div>
  );
}
// Coklu secim chip grubu: options [{value,label,disabled,hint}], value: dizi
export function ChipSelect({ value = [], onChange, options, single = false, className }) {
  const toggle = (v) => {
    if (single) return onChange(v);
    onChange(value.includes(v) ? value.filter((x) => x !== v) : [...value, v]);
  };
  return (
    <div className={cx("sr-chips", className)}>
      {options.map((o) => {
        const on = single ? value === o.value : value.includes(o.value);
        return <span key={o.value} className={cx("sr-chip", on && "on", o.disabled && "off")} onClick={() => !o.disabled && toggle(o.value)} title={o.title || ""}>{o.label}{o.hint && <small className="muted">{o.hint}</small>}</span>;
      })}
    </div>
  );
}
export function FormGrid({ children, className, style }) { return <div className={cx("sr-form-grid", className)} style={style}>{children}</div>; }
export function FormSection({ children }) { return <div className="sr-form-section">{children}</div>; }
