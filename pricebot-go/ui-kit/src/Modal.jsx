import React, { createContext, useCallback, useContext, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Button, IconButton } from "./Button.jsx";
import { Field, Input } from "./Form.jsx";
import { cx } from "./format.js";

// Modal: kisa formlar ve onaylar. size: sm | md | lg. Mobilde alt sayfa (sheet) olur.
export function Modal({ open, onClose, title, subtitle, size, footer, children, closeOnBackdrop = true }) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e) => { if (e.key === "Escape") onClose?.(); };
    document.addEventListener("keydown", onKey);
    const prev = document.body.style.overflow; document.body.style.overflow = "hidden";
    return () => { document.removeEventListener("keydown", onKey); document.body.style.overflow = prev; };
  }, [open, onClose]);
  if (!open) return null;
  return createPortal(
    <div className="sr-modal-backdrop" onMouseDown={(e) => { if (closeOnBackdrop && e.target === e.currentTarget) onClose?.(); }}>
      <div className={cx("sr-modal", size)} role="dialog" aria-modal="true" aria-label={typeof title === "string" ? title : undefined}>
        <div className="sr-modal-head"><div><h2>{title}</h2>{subtitle && <p>{subtitle}</p>}</div><IconButton icon="x" title="Kapat" onClick={onClose} /></div>
        <div className="sr-modal-body">{children}</div>
        {footer && <div className="sr-modal-foot">{footer}</div>}
      </div>
    </div>, document.body);
}

// FormModal: submit'li modal; onSubmit async olabilir, hata gosterir.
export function FormModal({ open, onClose, title, subtitle, size, submitText = "Kaydet", cancelText = "Vazgeç", onSubmit, busy, error, children, danger, extraFooter }) {
  const [localErr, setLocalErr] = useState("");
  const [localBusy, setLocalBusy] = useState(false);
  useEffect(() => { if (!open) { setLocalErr(""); setLocalBusy(false); } }, [open]);
  const submit = async (e) => {
    e.preventDefault(); setLocalErr(""); setLocalBusy(true);
    try { await onSubmit?.(e); } catch (ex) { setLocalErr(ex.message || String(ex)); } finally { setLocalBusy(false); }
  };
  const err = error || localErr; const isBusy = busy || localBusy;
  return (
    <Modal open={open} onClose={onClose} title={title} subtitle={subtitle} size={size}
      footer={<>{extraFooter && <div className="left">{extraFooter}</div>}<Button variant="secondary" onClick={onClose} disabled={isBusy}>{cancelText}</Button><Button type="submit" form="sr-form-modal" variant={danger ? "danger" : "primary"} loading={isBusy}>{submitText}</Button></>}>
      <form id="sr-form-modal" onSubmit={submit} className="sr-stack">
        {err && <div className="sr-alert bad">{err}</div>}
        {children}
      </form>
    </Modal>
  );
}

// Onay / soru diyaloglari: const confirm = useConfirm(); if (await confirm({title, message, danger})) ...
const DialogCtx = createContext(null);
export function DialogProvider({ children }) {
  const [state, setState] = useState(null);
  const resolver = useRef(null);
  const ask = useCallback((opts) => new Promise((resolve) => { resolver.current = resolve; setState(opts); }), []);
  const done = (v) => { resolver.current?.(v); resolver.current = null; setState(null); };
  const [val, setVal] = useState("");
  useEffect(() => { if (state?.prompt) setVal(state.defaultValue || ""); }, [state]);
  return (
    <DialogCtx.Provider value={ask}>
      {children}
      <Modal open={!!state} onClose={() => done(state?.prompt ? null : false)} title={state?.title || (state?.prompt ? "Bilgi girin" : "Emin misiniz?")} size="sm"
        footer={<><Button variant="secondary" onClick={() => done(state?.prompt ? null : false)}>{state?.cancelText || "Vazgeç"}</Button><Button variant={state?.danger ? "danger" : "primary"} onClick={() => done(state?.prompt ? val : true)} disabled={state?.prompt && state?.required && !val.trim()}>{state?.confirmText || (state?.prompt ? "Tamam" : "Onayla")}</Button></>}>
        {state?.message && <p style={{ color: "var(--text-2)", lineHeight: 1.55 }}>{state.message}</p>}
        {state?.prompt && <form onSubmit={(e) => { e.preventDefault(); if (!state.required || val.trim()) done(val); }} style={{ marginTop: state?.message ? 12 : 0 }}><Field label={state.label}><Input autoFocus type={state.type || "text"} value={val} onChange={(e) => setVal(e.target.value)} placeholder={state.placeholder} minLength={state.minLength} /></Field></form>}
      </Modal>
    </DialogCtx.Provider>
  );
}
export function useConfirm() { const ask = useContext(DialogCtx); return (opts) => ask({ ...opts, prompt: false }); }
export function usePrompt() { const ask = useContext(DialogCtx); return (opts) => ask({ ...opts, prompt: true }); }
