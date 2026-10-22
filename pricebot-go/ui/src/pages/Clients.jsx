import React, { useEffect, useState } from "react";
import { api } from "../api.js";
import ClientDetail from "./ClientDetail.jsx";

export default function Clients() {
  const [detailId, setDetailId] = useState(null);
  const [list, setList] = useState([]);
  const [plans, setPlans] = useState([]);
  const [search, setSearch] = useState("");
  const [sel, setSel] = useState(null); // {client, keys, usedToday}
  const [newKey, setNewKey] = useState(null);
  const [form, setForm] = useState({ name: "", email: "", planId: "" });
  const [err, setErr] = useState("");

  const load = async () => {
    try {
      const [c, p] = await Promise.all([api("/api/admin/clients?search=" + encodeURIComponent(search)), api("/api/admin/plans")]);
      setList(c); setPlans(p); setErr("");
    } catch (e) { setErr(e.message); }
  };
  useEffect(() => { load(); }, [search]);

  const open = async (id) => { try { setSel(await api("/api/admin/clients/" + id)); setNewKey(null); } catch (e) { setErr(e.message); } };
  const create = async (e) => {
    e.preventDefault();
    try {
      const body = { name: form.name, email: form.email || null, planId: form.planId ? Number(form.planId) : null };
      const c = await api("/api/admin/clients", { method: "POST", body });
      setForm({ name: "", email: "", planId: "" }); await load(); open(c.id);
    } catch (ex) { setErr(ex.message); }
  };
  const update = async (patch) => { try { await api("/api/admin/clients/" + sel.client.id, { method: "PUT", body: patch }); await load(); open(sel.client.id); } catch (e) { setErr(e.message); } };
  const remove = async () => { if (!confirm("Müşteri ve tüm anahtarları silinecek. Emin misiniz?")) return; try { await api("/api/admin/clients/" + sel.client.id, { method: "DELETE" }); setSel(null); await load(); } catch (e) { setErr(e.message); } };
  const createKey = async () => { const label = prompt("Anahtar etiketi (opsiyonel):") ?? ""; try { const r = await api(`/api/admin/clients/${sel.client.id}/keys`, { method: "POST", body: { label: label || null } }); setNewKey(r.key); open(sel.client.id); } catch (e) { setErr(e.message); } };
  const revoke = async (id) => { if (!confirm("Anahtar iptal edilsin mi?")) return; try { await api("/api/admin/keys/" + id, { method: "DELETE" }); open(sel.client.id); } catch (e) { setErr(e.message); } };

  return (
    <div>
      <h2>Müşteriler</h2>
      {err && <div className="err">{err}</div>}
      {detailId && <ClientDetail clientId={detailId} onClose={() => setDetailId(null)} />}
      <div className="card">
        <h3>Yeni müşteri</h3>
        <form className="row" onSubmit={create}>
          <div><label>Ad *</label><input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required /></div>
          <div><label>E-posta</label><input value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} /></div>
          <div><label>Plan</label><select value={form.planId} onChange={(e) => setForm({ ...form, planId: e.target.value })}><option value="">(plansız = sınırsız)</option>{plans.map((p) => <option key={p.id} value={p.id}>{p.name} ({p.code})</option>)}</select></div>
          <button className="btn">Oluştur</button>
        </form>
      </div>
      <div className="row" style={{ alignItems: "flex-start" }}>
        <div className="card" style={{ flex: 1, minWidth: 380 }}>
          <input placeholder="Ara…" value={search} onChange={(e) => setSearch(e.target.value)} style={{ width: "100%", marginBottom: 10 }} />
          <table>
            <thead><tr><th>Ad</th><th>Plan</th><th>Anahtar</th><th>Durum</th></tr></thead>
            <tbody>{list.map((c) => (
              <tr key={c.id} onClick={() => open(c.id)} style={{ cursor: "pointer" }}>
                <td><b>{c.name}</b><div className="muted">{c.email}</div></td><td>{c.planCode || <span className="muted">—</span>}</td><td>{c.keyCount}</td>
                <td>{c.isActive ? <span className="badge ok">aktif</span> : <span className="badge bad">pasif</span>}</td>
              </tr>))}</tbody>
          </table>
        </div>
        {sel && (
          <div className="card" style={{ flex: 1, minWidth: 420 }}>
            <div className="row" style={{ justifyContent: "space-between" }}><h3>{sel.client.name} <span className="muted">#{sel.client.id}</span></h3>
              <div className="row"><button className="btn" onClick={() => setDetailId(sel.client.id)}>Siteler & portal</button><button className="btn secondary" onClick={() => update({ isActive: !sel.client.isActive })}>{sel.client.isActive ? "Pasife al" : "Aktifleştir"}</button><button className="btn danger" onClick={remove}>Sil</button></div></div>
            <div className="row" style={{ marginTop: 8 }}>
              <div><label>Plan</label><select value={sel.client.planId ?? ""} onChange={(e) => update({ planId: e.target.value ? Number(e.target.value) : null })}><option value="">(plansız)</option>{plans.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}</select></div>
              <div><label>Bugünkü kredi</label><div style={{ padding: "8px 0" }}><b>{sel.usedToday}</b></div></div>
            </div>
            <div className="row" style={{ justifyContent: "space-between", marginTop: 16 }}><h3>API anahtarları</h3><button className="btn" onClick={createKey}>Yeni anahtar</button></div>
            {newKey && <div className="keybox"><b>Yeni anahtar (bir kez gösterilir):</b><br /><code>{newKey}</code></div>}
            <table style={{ marginTop: 8 }}>
              <thead><tr><th>Önek</th><th>Etiket</th><th>Son kullanım</th><th>Durum</th><th></th></tr></thead>
              <tbody>{sel.keys.map((k) => (
                <tr key={k.id}><td><code>{k.keyPrefix}…</code></td><td>{k.label}</td><td className="muted">{k.lastUsedAt ? new Date(k.lastUsedAt).toLocaleString("tr-TR") : "—"}</td>
                  <td>{k.revokedAt ? <span className="badge bad">iptal</span> : <span className="badge ok">aktif</span>}</td>
                  <td>{!k.revokedAt && <button className="btn secondary" onClick={() => revoke(k.id)}>İptal</button>}</td></tr>))}</tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
