import React, { useEffect, useState } from "react";
import { api, fmtDate, WEEKDAYS } from "../api.js";

const empty = { name: "", kind: "prices", weekdays: [1], runTime: "14:00", hotelIds: [], providers: [], daysAhead: 30, nights: 3, adults: 2, children: 0, airportCode: "SAW", isActive: true };

export default function Schedules({ me }) {
  const [list, setList] = useState([]);
  const [hotels, setHotels] = useState([]);
  const [form, setForm] = useState(null);
  const [err, setErr] = useState("");
  const [msg, setMsg] = useState("");
  const enabled = me.providers.filter((p) => p.enabled);
  const load = () => Promise.all([api("/api/portal/schedules"), api("/api/portal/hotels")]).then(([s, h]) => { setList(s); setHotels(h); }).catch((e) => setErr(e.message));
  useEffect(() => { load(); }, []);

  const toggle = (arr, v) => (arr.includes(v) ? arr.filter((x) => x !== v) : [...arr, v]);
  const save = async (e) => {
    e.preventDefault(); setErr(""); setMsg("");
    const body = { name: form.name, kind: form.kind, weekdays: form.weekdays, runTime: form.runTime, timezone: "Europe/Istanbul", isActive: form.isActive,
      request: { hotelIds: form.hotelIds, providers: form.providers, daysAhead: Number(form.daysAhead), nights: Number(form.nights), adults: Number(form.adults), children: Number(form.children), airportCode: form.kind === "packages" ? form.airportCode : undefined } };
    try {
      if (form.id) await api("/api/portal/schedules/" + form.id, { method: "PUT", body }); else await api("/api/portal/schedules", { method: "POST", body });
      setForm(null); await load(); setMsg("Plan kaydedildi.");
    } catch (ex) { setErr(ex.message); }
  };
  const edit = (s) => { const r = JSON.parse(s.request); setForm({ id: s.id, name: s.name, kind: s.kind, weekdays: s.weekdays, runTime: s.runTime, hotelIds: r.hotelIds || [], providers: r.providers || [], daysAhead: r.daysAhead || 30, nights: r.nights || 3, adults: r.adults || 2, children: r.children || 0, airportCode: r.airportCode || "SAW", isActive: s.isActive }); };
  const remove = async (id) => { if (!confirm("Plan silinsin mi?")) return; try { await api("/api/portal/schedules/" + id, { method: "DELETE" }); await load(); } catch (e) { setErr(e.message); } };
  const runNow = async (id) => { try { const s = await api(`/api/portal/schedules/${id}/run`, { method: "POST" }); setMsg(`Arama başlatıldı (#${s.id}).`); location.hash = "history/" + s.id; } catch (e) { setErr(e.message); } };

  return (
    <div>
      {err && <div className="err">{err}</div>}{msg && <div className="badge ok" style={{ marginBottom: 10 }}>{msg}</div>}
      <div className="card">
        <div className="row" style={{ justifyContent: "space-between" }}><h3>Planlarım</h3><button className="btn" onClick={() => setForm({ ...empty, hotelIds: hotels.slice(0, 1).map((h) => h.internalHotelId) })}>Yeni plan</button></div>
        <p className="muted">Örnek: her Pazartesi 14:00'te, 30 gün sonrasına 3 gecelik fiyat araması. Sonuçlar "Geçmiş" sayfasında birikir.</p>
        <table>
          <thead><tr><th>Ad</th><th>Zaman</th><th>Arama</th><th>Sonraki</th><th>Son</th><th>Durum</th><th></th></tr></thead>
          <tbody>{list.map((s) => { const r = JSON.parse(s.request); return (
            <tr key={s.id}><td><b>{s.name}</b></td>
              <td>{(s.weekdays.length ? s.weekdays.map((d) => WEEKDAYS[d].slice(0, 3)).join(", ") : "Her gün")} {s.runTime}</td>
              <td>{s.kind === "packages" ? "Paket" : "Otel"} · {(r.hotelIds || []).length} otel · +{r.daysAhead} gün, {r.nights} gece, {r.adults} yetişkin</td>
              <td>{fmtDate(s.nextRunAt)}</td><td className="muted">{fmtDate(s.lastRunAt)}</td>
              <td>{s.isActive ? <span className="badge ok">aktif</span> : <span className="badge">pasif</span>}</td>
              <td style={{ whiteSpace: "nowrap" }}><button className="btn ghost" onClick={() => runNow(s.id)}>Şimdi çalıştır</button><button className="btn ghost" onClick={() => edit(s)}>Düzenle</button><button className="btn ghost" onClick={() => remove(s.id)}>Sil</button></td></tr>); })}</tbody>
        </table>
        {list.length === 0 && <div className="muted">Henüz plan yok.</div>}
      </div>
      {form && (
        <div className="card"><h3>{form.id ? "Planı düzenle" : "Yeni plan"}</h3>
          <form onSubmit={save}>
            <div className="row">
              <div><label>Plan adı</label><input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required style={{ width: 260 }} /></div>
              <div><label>Tür</label><select value={form.kind} onChange={(e) => setForm({ ...form, kind: e.target.value })}><option value="prices">Sadece otel</option><option value="packages" disabled={!me.plan.allowPackages}>Otel + uçak + transfer</option></select></div>
              <div><label>Saat (İstanbul)</label><input type="time" value={form.runTime} onChange={(e) => setForm({ ...form, runTime: e.target.value })} /></div>
              <div><label>Aktif</label><input type="checkbox" checked={form.isActive} onChange={(e) => setForm({ ...form, isActive: e.target.checked })} style={{ minWidth: 0 }} /></div>
            </div>
            <div style={{ marginTop: 10 }}><label>Günler (boş = her gün)</label><div className="days">{WEEKDAYS.map((d, i) => <span key={i} className={"chip " + (form.weekdays.includes(i) ? "on" : "")} onClick={() => setForm({ ...form, weekdays: toggle(form.weekdays, i) })}>{d.slice(0, 3)}</span>)}</div></div>
            <div className="row" style={{ marginTop: 10 }}>
              <div><label>Kaç gün sonrası için</label><input type="number" min="1" max="400" value={form.daysAhead} onChange={(e) => setForm({ ...form, daysAhead: e.target.value })} style={{ width: 110 }} /></div>
              <div><label>Gece</label><input type="number" min="1" max="30" value={form.nights} onChange={(e) => setForm({ ...form, nights: e.target.value })} style={{ width: 90 }} /></div>
              <div><label>Yetişkin</label><input type="number" min="1" max="8" value={form.adults} onChange={(e) => setForm({ ...form, adults: e.target.value })} style={{ width: 90 }} /></div>
              <div><label>Çocuk</label><input type="number" min="0" max="6" value={form.children} onChange={(e) => setForm({ ...form, children: e.target.value })} style={{ width: 90 }} /></div>
              {form.kind === "packages" && <div><label>Kalkış (IATA)</label><input value={form.airportCode} maxLength={3} onChange={(e) => setForm({ ...form, airportCode: e.target.value.toUpperCase() })} style={{ width: 90 }} /></div>}
            </div>
            <div style={{ marginTop: 10 }}><label>Oteller</label><div className="chips">{hotels.map((h) => <span key={h.internalHotelId} className={"chip " + (form.hotelIds.includes(h.internalHotelId) ? "on" : "")} onClick={() => setForm({ ...form, hotelIds: toggle(form.hotelIds, h.internalHotelId) })}>{h.hotelName || h.internalHotelId}</span>)}</div></div>
            <div style={{ marginTop: 10 }}><label>Siteler (boş = tüm etkin siteler)</label><div className="chips">{enabled.map((p) => <span key={p.name} className={"chip " + (form.providers.includes(p.name) ? "on" : "")} onClick={() => setForm({ ...form, providers: toggle(form.providers, p.name) })}>{p.label}</span>)}</div></div>
            <div className="row" style={{ marginTop: 14 }}><button className="btn">Kaydet</button><button type="button" className="btn secondary" onClick={() => setForm(null)}>Vazgeç</button></div>
          </form>
        </div>
      )}
    </div>
  );
}
