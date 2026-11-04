import React from "react";
import { Icon } from "./Icon.jsx";
import { cx } from "./format.js";

// variant: primary | secondary | soft | ghost | danger | gradient ; size: sm | md | lg
export function Button({ variant = "primary", size = "md", icon, iconRight, loading, block, className, children, type = "button", ...rest }) {
  return (
    <button type={type} className={cx("sr-btn", variant !== "primary" && variant, size !== "md" && size, block && "block", className)} disabled={loading || rest.disabled} {...rest}>
      {loading ? <span className="sr-spin" /> : icon ? <Icon name={icon} /> : null}
      {children}
      {iconRight && !loading && <Icon name={iconRight} />}
    </button>
  );
}

// Satir aksiyonlari ve arac cubugu icin ikon dugmesi. title zorunlu (erisilebilirlik + tooltip).
export function IconButton({ icon, title, tone, size, className, label, ...rest }) {
  return (
    <button type="button" className={cx("sr-ibtn", tone, size, className)} title={title} aria-label={title} {...rest}>
      <Icon name={icon} />{label && <span className="sr-ibtn-label">{label}</span>}
    </button>
  );
}
