import React, { createContext, useCallback, useContext, useRef, useState } from "react";
import { Icon } from "./Icon.jsx";

const Ctx = createContext(null);
export function ToastProvider({ children }) {
  const [items, setItems] = useState([]);
  const id = useRef(0);
  const push = useCallback((tone, text, ms = 3800) => {
    const i = ++id.current;
    setItems((l) => [...l, { i, tone, text }]);
    setTimeout(() => setItems((l) => l.filter((x) => x.i !== i)), ms);
  }, []);
  const api = useRef({ success: (t) => push("ok", t), error: (t) => push("bad", t, 6000), info: (t) => push("info", t), warn: (t) => push("warn", t) }).current;
  const ic = { ok: "checkCircle", bad: "xCircle", info: "info", warn: "alert" };
  return (
    <Ctx.Provider value={api}>
      {children}
      <div className="sr-toasts" aria-live="polite">{items.map((t) => <div key={t.i} className={"sr-toast " + t.tone}><Icon name={ic[t.tone]} size={17} /><div>{t.text}</div></div>)}</div>
    </Ctx.Provider>
  );
}
export function useToast() { return useContext(Ctx) || { success() {}, error() {}, info() {}, warn() {} }; }
