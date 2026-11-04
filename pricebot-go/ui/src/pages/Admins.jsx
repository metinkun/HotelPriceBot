import React, { useEffect, useState } from "react";
import { api, fmtDate } from "../api.js";
import { PageHeader, DataTable, Button, FormModal, Field, Input, FormGrid, Badge, Alert, useToast } from "@kit";

export default function Admins() {
  const [list, setList] = useState([]);
  const [loading, setLoading] = useState(true);
  const [modal, setModal] = useState(false);
  const [form, setForm] = useState({ email: "", password: "", name: "" });
  const [err, setErr] = useState("");
  const toast = useToast();
  const load = async () => { try { setList(await api("/api/admin/admins")); setErr(""); } catch (e) { setErr(e.message); } finally { setLoading(false); } };
  useEffect(() => { load(); }, []);
  const create = async () => {
    await api("/api/admin/admins", { method: "POST", body: { ...form, name: form.name || null } });
    setModal(false); setForm({ email: "", password: "", name: "" }); toast.success("Admin oluşturuldu."); await load();
  };
  return (
    <div>
      <PageHeader title="Adminler" description="Yönetim paneline giriş yapabilen hesaplar." actions={<Button icon="plus" onClick={() => setModal(true)}>Yeni admin</Button>} />
      {err && <Alert tone="bad">{err}</Alert>}
      <DataTable rows={list} loading={loading}
        columns={[
          { key: "email", title: "E-posta", primary: true, sortable: true, sub: (a) => a.name || "" },
          { key: "lastLoginAt", title: "Son giriş", render: (a) => fmtDate(a.lastLoginAt), sortable: true },
          { key: "isActive", title: "Durum", render: (a) => (a.isActive ? <Badge tone="ok">aktif</Badge> : <Badge tone="bad">pasif</Badge>) },
        ]}
        empty={{ icon: "shield", title: "Admin yok" }} />
      <FormModal open={modal} onClose={() => setModal(false)} title="Yeni admin" onSubmit={create} submitText="Oluştur">
        <Field label="E-posta" required><Input type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} required autoFocus /></Field>
        <FormGrid>
          <Field label="Şifre" required hint="En az 8 karakter"><Input type="password" minLength={8} value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} required /></Field>
          <Field label="Ad"><Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} /></Field>
        </FormGrid>
      </FormModal>
    </div>
  );
}
