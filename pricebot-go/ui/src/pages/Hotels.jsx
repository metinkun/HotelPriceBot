import React, { useEffect, useState } from "react";
import { api } from "../api.js";
import { PageHeader, DataTable, Button, FormModal, Field, Input, FormGrid, Badge, Alert, useConfirm, useToast } from "@kit";

export default function Hotels({ navigate }) {
  const [hotels, setHotels] = useState([]);
  const [providers, setProviders] = useState([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");
  const [modal, setModal] = useState(false);
  const [name, setName] = useState("");
  const [urls, setUrls] = useState({});
  const [result, setResult] = useState(null);
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
  const remove = async (h) => {
    if (!(await confirm({ title: "Otel silinsin mi?", message: `${h.hotelName || h.internalHotelId} ve tüm site eşleştirmeleri silinecek.`, confirmText: "Sil", danger: true }))) return;
    try { await api("/api/admin/hotels/" + h.internalHotelId, { method: "DELETE" }); toast.success("Otel silindi."); await load(); } catch (e) { toast.error(e.message); }
  };
  const open = (h) => navigate("hoteldetail", h.internalHotelId);

  return (
    <div>
      <PageHeader title="Oteller" description="Sistemdeki oteller ve her sitedeki eşleştirmeleri. Satıra çift tıklayarak detayına gidin." actions={<><Button variant="secondary" icon="upload" onClick={() => navigate("import")}>Toplu ekle</Button><Button icon="plus" onClick={() => setModal(true)}>Otel ekle</Button></>} />
      {err && <Alert tone="bad">{err}</Alert>}
      {result?.errors?.length > 0 && <Alert tone="warn">Çözümlenemeyen siteler: {result.errors.map((e) => `${e.provider}: ${e.error}`).join(" · ")}</Alert>}
      <DataTable rows={hotels} rowKey={(h) => h.internalHotelId} loading={loading} search={{ placeholder: "Otel adı veya kimlik ara…", keys: ["hotelName", "internalHotelId"] }}
        columns={[
          { key: "hotelName", title: "Otel", primary: true, sortable: true, sub: (h) => <code>{h.internalHotelId}</code> },
          { key: "providers", title: "Siteler", render: (h) => <div className="sr-chips" style={{ gap: 4 }}>{h.providers.map((p) => <Badge key={p.provider} tone={p.isActive ? "info" : "neutral"} dot={false} title={p.sourceUrl || ""}>{p.provider}</Badge>)}</div> },
          { key: "count", title: "Site", align: "num", width: 70, sortable: true, sortValue: (h) => h.providers.length, render: (h) => h.providers.length, hideOnMobile: true },
        ]}
        actions={(h) => [{ icon: "eye", title: "Detay", onClick: open }, { icon: "trash", title: "Sil", tone: "danger", onClick: remove }]}
        onRowDoubleClick={open} empty={{ icon: "building", title: "Henüz otel yok", text: "URL ile tek tek veya listeyle toplu ekleyebilirsiniz.", action: <Button icon="plus" onClick={() => setModal(true)}>Otel ekle</Button> }} />

      <FormModal open={modal} onClose={() => setModal(false)} title="URL'lerden otel ekle" subtitle="Her site için otelin detay sayfası linkini yapıştırın; sistem otel kimliğini kendisi çözer. En az bir site yeterlidir." size="md" onSubmit={add} submitText="Ekle">
        <Field label="Otel adı" required><Input value={name} onChange={(e) => setName(e.target.value)} required placeholder="Örn. Merit Crystal Cove Hotel" /></Field>
        <FormGrid>{providers.map((p) => <Field key={p} label={p}><Input placeholder="https://…" value={urls[p] || ""} onChange={(e) => setUrls({ ...urls, [p]: e.target.value })} /></Field>)}</FormGrid>
        <p className="muted small">Touristica çözümlemesi 20-60 saniye sürebilir.</p>
      </FormModal>
    </div>
  );
}
