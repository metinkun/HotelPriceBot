import React, { useEffect, useState } from "react";
import { api } from "../api.js";
import { PageHeader, DataTable, Button, Modal, FormModal, Field, Input, Select, Textarea, Switch, FormGrid, Segmented, Badge, Alert, EmptyState, useConfirm, useToast } from "@kit";

export default function Hotels({ navigate }) {
  const [hotels, setHotels] = useState([]);
  const [providers, setProviders] = useState([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");
  const [modal, setModal] = useState(false);
  const [name, setName] = useState("");
  const [urls, setUrls] = useState({});
  const [result, setResult] = useState(null);
  const [filter, setFilter] = useState("all");
  const [meta, setMeta] = useState(null); // kunye modali
  const [clientsOf, setClientsOf] = useState(null); // {hotel, clients}
  const confirm = useConfirm(); const toast = useToast();

  const load = async () => { try { const [h, hh] = await Promise.all([api("/api/admin/hotels"), fetch("/health").then((r) => r.json())]); setHotels(h); setProviders(hh.providers || []); setErr(""); } catch (e) { setErr(e.message); } finally { setLoading(false); } };
  useEffect(() => { load(); }, []);

  const add = async () => {
    const clean = Object.fromEntries(Object.entries(urls).filter(([, v]) => v && v.trim()));
    if (!Object.keys(clean).length) throw new Error("En az bir site linki girin.");
    const r = await api("/api/admin/hotels", { method: "POST", body: { hotelName: name, urls: clean } });
    setResult(r); setModal(false); setName(""); setUrls({}); await load();
    toast.success(`Otel eklendi: ${r.internalHotelId}` + (r.errors?.length ? ` (${r.errors.length} site çözülemedi)` : ""));
  };
  const toggleActive = async (h) => {
    const on = !isActive(h);
    if (!on && !(await confirm({ title: "Otel pasife alınsın mı?", message: `${h.hotelName || h.internalHotelId} aramalarda atlanacak; kayıt ve eşleştirmeler korunur.`, confirmText: "Pasife al", danger: true }))) return;
    try { await api(`/api/admin/hotels/${h.internalHotelId}/meta`, { method: "PUT", body: { isActive: on } }); toast.success(on ? "Otel aktifleştirildi." : "Otel pasife alındı."); await load(); } catch (e) { toast.error(e.message); }
  };
  const open = (h) => navigate("hoteldetail", h.internalHotelId);
  const openMeta = (h) => { const m = h.meta || {}; setMeta({ id: h.internalHotelId, name: h.hotelName || "", city: m.city || "", region: m.region || "", country: m.country || "", stars: m.stars ?? "", chain: m.chain || "", externalId: m.externalId || "", isActive: m.isActive !== false, notes: m.notes || "" }); };
  const saveMeta = async () => {
    const body = { name: meta.name, city: meta.city, region: meta.region, country: meta.country, chain: meta.chain, externalId: meta.externalId, isActive: !!meta.isActive, notes: meta.notes };
    if (meta.stars !== "") body.stars = Number(meta.stars);
    await api(`/api/admin/hotels/${meta.id}/meta`, { method: "PUT", body }); setMeta(null); toast.success("Otel künyesi kaydedildi."); await load();
  };
  const openClients = async (h) => { try { setClientsOf({ hotel: h, clients: await api(`/api/admin/hotels/${h.internalHotelId}/clients`) }); } catch (e) { toast.error(e.message); } };
  const isActive = (h) => h.meta?.isActive !== false;
  const rows = hotels.filter((h) => filter === "all" || (filter === "active" ? isActive(h) : !isActive(h)));
  const loc = (h) => [h.meta?.city, h.meta?.country].filter(Boolean).join(" · ");

  return (
    <div>
      <PageHeader title="Oteller" description="Sistemdeki oteller, künyeleri ve her sitedeki eşleştirmeleri. Satıra çift tıklayarak otel özetine gidin. Silme yoktur; otel pasife alınır." actions={<><Button variant="secondary" icon="upload" onClick={() => navigate("import")}>Toplu ekle</Button><Button icon="plus" onClick={() => setModal(true)}>Otel ekle</Button></>} />
      {err && <Alert tone="bad">{err}</Alert>}
      {result?.errors?.length > 0 && <Alert tone="warn">Çözümlenemeyen siteler: {result.errors.map((e) => `${e.provider}: ${e.error}`).join(" · ")}</Alert>}
      <DataTable rows={rows} rowKey={(h) => h.internalHotelId} loading={loading}
        search={{ placeholder: "Otel adı, kimlik, şehir, zincir, dış kimlik…", keys: ["hotelName", "internalHotelId", (h) => h.meta?.city, (h) => h.meta?.chain, (h) => h.meta?.externalId, (h) => h.providers.map((p) => p.provider).join(" ")] }}
        toolbar={<Segmented value={filter} onChange={setFilter} options={[{ value: "all", label: `Tümü (${hotels.length})` }, { value: "active", label: "Aktif" }, { value: "passive", label: "Pasif" }]} />}
        columns={[
          { key: "hotelName", title: "Otel", primary: true, sortable: true, render: (h) => <span style={{ opacity: isActive(h) ? 1 : 0.6 }}>{h.hotelName || h.internalHotelId}</span>, sub: (h) => <><code>{h.internalHotelId}</code>{h.meta?.chain ? <span className="muted"> · {h.meta.chain}</span> : null}</> },
          { key: "loc", title: "Konum", render: (h) => <span className="muted">{loc(h) || "—"}</span>, sortable: true, sortValue: loc, hideOnMobile: true },
          { key: "stars", title: "Yıldız", render: (h) => (h.meta?.stars ? <span style={{ color: "var(--warn)" }}>{"★".repeat(h.meta.stars)}</span> : <span className="muted">—</span>), sortable: true, sortValue: (h) => h.meta?.stars || 0, hideOnMobile: true },
          { key: "providers", title: "Siteler", render: (h) => <div className="sr-chips" style={{ gap: 4 }}>{h.providers.map((p) => <Badge key={p.provider} tone={p.isActive ? "info" : "neutral"} dot={false} title={p.sourceUrl || ""}>{p.provider}</Badge>)}</div>, hideOnMobile: true, filterValue: (h) => h.providers.map((p) => p.provider).join(" ") },
          { key: "count", title: "Site", align: "num", width: 70, sortable: true, sortValue: (h) => h.providers.length, render: (h) => h.providers.length },
          { key: "active", title: "Durum", render: (h) => (isActive(h) ? <Badge tone="ok">aktif</Badge> : <Badge tone="neutral">pasif</Badge>), filterValue: (h) => (isActive(h) ? "aktif" : "pasif") },
        ]}
        actions={(h) => [{ icon: "eye", title: "Detay", onClick: open }, { icon: "pencil", title: "Künye", onClick: openMeta }, { icon: "users", title: "Müşteriler", onClick: openClients }, { icon: "power", title: isActive(h) ? "Pasife al" : "Aktifleştir", onClick: toggleActive }]}
        onRowDoubleClick={open} empty={{ icon: "building", title: "Henüz otel yok", text: "URL ile tek tek veya listeyle toplu ekleyebilirsiniz.", action: <Button icon="plus" onClick={() => setModal(true)}>Otel ekle</Button> }} />

      <FormModal open={modal} onClose={() => setModal(false)} title="URL'lerden otel ekle" subtitle="Her site için otelin detay sayfası linkini yapıştırın; sistem otel kimliğini kendisi çözer. En az bir site yeterlidir." size="md" onSubmit={add} submitText="Ekle">
        <Field label="Otel adı" required><Input value={name} onChange={(e) => setName(e.target.value)} required placeholder="Örn. Merit Crystal Cove Hotel" /></Field>
        <FormGrid>{providers.map((p) => <Field key={p} label={p}><Input placeholder="https://…" value={urls[p] || ""} onChange={(e) => setUrls({ ...urls, [p]: e.target.value })} /></Field>)}</FormGrid>
        <p className="muted small">Touristica çözümlemesi 20-60 saniye sürebilir.</p>
      </FormModal>

      <FormModal open={!!meta} onClose={() => setMeta(null)} title="Otel künyesi" subtitle={meta ? meta.id : ""} size="md" onSubmit={saveMeta}>
        {meta && (<>
          <Field label="Ad" required><Input value={meta.name} onChange={(e) => setMeta({ ...meta, name: e.target.value })} required /></Field>
          <FormGrid>
            <Field label="Şehir"><Input value={meta.city} onChange={(e) => setMeta({ ...meta, city: e.target.value })} placeholder="Girne" /></Field>
            <Field label="Bölge"><Input value={meta.region} onChange={(e) => setMeta({ ...meta, region: e.target.value })} placeholder="Alsancak" /></Field>
            <Field label="Ülke"><Input value={meta.country} onChange={(e) => setMeta({ ...meta, country: e.target.value })} placeholder="TRNC" /></Field>
            <Field label="Yıldız"><Select value={meta.stars} onChange={(e) => setMeta({ ...meta, stars: e.target.value })}><option value="">—</option>{[0, 1, 2, 3, 4, 5, 6, 7].map((n) => <option key={n} value={n}>{n ? "★".repeat(n) : "yıldızsız"}</option>)}</Select></Field>
            <Field label="Zincir"><Input value={meta.chain} onChange={(e) => setMeta({ ...meta, chain: e.target.value })} placeholder="Merit" /></Field>
            <Field label="Dış kimlik" hint="Müşteri/Excel kimliği"><Input value={meta.externalId} onChange={(e) => setMeta({ ...meta, externalId: e.target.value })} /></Field>
          </FormGrid>
          <Switch label="Aktif (pasif oteller aramalarda atlanır)" checked={!!meta.isActive} onChange={(e) => setMeta({ ...meta, isActive: e.target.checked })} />
          <Field label="Notlar"><Textarea value={meta.notes} onChange={(e) => setMeta({ ...meta, notes: e.target.value })} rows={3} /></Field>
        </>)}
      </FormModal>

      <Modal open={!!clientsOf} onClose={() => setClientsOf(null)} title={clientsOf ? `Müşteriler · ${clientsOf.hotel.hotelName || clientsOf.hotel.internalHotelId}` : ""} subtitle="Bu oteli listesinde bulunduran müşteriler" footer={<Button variant="secondary" onClick={() => setClientsOf(null)}>Kapat</Button>}>
        {clientsOf && (clientsOf.clients.length ? <DataTable rows={clientsOf.clients} dense pageSize={0}
          columns={[{ key: "name", title: "Müşteri", primary: true, sub: (c) => c.email || "" }, { key: "kind", title: "Tür", render: (c) => <Badge tone={c.kind === "otel" ? "info" : "brand"} dot={false}>{c.kind === "otel" ? "OTEL" : "ACENTE"}</Badge> }, { key: "planCode", title: "Paket", render: (c) => c.planCode ? <code>{c.planCode}</code> : <span className="muted">—</span> }, { key: "isActive", title: "Durum", render: (c) => (c.isActive ? <Badge tone="ok">aktif</Badge> : <Badge tone="bad">pasif</Badge>) }]}
          actions={(c) => [{ icon: "eye", title: "Müşteri detayı", onClick: (x) => { setClientsOf(null); navigate("clientdetail", x.id); } }]} onRowDoubleClick={(c) => { setClientsOf(null); navigate("clientdetail", c.id); }} />
          : <EmptyState icon="users" title="Bu oteli kullanan müşteri yok" text="Müşteri detayından 'Sistem otelinden ekle' ile bağlayabilirsiniz." />)}
      </Modal>
    </div>
  );
}
