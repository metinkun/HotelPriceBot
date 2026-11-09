import React, { useEffect, useState } from "react";
import { api } from "../api.js";
import { PageHeader, DataTable, Button, FormModal, Field, Input, Select, FormGrid, FormSection, Checkbox, ChipSelect, Badge, Alert, useToast } from "@kit";

const empty = {
  code: "", name: "", kind: "both", dailyQuota: 0, allowedProviders: [], allowPackages: true, allowFresh: false, maxConcurrency: 5, ratePerMinute: 60, isActive: true,
  maxSites: 0, maxHotels: 0, maxUsers: 0, dailySearches: 0, dailyScheduledRuns: 0, allowApi: true, isDemo: false, demoDays: 14, maxDevicesPc: 2, maxDevicesMobile: 2,
};
const KIND = { acente: "ACENTE", otel: "OTEL", both: "Her ikisi" };
const inf = (v) => (v ? v : "∞");

export default function Plans() {
  const [plans, setPlans] = useState([]);
  const [providers, setProviders] = useState([]);
  const [loading, setLoading] = useState(true);
  const [edit, setEdit] = useState(null);
  const [err, setErr] = useState("");
  const toast = useToast();
  const load = async () => { try { const [p, h] = await Promise.all([api("/api/admin/plans"), fetch("/health").then((r) => r.json())]); setPlans(p); setProviders(h.providers || []); } catch (e) { setErr(e.message); } finally { setLoading(false); } };
  useEffect(() => { load(); }, []);

  const num = (v, d = 0) => { const n = Number(v); return Number.isFinite(n) ? n : d; };
  const providerErr = edit && Number(edit.maxSites) > 0 && edit.allowedProviders.length > Number(edit.maxSites) ? `İzinli site sayısı (${edit.allowedProviders.length}) site limitini (${edit.maxSites}) aşıyor.` : "";
  const save = async () => {
    if (providerErr) throw new Error(providerErr);
    const body = {
      code: edit.code, name: edit.name, kind: edit.kind || "both", allowedProviders: edit.allowedProviders, allowPackages: !!edit.allowPackages, allowFresh: !!edit.allowFresh, isActive: !!edit.isActive,
      dailyQuota: num(edit.dailyQuota), maxConcurrency: num(edit.maxConcurrency, 5), ratePerMinute: num(edit.ratePerMinute, 60),
      maxSites: num(edit.maxSites), maxHotels: num(edit.maxHotels), maxUsers: num(edit.maxUsers), dailySearches: num(edit.dailySearches), dailyScheduledRuns: num(edit.dailyScheduledRuns),
      allowApi: !!edit.allowApi, isDemo: !!edit.isDemo, demoDays: num(edit.demoDays, 14), maxDevicesPc: num(edit.maxDevicesPc, 2), maxDevicesMobile: num(edit.maxDevicesMobile, 2),
    };
    if (edit.id) await api("/api/admin/plans/" + edit.id, { method: "PUT", body }); else await api("/api/admin/plans", { method: "POST", body });
    setEdit(null); await load(); toast.success("Paket kaydedildi.");
  };
  const f = (k) => (e) => setEdit({ ...edit, [k]: e.target.type === "checkbox" ? e.target.checked : e.target.value });
  const openEdit = (p) => setEdit({ ...empty, ...p });

  return (
    <div>
      <PageHeader title="Paketler" description="Müşteriye atanınca değerleri hesaba kopyalanır ve orada özelleştirilebilir. Müşteriye gösterilen birim günlük arama sayısıdır; site çağrısı yalnızca iç raporlamada kullanılır. 0 = sınırsız." actions={<Button icon="plus" onClick={() => setEdit({ ...empty })}>Yeni paket</Button>} />
      {err && <Alert tone="bad">{err}</Alert>}
      <DataTable rows={plans} loading={loading} onRowDoubleClick={openEdit}
        columns={[
          { key: "name", title: "Paket", primary: true, sortable: true, sub: (p) => <code>{p.code}</code> },
          { key: "kind", title: "Tür", render: (p) => <Badge tone={p.kind === "otel" ? "info" : p.kind === "acente" ? "brand" : "neutral"} dot={false}>{KIND[p.kind] || "Her ikisi"}</Badge> },
          { key: "cap", title: "Kapasite", render: (p) => <span title="Site · Otel · Kullanıcı">{inf(p.maxSites)} site · {inf(p.maxHotels)} otel · {inf(p.maxUsers)} kullanıcı</span>, hideOnMobile: true },
          { key: "dailySearches", title: "Günlük arama", align: "num", sortable: true, render: (p) => inf(p.dailySearches) },
          { key: "dailyScheduledRuns", title: "Planlı görev", align: "num", sortable: true, render: (p) => inf(p.dailyScheduledRuns), hideOnMobile: true },
          { key: "allowApi", title: "API", align: "center", render: (p) => (p.allowApi ? <Badge tone="ok" dot={false}>açık</Badge> : <Badge tone="neutral" dot={false}>kapalı</Badge>), hideOnMobile: true },
          { key: "isDemo", title: "Demo", align: "center", render: (p) => (p.isDemo ? <Badge tone="warn" dot={false}>demo {p.demoDays}g</Badge> : <span className="muted">—</span>), hideOnMobile: true },
          { key: "isActive", title: "Durum", render: (p) => (p.isActive ? <Badge tone="ok">aktif</Badge> : <Badge tone="bad">pasif</Badge>) },
        ]}
        actions={(p) => [{ icon: "pencil", title: "Düzenle", onClick: openEdit }]} />

      <FormModal open={!!edit} onClose={() => setEdit(null)} title={edit?.id ? "Paketi düzenle" : "Yeni paket"} size="md" onSubmit={save}>
        {edit && (<>
          <FormSection>Genel</FormSection>
          <FormGrid>
            <Field label="Kod" required><Input value={edit.code} onChange={f("code")} required placeholder="standard" /></Field>
            <Field label="Ad" required><Input value={edit.name} onChange={f("name")} required placeholder="Standart" /></Field>
            <Field label="Tür" hint="Paket hangi müşteri türüne satılır"><Select value={edit.kind || "both"} onChange={f("kind")}><option value="both">Her ikisi</option><option value="acente">ACENTE</option><option value="otel">OTEL</option></Select></Field>
            <Field label="Durum" inline style={{ paddingTop: 26 }}><Checkbox label="Aktif" checked={!!edit.isActive} onChange={f("isActive")} /></Field>
          </FormGrid>
          <FormSection>Kapasite</FormSection>
          <FormGrid>
            <Field label="Site sayısı" hint="0 = sınırsız"><Input type="number" min="0" value={edit.maxSites} onChange={f("maxSites")} /></Field>
            <Field label="Otel sayısı" hint="0 = sınırsız"><Input type="number" min="0" value={edit.maxHotels} onChange={f("maxHotels")} /></Field>
            <Field label="Kullanıcı sayısı" hint="0 = sınırsız"><Input type="number" min="0" value={edit.maxUsers} onChange={f("maxUsers")} /></Field>
          </FormGrid>
          <FormSection>Günlük haklar</FormSection>
          <FormGrid>
            <Field label="Günlük anlık arama" hint="Bir arama kaç siteye giderse gitsin 1 sayılır"><Input type="number" min="0" value={edit.dailySearches} onChange={f("dailySearches")} /></Field>
            <Field label="Günlük planlı görev" hint="Günde çalışabilecek planlı arama"><Input type="number" min="0" value={edit.dailyScheduledRuns} onChange={f("dailyScheduledRuns")} /></Field>
            <Field label="Site çağrısı / gün" hint="İç gösterim; müşteri görmez"><Input type="number" min="0" value={edit.dailyQuota} onChange={f("dailyQuota")} /></Field>
          </FormGrid>
          <FormSection>Erişim</FormSection>
          <div className="sr-row" style={{ gap: 18 }}>
            <Checkbox label="API kullanıcısı" checked={!!edit.allowApi} onChange={f("allowApi")} />
            <Checkbox label="Paket (otel + uçak) sorgusu" checked={!!edit.allowPackages} onChange={f("allowPackages")} />
            <Checkbox label="Önbelleği atlama (fresh)" checked={!!edit.allowFresh} onChange={f("allowFresh")} />
            <Checkbox label="Demo paketi" checked={!!edit.isDemo} onChange={f("isDemo")} />
          </div>
          {edit.isDemo && <FormGrid><Field label="Demo süresi (gün)" hint="Atandığı günden itibaren; süre dolunca hesap kapanır"><Input type="number" min="1" value={edit.demoDays} onChange={f("demoDays")} /></Field></FormGrid>}
          <FormSection>Cihaz</FormSection>
          <FormGrid>
            <Field label="Bilgisayar" hint="Kullanıcı başına kayıtlı cihaz"><Input type="number" min="0" value={edit.maxDevicesPc} onChange={f("maxDevicesPc")} /></Field>
            <Field label="Mobil" hint="Kullanıcı başına kayıtlı cihaz"><Input type="number" min="0" value={edit.maxDevicesMobile} onChange={f("maxDevicesMobile")} /></Field>
          </FormGrid>
          <FormSection>Hız</FormSection>
          <FormGrid>
            <Field label="İstek / dakika"><Input type="number" min="1" value={edit.ratePerMinute} onChange={f("ratePerMinute")} /></Field>
            <Field label="Eşzamanlı sorgu"><Input type="number" min="1" value={edit.maxConcurrency} onChange={f("maxConcurrency")} /></Field>
          </FormGrid>
          <Field label="İzinli siteler" hint="Boş bırakılırsa tüm siteler" error={providerErr}><ChipSelect value={edit.allowedProviders} onChange={(v) => setEdit({ ...edit, allowedProviders: v })} options={providers.map((n) => ({ value: n, label: n }))} /></Field>
        </>)}
      </FormModal>
    </div>
  );
}
