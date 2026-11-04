import { useEffect, useState } from "react";
const KEY = "smartrate_theme";
export function getTheme() { try { return localStorage.getItem(KEY) || "light"; } catch { return "light"; } }
export function applyTheme(t) { document.documentElement.setAttribute("data-theme", t); try { localStorage.setItem(KEY, t); } catch {} }
export function useTheme() {
  const [theme, set] = useState(getTheme);
  useEffect(() => { applyTheme(theme); }, [theme]);
  return [theme, () => set((t) => (t === "dark" ? "light" : "dark"))];
}
