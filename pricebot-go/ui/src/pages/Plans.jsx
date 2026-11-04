import React, { useEffect, useState } from "react";
import { api } from "../api.js";
import { PageHeader, DataTable, Button, FormModal, Field, Input, FormGrid, FormSection, Checkbox, ChipSelect, Badge, Alert, useToast } from "@kit";

const empty = { code: "", name: "", dailyQuota: 0, allowedProviders: [], allowPackages: true, allowFresh: false, maxConcurrency: 5, ratePerMinute: 60, isActive: true };

export default function Plans() {
  const [plans, setPlans] = useState([]);
  const [providers, setProviders] = useState([]);
  const [loading, setLoading] = useState(true);
  const [edit, setEdit] = useState(null);
  const [err, setErr] = useState("");
  const toast = useToast();
  const load = async () => { try { const [p, h] = await Promise.all([api("/api/admin/plans"), fetch("/health").then((r) => r.json())]); setPlans(p); setProviders(h.providers || []); } catch (e) { setErr(e.message); } finally { setLoading(false); } };
  useEffect(() => { load(); }, []);

  const save = async () => {
    const body = { ...edit, dailyQuota: Number(edit.dailyQuota), maxConcurrency: Number(edit.maxConcurrency), ratePerMinute: Number(edit.ratePerMinute) };
    if (edit.id) await api("/api/admin/plans/" + edit.id, { method: "PUT", body }); else await api("/api/admin/plans", { method: "POST", body });
    setEdit(null); await load(); toast.success("Plan kaydedildi.");
  };
  const f = (k) => (e) => setEdit({ ...edit, [k]: e.target.type === "checkbox" ? e.target.checked : e.target.value });

  return (
    <div>
      <PageHeader title="Planlar" description="Kota birimi kredi: bir siteye atılan her gerçek sorgu 1 kredi, önbellekten dönen sonuç 0 kredi. 0 = sınırsız. İzinli site boş = tümü." actions={<Button icon="plus" onClick={() => setEdit({ ...empty })}>Yeni plan</Button>} />
      {err && <Alert tone="bad">{err}</Alert>}
      <DataTable rows={plans} loading={loading} onRowDoubleClick={(p) => setEdit({ ...p })}
        columns={[
          { key: "name", title: "Plan", primary: true, sortable: true, sub: (p) => <code>{p.code}</code> },
          { key: "dailyQuota", title: "Günlük kredi", align: "num", sortable: true, render: (p) => (p.dailyQuota ? p.dailyQuota : "∞") },
          { key: "allowedProviders", title: "İzinli siteler", render: (p) => (p.allowedProviders.length ? p.allowedProviders.join(", ") : <span className="muted">tümü</span>) },
          { key: "allowPackages", title: "Paket", align: "center", render: (p) => (p.allowPackages ? <Badge tone="ok" dot={false}>açık</Badge> : <Badge tone="neutral" dot={false}>kapalı</Badge>), hideOnMobile: true },
          { key: "allowFresh", title: "Fresh", align: "center", render: (p) => (p.allowFresh ? <Badge tone="ok" dot={false}>açık</Badge> : <Badge tone="neutral" dot={false}>kapalı</Badge>), hideOnMobile: true },
          { key: "ratePerMinute", title: "İstek/dk", align: "num", hideOnMobile: true },
          { key: "isActive", title: "Durum", render: (p) => (p.isActive ? <Badge tone="ok">aktif</Badge> : <Badge tone="bad">pasif</Badge>) },
        ]}
        actions={(p) => [{ icon: "pencil", title: "Düzenle", onClick: (r) => setEdit({ ...r }) }]} />

      <FormModal open={!!edit} onClose={() => setEdit(null)} title={edit?.id ? "Planı düzenle" : "Yeni plan"} size="md" onSubmit={save}>
        {edit && (<>
          <FormGrid>
            <Field label="Kod" required><Input value={edit.code} onChange={f("code")} required placeholder="standard" /></Field>
            <Field label="Ad" required><Input value={edit.name} onChange={f("name")} required placeholder="Standart" /></Field>
          </FormGrid>
          <FormSection>Limitler</FormSection>
          <FormGrid>
            <Field label="Günlük kredi" hint="0 = sınırsız"><Input type="number" min="0" value={edit.dailyQuota} onChange={f("dailyQuota")} /></Field>
            <Field label="İstek / dakika"><Input type="number" min="1" value={edit.ratePerMinute} onChange={f("ratePerMinute")} /></Field>
            <Field label="Eşzamanlı sorgu"><Input type="number" min="1" value={edit.maxConcurrency} onChange={f("maxConcurrency")} /></Field>
          </FormGrid>
          <FormSection>İzinler</FormSection>
          <div className="sr-row" style={{ gap: 18 }}>
            <Checkbox label="Paket (otel + uçak) sorgusu" checked={edit.allowPackages} onChange={f("allowPackages")} />
            <Checkbox label="Önbelleği atlama (fresh)" checked={edit.allowFresh} onChange={f("allowFresh")} />
            <Checkbox label="Aktif" checked={edit.isActive} onChange={f("isActive")} />
          </div>
          <Field label="İzinli siteler" hint="Boş bırakılırsa tüm siteler"><ChipSelect value={edit.allowedProviders} onChange={(v) => setEdit({ ...edit, allowedProviders: v })} options={providers.map((n) => ({ value: n, label: n }))} /></Field>
        </>)}
      </FormModal>
    </div>
  );
}
