import React, { useEffect, useState } from "react";
import { api, fmtDate, fmtRel } from "../api.js";
import { PageHeader, Card, Stat, DataTable, Button, Badge, Alert, Tabs, KV, Skeleton, FormModal, Field, Input, Textarea, Switch, ChangeRecordModal, useConfirm, useToast } from "@kit";

// Site detayi: ayarlar + saglik + bu sitede eslesen oteller + kullanan musteriler + son degisiklikler
export default function SiteDetail({ param: name, navigate }) {
  const [d, setD] = useState(null);
  const [err, setErr] = useState("");
  const [tab, setTab] = useState("hotels");
  const [edit, setEdit] = useState(null);
  const [rec, setRec] = useState(null);
  const confirm = useConfirm(); const toast = useToast();
  const load = () => api("/api/admin/sites/" + name).then(setD).catch((e) => setErr(e.message));
  useEffect(() => { load(); }, [name]);
  if (err) return <Alert tone="bad">{err}</Alert>;
  if (!d) return <div className="sr-stack"><Skeleton h={28} w="40%" /><Skeleton h={90} /><Skeleton h={300} /></div>;
  const s = d.site || {};
  const toggle = async (on) => {
    if (!on && !(await confirm({ title: `${s.label} kapatılsın mı?`, message: "Site tüm müşteriler için kapatılacak (bakım). Portalda 'bakımda' görünür, aramalarda sorgulanmaz.", confirmText: "Kapat", danger: true }))) return;
    try { await api("/api/admin/sites/" + name, { method: "PUT", body: { enabled: on } }); toast.success(on ? "Site açıldı." : "Site kapatıldı (bakım)."); await load(); } catch (e) { toast.error(e.message); }
  };
  const save = async () => { await api("/api/admin/sites/" + name, { method: "PUT", body: { displayName: edit.displayName || "", sortOrder: Number(edit.sortOrder) || 100, note: edit.note || "" } }); setEdit(null); await load(); toast.success("Site ayarları kaydedildi."); };
  const health = () => { if (s.health?.circuitOpen) return <Badge tone="bad">devre açık</Badge>; const c = s.canary; if (!c) return <Badge tone="neutral">bilinmiyor</Badge>; if (c.ok && c.found) return <Badge tone="ok">sağlıklı</Badge>; return <Badge tone="warn" title={c.error || ""}>{c.error ? "kanarya: hata" : "kanarya: fiyat yok"}</Badge>; };
  const hotels = d.hotels || [];
  const fetched = hotels.filter((h) => h.detail?.version > 0).length;
  const hname = (h) => h.hotelName || h.internalHotelId;
  const openHotel = (h) => navigate("hoteldetail", h.internalHotelId);
  const resetCircuit = async () => { try { await api(`/api/admin/providers/${name}/reset-circuit`, { method: "POST" }); toast.success("Devre kesici sıfırlandı."); await load(); } catch (e) { toast.error(e.message); } };
  return (
    <div>
      <PageHeader title={s.label || name} description={<span><code>{name}</code> · {s.domain}{s.note ? ` · ${s.note}` : ""}</span>} actions={<>
        <Button variant="secondary" icon="chevronLeft" onClick={() => navigate("sites")}>Site yönetimi</Button>
        <Button variant="secondary" icon="pencil" onClick={() => setEdit({ displayName: s.displayName || "", sortOrder: s.sortOrder, note: s.note || "" })}>Ayarlar</Button>
        {s.health?.circuitOpen && <Button variant="secondary" icon="refresh" onClick={resetCircuit}>Devreyi sıfırla</Button>}
        <Button variant={s.enabled ? "danger" : "primary"} icon="power" onClick={() => toggle(!s.enabled)}>{s.enabled ? "Bakıma al" : "Aç"}</Button></>}>
        <div className="sr-row center" style={{ gap: 8, marginTop: 8 }}>{s.enabled ? <Badge tone="ok">açık</Badge> : <Badge tone="warn">bakımda</Badge>}{health()}{s.capabilities?.supportsPackages && <Badge tone="info" dot={false}>Paket</Badge>}{s.capabilities?.requiresPuppeteer && <Badge tone="neutral" dot={false}>Tarayıcı</Badge>}{s.supportsDetails && <Badge tone="neutral" dot={false}>Detay</Badge>}</div>
      </PageHeader>
      <div className="sr-grid">
        <Stat icon="building" label="Eşleşen otel" value={hotels.length} sub={`${hotels.filter((h) => h.isActive).length} aktif eşleştirme`} />
        <Stat icon="file" label="Statik detay çekilen" value={fetched} unit={`/ ${hotels.length}`} soft />
        <Stat icon="users" label="Kullanan müşteri" value={(d.clients || []).length} soft />
        <Stat icon="activity" label="Kanarya" value={<span style={{ fontSize: 15 }}>{s.canary ? (s.canary.ok && s.canary.found ? "sağlıklı" : s.canary.error ? "hata" : "fiyat yok") : "—"}</span>} sub={s.canary?.durationMs ? `${s.canary.durationMs} ms` : s.canary?.error || "günlük kontrol"} soft />
      </div>
      <Card style={{ marginTop: 16 }} pad0>
        <div style={{ padding: "6px 20px 0" }}><Tabs value={tab} onChange={setTab} items={[{ value: "hotels", label: "Oteller", icon: "building", count: hotels.length }, { value: "clients", label: "Müşteriler", icon: "users", count: (d.clients || []).length }, { value: "changes", label: "Son değişiklikler", icon: "diff", count: (d.changes || []).length || undefined }, { value: "health", label: "Sağlık", icon: "activity" }]} /></div>
        <div style={{ padding: "0 20px 20px" }}>
          {tab === "hotels" && <DataTable rows={hotels} rowKey={(h) => h.internalHotelId} dense pageSize={50} onRowDoubleClick={openHotel}
            columns={[
              { key: "hotelName", title: "Otel", primary: true, render: hname, sub: (h) => <code>{h.internalHotelId}</code>, sortable: true, sortValue: hname },
              { key: "providerHotelId", title: "Sitedeki kimlik", render: (h) => <code>{h.providerHotelId}</code>, hideOnMobile: true },
              { key: "loc", title: "Konum", render: (h) => <span className="muted">{[h.meta?.city, h.meta?.country].filter(Boolean).join(" · ") || "—"}</span>, hideOnMobile: true },
              { key: "detail", title: "Statik detay", render: (h) => (h.detail?.version > 0 ? <Badge tone="ok" dot={false}>sürüm {h.detail.version}</Badge> : h.detail?.error ? <Badge tone="bad" title={h.detail.error}>hata</Badge> : <Badge tone="neutral">çekilmedi</Badge>), filterValue: (h) => (h.detail?.version > 0 ? "çekildi" : h.detail?.error ? "hata" : "çekilmedi") },
              { key: "changedAt", title: "Son değişiklik", render: (h) => (h.detail?.changedAt ? <span title={fmtDate(h.detail.changedAt)}>{fmtRel(h.detail.changedAt)}</span> : <span className="muted">—</span>), sortable: true, sortValue: (h) => h.detail?.changedAt || "", hideOnMobile: true },
              { key: "isActive", title: "Eşleştirme", render: (h) => (h.isActive ? <Badge tone="ok">aktif</Badge> : <Badge tone="neutral">pasif</Badge>), filterValue: (h) => (h.isActive ? "aktif" : "pasif") },
            ]}
            actions={(h) => [{ icon: "eye", title: "Otel özeti", onClick: openHotel }, { icon: "file", title: "Bu sitedeki statik bilgiler", onClick: (x) => { location.hash = `hotelstatic/${x.internalHotelId}/${name}`; } }, { icon: "external", title: "Sitede aç", hidden: !h.sourceUrl, onClick: (x) => window.open(x.sourceUrl, "_blank", "noreferrer") }]}
            empty={{ icon: "building", title: "Bu siteyle eşleşen otel yok" }} />}
          {tab === "clients" && <DataTable rows={d.clients || []} dense pageSize={50} onRowDoubleClick={(c) => navigate("clientdetail", c.id)}
            columns={[{ key: "name", title: "Müşteri", primary: true, sub: (c) => c.email || "" }, { key: "kind", title: "Tür", render: (c) => <Badge tone={c.kind === "otel" ? "info" : "brand"} dot={false}>{c.kind === "otel" ? "OTEL" : "ACENTE"}</Badge> }, { key: "planCode", title: "Paket", render: (c) => c.planCode ? <code>{c.planCode}</code> : <span className="muted">—</span> }, { key: "clientType", title: "Erişim", render: (c) => c.clientType }, { key: "isActive", title: "Durum", render: (c) => (c.isActive ? <Badge tone="ok">aktif</Badge> : <Badge tone="bad">pasif</Badge>) }]}
            actions={(c) => [{ icon: "eye", title: "Müşteri detayı", onClick: (x) => navigate("clientdetail", x.id) }]} empty={{ icon: "users", title: "Bu siteyi kullanan aktif müşteri yok" }} />}
          {tab === "changes" && <DataTable rows={d.changes || []} dense pageSize={30} onRowDoubleClick={setRec}
            columns={[{ key: "changedAt", title: "Zaman", render: (c) => fmtDate(c.changedAt), sortable: true, sortValue: (c) => c.changedAt, width: 170 }, { key: "internalHotelId", title: "Otel", render: (c) => { const h = hotels.find((x) => x.internalHotelId === c.internalHotelId); return h ? hname(h) : c.internalHotelId; } }, { key: "summary", title: "Özet", primary: true, sub: (c) => `sürüm ${c.version}` }]}
            actions={(c) => [{ icon: "eye", title: "Eski / yeni", onClick: setRec }]} empty={{ icon: "diff", title: "Bu sitede kayıtlı değişiklik yok" }} />}
          {tab === "health" && <KV items={[["Devre kesici", s.health?.circuitOpen ? `açık (site geçici olarak atlanıyor${s.health.circuitOpenUntil ? ", " + fmtDate(s.health.circuitOpenUntil) + " tarihine kadar" : ""})` : "kapalı (normal)"], ["Ardışık hata", s.health?.consecutiveFailures ?? 0], ["Toplam / başarılı / hatalı / bulunamadı", s.health ? `${s.health.total} / ${s.health.success} / ${s.health.failed} / ${s.health.notFound}` : "—"], ["Eşzamanlılık", s.health ? `${s.health.inFlight} / ${s.health.concurrency}` : "—"], ["Ortalama süre", s.health?.avgLatencyMs ? `${s.health.avgLatencyMs} ms` : "—"], ["Son başarılı sorgu", s.health?.lastSuccess ? fmtDate(s.health.lastSuccess) : "—"], ["Kanarya", s.canary ? `${s.canary.ok && s.canary.found ? "sağlıklı" : "sorunlu"}${s.canary.price ? " · " + s.canary.price + " TL" : ""}${s.canary.durationMs ? " · " + s.canary.durationMs + " ms" : ""}${s.canary.error ? " · " + s.canary.error : ""}` : "henüz çalışmadı"], ["Görünen ad", s.displayName || `(varsayılan: ${s.label})`], ["Sıra", s.sortOrder], ["Son ayar", fmtDate(s.updatedAt)]]} />}
        </div>
      </Card>
      <FormModal open={!!edit} onClose={() => setEdit(null)} title={`${s.label} ayarları`} subtitle="Görünen ad müşteri portalında ve raporlarda kullanılır." onSubmit={save}>
        {edit && (<>
          <Field label="Görünen ad" hint={`Boş bırakılırsa varsayılan: ${s.label}`}><Input value={edit.displayName} onChange={(e) => setEdit({ ...edit, displayName: e.target.value })} placeholder={s.label} /></Field>
          <Field label="Sıra" hint="Küçük sayı önce gelir"><Input type="number" value={edit.sortOrder} onChange={(e) => setEdit({ ...edit, sortOrder: e.target.value })} style={{ width: 140 }} /></Field>
          <Field label="Not" hint="Yalnızca admin görür (bakım nedeni, iletişim vb.)"><Textarea value={edit.note} onChange={(e) => setEdit({ ...edit, note: e.target.value })} rows={3} /></Field>
        </>)}
      </FormModal>
      <ChangeRecordModal record={rec} meta={{ label: s.label, hotelName: rec ? (hotels.find((x) => x.internalHotelId === rec.internalHotelId) || {}).hotelName : "" }} onClose={() => setRec(null)} />
    </div>
  );
}
