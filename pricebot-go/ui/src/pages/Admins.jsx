import React, { useEffect, useState } from "react";
import { api } from "../api.js";

export default function Admins() {
  const [list, setList] = useState([]);
  const [form, setForm] = useState({ email: "", password: "", name: "" });
  const [err, setErr] = useState("");
  const load = async () => { try { setList(await api("/api/admin/admins")); } catch (e) { setErr(e.message); } };
  useEffect(() => { load(); }, []);
  const create = async (e) => { e.preventDefault(); try { await api("/api/admin/admins", { method: "POST", body: { ...form, name: form.name || null } }); setForm({ email: "", password: "", name: "" }); setErr(""); await load(); } catch (ex) { setErr(ex.message); } };
  return (
    <div>
      <h2>Adminler</h2>
      {err && <div className="err">{err}</div>}
      <div className="card"><h3>Yeni admin</h3>
        <form className="row" onSubmit={create}>
          <div><label>E-posta</label><input type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} required /></div>
          <div><label>Şifre (min 8)</label><input type="password" minLength={8} value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} required /></div>
          <div><label>Ad</label><input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} /></div>
          <button className="btn">Oluştur</button>
        </form></div>
      <div className="card"><table><thead><tr><th>E-posta</th><th>Ad</th><th>Son giriş</th><th>Durum</th></tr></thead>
        <tbody>{list.map((a) => <tr key={a.id}><td>{a.email}</td><td>{a.name}</td><td className="muted">{a.lastLoginAt ? new Date(a.lastLoginAt).toLocaleString("tr-TR") : "—"}</td><td>{a.isActive ? <span className="badge ok">aktif</span> : <span className="badge bad">pasif</span>}</td></tr>)}</tbody></table></div>
    </div>
  );
}
