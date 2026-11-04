import React, { useEffect, useState } from "react";
import { api } from "../api.js";
import { PageHeader, DataTable, Button, FormModal, Field, Input, FormGrid, Badge, Alert, useConfirm, useToast } from "@kit";

export default function Hotels({ me, navigate }) {
  const [hotels, setHotels] = useState([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");
  const [modal, setModal] = useState(false);
  const [name, setName] = useState("");
  const [urls, setUrls] = useState({});
  const [result, setResult] = useState(null);
  const enabled = me.providers.filter((p) => p.enabled);
  const label = (n) => (me.providers.find((x) => x.name === n) || {}).label || n;
  const confirm = useConfirm(); const toast = useToast();

  const load = () => api("/api/portal/hotels").then(setHotels).catch((e) => setErr(e.message)).finally(() => setLoading(false));
  useEffect(() => { load(); }, []);

  const add = async () => {
    const clean = Object.fromEntries(Object.entries(urls).filter(([, v]) => v && v.trim()));
    if (!Object.keys(clean).length) throw new Error("En az bir site linki girin.");
    try {
      const r = await api("/api/portal/hotels", { method: "POST", body: { hotelName: name, urls: clean } });
      setResult(r); setModal(false); setName(""); setUrls({}); await load();
      toast.success(`Otel eklendi: ${r.hotelName || r.internalHotelId}` + (r.errors?.length ? ` (${r.errors.length} site çözülemedi)` : ""));
    } catch (ex) { throw new Error(ex.message + (ex.data?.details ? " " + ex.data.details.map((d) => `${d.provider}: ${d.error}`).join(" · ") : "")); }
  };
  const remove = async (h) => {
    if (!(await confirm({ title: "Otel listenizden kaldırılsın mı?", message: h.hotelName || h.internalHotelId, confirmText: "Kaldır", danger: true }))) return;
    try { await api("/api/portal/hotels/" + h.internalHotelId, { method: "DELETE" }); toast.success("Otel kaldırıldı."); await load(); } catch (e) { toast.error(e.message); }
  };
  const open = (h) => navigate("hoteldetail", h.internalHotelId);

  return (
    <div>
      <PageHeader title="Otellerim" description="Takip ettiğiniz oteller ve her sitedeki eşleştirmeleri. Satıra çift tıklayarak detayına gidin." actions={<Button icon="plus" onClick={() => setModal(true)}>Otel ekle</Button>} />
      {err && <Alert tone="bad">{err}</Alert>}
      {result?.errors?.length > 0 && <Alert tone="warn">Çözümlenemeyen siteler: {result.errors.map((e) => `${label(e.provider)}: ${e.error}`).join(" · ")}</Alert>}
      <DataTable rows={hotels} rowKey={(h) => h.internalHotelId} loading={loading} search={hotels.length > 8 ? { placeholder: "Otel ara…", keys: ["hotelName", "internalHotelId"] } : undefined}
        columns={[
          { key: "hotelName", title: "Otel", primary: true, sortable: true, render: (h) => h.hotelName || h.internalHotelId, sub: (h) => <code>{h.internalHotelId}</code> },
          { key: "providers", title: "Siteler", render: (h) => <div className="sr-chips" style={{ gap: 4 }}>{h.providers.map((p) => <Badge key={p.provider} tone={p.enabled ? "info" : "neutral"} dot={false} title={p.sourceUrl || ""}>{label(p.provider)}</Badge>)}</div>, hideOnMobile: true },
          { key: "count", title: "Site", align: "num", width: 70, sortable: true, sortValue: (h) => h.providers.length, render: (h) => h.providers.length },
        ]}
        actions={(h) => [{ icon: "eye", title: "Detay", onClick: open }, { icon: "search", title: "Fiyat ara", tone: "primary", onClick: (x) => navigate("search", x.internalHotelId) }, { icon: "trash", title: "Kaldır", tone: "danger", onClick: remove }]}
        onRowDoubleClick={open} empty={{ icon: "building", title: "Henüz otel eklenmedi", text: "Takip etmek istediğiniz otelleri site linkleriyle ekleyin.", action: <Button icon="plus" onClick={() => setModal(true)}>Otel ekle</Button> }} />

      <FormModal open={modal} onClose={() => setModal(false)} title="Yeni otel ekle" subtitle="Otelin her sitedeki detay sayfası linkini yapıştırın. Sistem otel kimliğini otomatik çözer; en az bir site yeterlidir." size="md" onSubmit={add} submitText="Oteli ekle">
        <Field label="Otel adı" required><Input value={name} onChange={(e) => setName(e.target.value)} required autoFocus placeholder="Örn. Acapulco Resort" /></Field>
        <FormGrid>{enabled.map((p) => <Field key={p.name} label={p.label} hint={p.domain}><Input placeholder={"https://www." + p.domain + "/…"} value={urls[p.name] || ""} onChange={(e) => setUrls({ ...urls, [p.name]: e.target.value })} /></Field>)}</FormGrid>
        <p className="muted small">Çözümleme sitelere göre birkaç saniye, tarayıcı tabanlı sitelerde 20-60 saniye sürebilir.</p>
      </FormModal>
    </div>
  );
}
