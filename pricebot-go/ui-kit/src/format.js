// Ortak bicimlendirme yardimcilari (admin + portal)
export const fmtTL = (v) => (v == null ? "—" : new Intl.NumberFormat("tr-TR", { maximumFractionDigits: 0 }).format(v) + " ₺");
export const fmtNum = (v) => (v == null ? "—" : new Intl.NumberFormat("tr-TR").format(v));
export const fmtDate = (s) => (s ? new Date(s).toLocaleString("tr-TR", { dateStyle: "medium", timeStyle: "short" }) : "—");
export const fmtDay = (s) => (s ? new Date(s).toLocaleDateString("tr-TR", { dateStyle: "medium" }) : "—");
export const fmtRel = (s) => {
  if (!s) return "—";
  const d = (Date.now() - new Date(s).getTime()) / 1000;
  if (d < 60) return "az önce"; if (d < 3600) return Math.floor(d / 60) + " dk önce"; if (d < 86400) return Math.floor(d / 3600) + " sa önce";
  if (d < 7 * 86400) return Math.floor(d / 86400) + " gün önce"; return fmtDay(s);
};
export const WEEKDAYS = ["Pazar", "Pazartesi", "Salı", "Çarşamba", "Perşembe", "Cuma", "Cumartesi"];
export const WEEKDAYS_SHORT = ["Paz", "Pzt", "Sal", "Çar", "Per", "Cum", "Cmt"];
export const addDays = (n) => { const d = new Date(); d.setDate(d.getDate() + n); return d.toISOString().slice(0, 10); };
export const today = () => new Date().toISOString().slice(0, 10);
export const parseReq = (v) => { if (!v) return {}; if (typeof v === "object") return v; try { return JSON.parse(v); } catch { return {}; } };
export const statusTone = (s) => (s === "done" ? "ok" : s === "failed" ? "bad" : s === "running" || s === "queued" ? "warn" : "neutral");
export const statusLabel = (s) => ({ done: "tamamlandı", failed: "hata", running: "çalışıyor", queued: "kuyrukta", partial: "kısmi" }[s] || s || "—");
export const initials = (s = "") => s.split(/\s+/).filter(Boolean).slice(0, 2).map((w) => w[0].toUpperCase()).join("") || "?";
export const cx = (...a) => a.filter(Boolean).join(" ");

// API istemcisi fabrikasi: token anahtari uygulamaya gore degisir
export function createApi(tokenKey) {
  const getToken = () => { try { return localStorage.getItem(tokenKey) || ""; } catch { return ""; } };
  const setToken = (t) => { try { t ? localStorage.setItem(tokenKey, t) : localStorage.removeItem(tokenKey); } catch {} };
  async function api(path, { method = "GET", body } = {}) {
    const headers = { "Content-Type": "application/json" };
    const tok = getToken();
    if (tok) headers.Authorization = "Bearer " + tok;
    const res = await fetch(path, { method, headers, body: body ? JSON.stringify(body) : undefined });
    const text = await res.text();
    let data = null;
    try { data = text ? JSON.parse(text) : null; } catch { data = { error: text }; }
    if (res.status === 401 && !path.endsWith("/auth/login")) { setToken(""); window.dispatchEvent(new Event("pricebot:logout")); }
    if (!res.ok) { const err = new Error((data && data.error) || `HTTP ${res.status}`); err.status = res.status; err.data = data; throw err; }
    return data;
  }
  // Dosya indirme (xlsx vb.): Bearer ile GET, blob -> tarayici indirme.
  async function download(path, fallbackName = "dosya", { method = "GET", body } = {}) {
    const headers = {}; const tok = getToken(); if (tok) headers.Authorization = "Bearer " + tok;
    if (body) headers["Content-Type"] = "application/json";
    const res = await fetch(path, { method, headers, body: body ? JSON.stringify(body) : undefined });
    if (!res.ok) { let msg = `HTTP ${res.status}`; try { msg = (await res.json()).error || msg; } catch {} throw new Error(msg); }
    const cd = res.headers.get("Content-Disposition") || ""; const m = /filename="?([^";]+)"?/.exec(cd);
    const blob = await res.blob(); const url = URL.createObjectURL(blob);
    const a = document.createElement("a"); a.href = url; a.download = (m && m[1]) || fallbackName; document.body.appendChild(a); a.click(); a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 2000);
  }
  return { api, getToken, setToken, download };
}
