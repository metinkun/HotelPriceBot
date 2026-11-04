import React, { useEffect, useState } from "react";
import { api } from "../api.js";
import { PageHeader, DataTable, Button, FormModal, Field, Input, Select, FormGrid, Badge, Alert, useToast } from "@kit";

const TYPE = { api: "API", portal: "Portal", both: "API + Portal" };

export default function Clients({ navigate }) {
  const [list, setList] = useState([]);
  const [plans, setPlans] = useState([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");
  const [modal, setModal] = useState(false);
  const [form, setForm] = useState({ name: "", email: "", planId: "" });
  const toast = useToast();

  const load = async () => { try { const [c, p] = await Promise.all([api("/api/admin/clients"), api("/api/admin/plans")]); setList(c); setPlans(p); setErr(""); } catch (e) { setErr(e.message); } finally { setLoading(false); } };
  useEffect(() => { load(); }, []);

  const create = async () => {
    const c = await api("/api/admin/clients", { method: "POST", body: { name: form.name, email: form.email || null, planId: form.planId ? Number(form.planId) : null } });
    setModal(false); setForm({ name: "", email: "", planId: "" }); toast.success("Müşteri oluşturuldu."); navigate("clientdetail", c.id);
  };
  const toggleActive = async (c) => { try { await api("/api/admin/clients/" + c.id, { method: "PUT", body: { isActive: !c.isActive } }); toast.success(c.isActive ? "Müşteri pasife alındı." : "Müşteri aktifleştirildi."); await load(); } catch (e) { toast.error(e.message); } };
  const open = (c) => navigate("clientdetail", c.id);

  return (
    <div>
      <PageHeader title="Müşteriler" description="API entegrasyonu ve web portal müşterileri. Satıra çift tıklayarak detayına gidin." actions={<Button icon="plus" onClick={() => setModal(true)}>Yeni müşteri</Button>} />
      {err && <Alert tone="bad">{err}</Alert>}
      <DataTable rows={list} loading={loading} search={{ placeholder: "Müşteri adı veya e-posta…", keys: ["name", "email"] }}
        columns={[
          { key: "name", title: "Müşteri", primary: true, sortable: true, sub: (c) => c.email || "—" },
          { key: "clientType", title: "Tip", render: (c) => <Badge tone="info" dot={false}>{TYPE[c.clientType] || "API"}</Badge>, hideOnMobile: true },
          { key: "planCode", title: "Plan", sortable: true, render: (c) => c.planCode ? <code>{c.planCode}</code> : <span className="muted">plansız</span> },
          { key: "keyCount", title: "Anahtar", align: "num", hideOnMobile: true },
          { key: "isActive", title: "Durum", render: (c) => (c.isActive ? <Badge tone="ok">aktif</Badge> : <Badge tone="bad">pasif</Badge>) },
        ]}
        actions={(c) => [{ icon: "eye", title: "Detay", onClick: open }, { icon: "power", title: c.isActive ? "Pasife al" : "Aktifleştir", onClick: toggleActive }]}
        onRowDoubleClick={open} empty={{ icon: "users", title: "Henüz müşteri yok", action: <Button icon="plus" onClick={() => setModal(true)}>Yeni müşteri</Button> }} />

      <FormModal open={modal} onClose={() => setModal(false)} title="Yeni müşteri" subtitle="Tip, siteler, anahtarlar ve portal kullanıcıları oluşturduktan sonra detay sayfasında ayarlanır." onSubmit={create} submitText="Oluştur">
        <Field label="Ad" required><Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required autoFocus placeholder="Örn. Acme Travel" /></Field>
        <FormGrid>
          <Field label="E-posta"><Input type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} placeholder="ops@acme.com" /></Field>
          <Field label="Plan" hint="Plansız = sınırsız"><Select value={form.planId} onChange={(e) => setForm({ ...form, planId: e.target.value })}><option value="">(plansız)</option>{plans.map((p) => <option key={p.id} value={p.id}>{p.name} ({p.code})</option>)}</Select></Field>
        </FormGrid>
      </FormModal>
    </div>
  );
}
