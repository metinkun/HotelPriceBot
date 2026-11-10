import React, { useEffect, useState } from "react";
import { api, fmtDate, WEEKDAYS_SHORT, parseReq } from "../api.js";
import { PageHeader, DataTable, Button, FormModal, Field, Input, FormGrid, FormSection, Segmented, Switch, ChipSelect, SitePicker, Badge, Alert, useConfirm, useToast } from "@kit";

const empty = { name: "", kind: "prices", weekdays: [1], runTime: "14:00", hotelIds: [], providers: [], daysAhead: 30, nights: 3, adults: 2, children: 0, airportCode: "SAW", isActive: true, startsOn: "", endsOn: "" };
const todayStr = () => new Date().toISOString().slice(0, 10);
const expired = (s) => s.endsOn && s.endsOn < todayStr();
const range = (s) => (s.startsOn || s.endsOn ? `${s.startsOn || "hemen"} → ${s.endsOn || "süresiz"}` : "süresiz");

export default function Schedules({ me, navigate }) {
  const [list, setList] = useState([]);
  const [hotels, setHotels] = useState([]);
  const [loading, setLoading] = useState(true);
  const [form, setForm] = useState(null);
  const [err, setErr] = useState("");
  const enabled = me.providers.filter((p) => p.enabled);
  const confirm = useConfirm(); const toast = useToast();
  const load = () => Promise.all([api("/api/portal/schedules"), api("/api/portal/hotels")]).then(([s, h]) => { setList(s); setHotels(h); }).catch((e) => setErr(e.message)).finally(() => setLoading(false));
  useEffect(() => { load(); }, []);

  const save = async () => {
    if (form.startsOn && form.endsOn && form.endsOn < form.startsOn) throw new Error("Bitiş tarihi başlangıçtan önce olamaz.");
    const body = { name: form.name, kind: form.kind, weekdays: form.weekdays, runTime: form.runTime, timezone: "Europe/Istanbul", isActive: form.isActive, startsOn: form.startsOn || "", endsOn: form.endsOn || "",
      request: { hotelIds: form.hotelIds, providers: form.providers, daysAhead: Number(form.daysAhead), nights: Number(form.nights), adults: Number(form.adults), children: Number(form.children), airportCode: form.kind === "packages" ? form.airportCode : undefined } };
    if (!form.hotelIds.length) throw new Error("En az bir otel seçin.");
    if (form.id) await api("/api/portal/schedules/" + form.id, { method: "PUT", body }); else await api("/api/portal/schedules", { method: "POST", body });
    setForm(null); await load(); toast.success("Planlı arama kaydedildi.");
  };
  const edit = (s) => { const r = parseReq(s.request); setForm({ id: s.id, name: s.name, kind: s.kind, weekdays: s.weekdays, runTime: s.runTime, hotelIds: r.hotelIds || [], providers: r.providers || [], daysAhead: r.daysAhead || 30, nights: r.nights || 3, adults: r.adults || 2, children: r.children || 0, airportCode: r.airportCode || "SAW", isActive: s.isActive, startsOn: s.startsOn || "", endsOn: s.endsOn || "" }); };
  const toggleActive = async (s) => {
    if (s.isActive && !(await confirm({ title: "Planlı arama pasife alınsın mı?", message: `${s.name} çalışmayı durdurur; geçmiş sonuçları ve ayarları korunur, istediğinizde yeniden aktifleştirebilirsiniz.`, confirmText: "Pasife al", danger: true }))) return;
    try { await api(`/api/portal/schedules/${s.id}/active`, { method: "POST", body: { isActive: !s.isActive } }); toast.success(s.isActive ? "Pasife alındı." : "Aktifleştirildi."); await load(); } catch (e) { toast.error(e.message); }
  };
  const runNow = async (s) => { try { const r = await api(`/api/portal/schedules/${s.id}/run`, { method: "POST" }); toast.success(`Arama başlatıldı (#${r.id}).`); navigate("history", r.id); } catch (e) { toast.error(e.message); } };
  const f = (k) => (e) => setForm({ ...form, [k]: e.target.type === "checkbox" ? e.target.checked : e.target.value });

  return (
    <div>
      <PageHeader title="Planlı aramalar" description={`Örnek: her Pazartesi 14:00'te, 30 gün sonrasına 3 gecelik fiyat araması. Sonuçlar Geçmiş sayfasında birikir. Bugün ${me.usage?.scheduledRuns ?? 0}/${me.limits?.dailyScheduledRuns ? me.limits.dailyScheduledRuns : "∞"} planlı çalışma. Planlı aramalar silinmez, pasife alınır.`} actions={<Button icon="plus" onClick={() => setForm({ ...empty, hotelIds: hotels.slice(0, 1).map((h) => h.internalHotelId) })}>Yeni planlı arama</Button>} />
      {err && <Alert tone="bad">{err}</Alert>}
      <DataTable rows={list} loading={loading} onRowDoubleClick={edit}
        columns={[
          { key: "name", title: "Planlı arama", primary: true, sortable: true, sub: (s) => range(s) },
          { key: "time", title: "Zaman", render: (s) => `${s.weekdays.length ? s.weekdays.map((d) => WEEKDAYS_SHORT[d]).join(", ") : "Her gün"} ${s.runTime}` },
          { key: "req", title: "Arama", render: (s) => { const r = parseReq(s.request); return `${s.kind === "packages" ? "Paket" : "Otel"} · ${(r.hotelIds || []).length} otel · +${r.daysAhead} gün, ${r.nights} gece, ${r.adults} yetişkin`; } },
          { key: "nextRunAt", title: "Sonraki", render: (s) => fmtDate(s.nextRunAt), sortable: true },
          { key: "lastRunAt", title: "Son", render: (s) => <span className="muted">{fmtDate(s.lastRunAt)}</span>, hideOnMobile: true },
          { key: "isActive", title: "Durum", render: (s) => (expired(s) ? <Badge tone="neutral">süresi doldu</Badge> : s.isActive ? <Badge tone="ok">aktif</Badge> : <Badge tone="neutral">pasif</Badge>), filterValue: (s) => (expired(s) ? "süresi doldu" : s.isActive ? "aktif" : "pasif") },
        ]}
        actions={(s) => [{ icon: "play", title: "Şimdi çalıştır", tone: "primary", onClick: runNow }, { icon: "history", title: "Geçmiş sonuçlar", onClick: (x) => navigate("history", "s" + x.id) }, { icon: "pencil", title: "Düzenle", onClick: edit }, { icon: "power", title: s.isActive ? "Pasife al" : "Aktifleştir", tone: s.isActive ? "danger" : "primary", onClick: toggleActive }]}
        empty={{ icon: "calendar", title: "Henüz planlı arama yok", text: "Belirlediğiniz gün ve saatte otomatik fiyat araması kurun.", action: <Button icon="plus" onClick={() => setForm({ ...empty, hotelIds: hotels.slice(0, 1).map((h) => h.internalHotelId) })}>Yeni planlı arama</Button> }} />

      <FormModal open={!!form} onClose={() => setForm(null)} title={form?.id ? "Planlı aramayı düzenle" : "Yeni planlı arama"} size="md" onSubmit={save}>
        {form && (<>
          <FormGrid>
            <Field label="Arama adı" required className="span-2"><Input value={form.name} onChange={f("name")} required autoFocus placeholder="Örn. Haftalık Girne kontrolü" /></Field>
            <Field label="Saat (İstanbul)"><Input type="time" value={form.runTime} onChange={f("runTime")} required /></Field>
          </FormGrid>
          <FormGrid style={{ marginTop: 12 }}>
            <Field label="Başlangıç tarihi" hint="Boş = hemen başlar"><Input type="date" value={form.startsOn} onChange={f("startsOn")} min={todayStr()} /></Field>
            <Field label="Bitiş tarihi" hint="Boş = süresiz; bu tarihten sonra çalışmaz"><Input type="date" value={form.endsOn} onChange={f("endsOn")} min={form.startsOn || todayStr()} /></Field>
          </FormGrid>
          <div className="sr-row center" style={{ marginTop: 12, gap: 18 }}>
            <Segmented value={form.kind} onChange={(v) => setForm({ ...form, kind: v })} options={[{ value: "prices", label: "Sadece otel" }, { value: "packages", label: "Otel + uçak + transfer", disabled: !me.plan.allowPackages, title: me.plan.allowPackages ? "" : "Paketiniz otel + uçak sorgusunu kapsamıyor" }]} />
            <Switch label="Aktif" checked={form.isActive} onChange={f("isActive")} />
          </div>
          <Field label="Günler" hint="Boş = her gün" style={{ marginTop: 12 }}><ChipSelect value={form.weekdays} onChange={(v) => setForm({ ...form, weekdays: v })} options={WEEKDAYS_SHORT.map((d, i) => ({ value: i, label: d }))} className="sr-days" /></Field>
          <FormSection>Arama</FormSection>
          <FormGrid style={{ gridTemplateColumns: "repeat(auto-fit, minmax(120px, 1fr))" }}>
            <Field label="Kaç gün sonrası"><Input type="number" min="1" max="400" value={form.daysAhead} onChange={f("daysAhead")} /></Field>
            <Field label="Gece"><Input type="number" min="1" max="30" value={form.nights} onChange={f("nights")} /></Field>
            <Field label="Yetişkin"><Input type="number" min="1" max="8" value={form.adults} onChange={f("adults")} /></Field>
            <Field label="Çocuk"><Input type="number" min="0" max="6" value={form.children} onChange={f("children")} /></Field>
            {form.kind === "packages" && <Field label="Kalkış (IATA)"><Input value={form.airportCode} maxLength={3} onChange={(e) => setForm({ ...form, airportCode: e.target.value.toUpperCase() })} /></Field>}
          </FormGrid>
          <Field label="Oteller" required style={{ marginTop: 12 }}><ChipSelect value={form.hotelIds} onChange={(v) => setForm({ ...form, hotelIds: v })} options={hotels.map((h) => ({ value: h.internalHotelId, label: h.hotelName || h.internalHotelId }))} /></Field>
          <Field label="Siteler" hint="Boş = tüm etkin siteler" style={{ marginTop: 12 }}><SitePicker compact value={form.providers} onChange={(v) => setForm({ ...form, providers: v })} options={enabled.map((p) => ({ value: p.name, label: p.label, hint: p.domain }))} emptyMeansAll /></Field>
        </>)}
      </FormModal>
    </div>
  );
}
