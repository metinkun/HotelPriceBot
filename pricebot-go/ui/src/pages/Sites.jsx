import React, { useEffect, useState } from "react";
import { api } from "../api.js";
import { PageHeader, DataTable, FormModal, Field, Input, Textarea, Switch, Badge, Alert, Card, KV, useConfirm, useToast } from "@kit";
import { fmtDate } from "../api.js";

// Site yonetimi: genel ac/kapa (bakim), gorunen ad, sira, not. Siteler kodla entegre edilir; buradan eklenmez.
export default function Sites({ navigate }) {
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");
  const [edit, setEdit] = useState(null);
  const [proxy, setProxy] = useState(null);
  const confirm = useConfirm(); const toast = useToast();
  const openDetail = (s) => navigate("sitedetail", s.name);

  const load = async () => { try { setRows(await api("/api/admin/sites")); setErr(""); } catch (e) { setErr(e.message); } finally { setLoading(false); } };
  useEffect(() => { load(); api("/api/admin/proxies/status").then(setProxy).catch(() => {}); }, []);

  const toggle = async (s, on) => {
    if (!on && !(await confirm({ title: `${s.label} kapatılsın mı?`, message: "Site tüm müşteriler için kapatılacak (bakım). Portalda 'bakımda' görünür, aramalarda sorgulanmaz.", confirmText: "Kapat", danger: true }))) return;
    try { await api("/api/admin/sites/" + s.name, { method: "PUT", body: { enabled: on } }); toast.success(on ? `${s.label} açıldı.` : `${s.label} kapatıldı (bakım).`); await load(); } catch (e) { toast.error(e.message); }
  };
  const save = async () => {
    await api("/api/admin/sites/" + edit.name, { method: "PUT", body: { displayName: edit.displayName || "", sortOrder: Number(edit.sortOrder) || 100, note: edit.note || "" } });
    setEdit(null); await load(); toast.success("Site ayarları kaydedildi.");
  };
  const health = (s) => {
    if (s.health?.circuitOpen) return <Badge tone="bad">devre açık</Badge>;
    const c = s.canary;
    if (!c) return <Badge tone="neutral">bilinmiyor</Badge>;
    if (c.ok && c.found) return <Badge tone="ok">sağlıklı</Badge>;
    return <Badge tone="warn" title={c.error || ""}>{c.error ? "kanarya: hata" : "kanarya: fiyat yok"}</Badge>;
  };
  const openEdit = (s) => setEdit({ name: s.name, label: s.label, displayName: s.displayName || "", sortOrder: s.sortOrder, note: s.note || "" });

  return (
    <div>
      <PageHeader title="Site yönetimi" description="Siteler kodla entegre edilir; buradan genel açık/kapalı, görünen ad ve sıra yönetilir. Kapalı site tüm müşterilerde 'bakımda' görünür ve sorgulanmaz. Çift tıklayınca site detayı (oteller, müşteriler, değişiklikler) açılır." />
      {err && <Alert tone="bad">{err}</Alert>}
      {proxy && (
        <Card title="IP rotasyonu (proxy)" subtitle="Sitelere giden istekler havuzdaki proxy adreslerinden sırayla çıkar; tek IP'den çok istek gidip banlanma riski azalır." style={{ marginBottom: 16 }}>
          <div className="sr-row center" style={{ gap: 10, flexWrap: "wrap" }}>
            <Badge tone={proxy.mode === "rotating" ? "ok" : proxy.mode === "static" ? "warn" : "bad"}>{proxy.mode === "rotating" ? `dönüşümlü · ${proxy.poolSize} proxy` : proxy.mode === "static" ? `tek proxy · ${proxy.staticHost}` : "proxy yok (doğrudan IP)"}</Badge>
            <Badge tone={proxy.browserProxy ? "ok" : "neutral"} dot={false}>tarayıcı (Touristica): {proxy.browserProxy ? "proxy'li" : "doğrudan"}</Badge>
            {proxy.lastSync && <span className="muted small">son senkron {fmtDate(proxy.lastSync)}</span>}
            {proxy.served > 0 && <span className="muted small">{proxy.served} istek proxy üzerinden</span>}
            {proxy.lastError && <Badge tone="bad" title={proxy.lastError}>senkron hatası</Badge>}
          </div>
          {proxy.mode === "direct" && <Alert tone="warn" className="" >Proxy tanımlı değil: tüm istekler sunucunun kendi IP'sinden çıkıyor. WEBSHARE_API_KEY (dönüşümlü havuz) veya PROXY_HOST/PORT (tek proxy) ve BROWSER_PROXY_URL ortam değişkenlerini ayarlayın; servis 5 dakikada bir listeyi yeniler.</Alert>}
        </Card>
      )}
      <DataTable rows={rows} rowKey={(s) => s.name} loading={loading} pageSize={0} onRowDoubleClick={openDetail}
        columns={[
          { key: "label", title: "Site", primary: true, render: (s) => <span style={{ opacity: s.enabled ? 1 : 0.6 }}>{s.label}</span>, sub: (s) => <><code>{s.name}</code> <span className="muted">{s.domain}</span></> },
          { key: "enabled", title: "Durum", render: (s) => <span onClick={(e) => e.stopPropagation()} onDoubleClick={(e) => e.stopPropagation()}><Switch checked={s.enabled} onChange={(e) => toggle(s, e.target.checked)} label={s.enabled ? "açık" : "bakımda"} /></span> },
          { key: "health", title: "Sağlık", render: health, filterValue: (s) => (s.health?.circuitOpen ? "devre açık" : s.canary ? (s.canary.ok && s.canary.found ? "sağlıklı" : "kanarya hata") : "bilinmiyor") },
          { key: "hotelCount", title: "Otel", align: "num", sortable: true, render: (s) => s.hotelCount ?? "—" },
          { key: "caps", title: "Yetenekler", hideOnMobile: true, render: (s) => <div className="sr-chips" style={{ gap: 4 }}>{s.capabilities?.supportsPackages && <Badge tone="info" dot={false}>Paket</Badge>}{s.capabilities?.requiresPuppeteer && <Badge tone="neutral" dot={false}>Tarayıcı</Badge>}{s.supportsDetails && <Badge tone="neutral" dot={false}>Detay</Badge>}</div> },
          { key: "clientsUsing", title: "Kullanan müşteri", align: "num", sortable: true, hideOnMobile: true },
          { key: "sortOrder", title: "Sıra", align: "num", sortable: true, hideOnMobile: true },
          { key: "note", title: "Not", hideOnMobile: true, render: (s) => <span className="muted truncate" style={{ maxWidth: 220, display: "inline-block" }} title={s.note || ""}>{s.note || "—"}</span> },
        ]}
        actions={(s) => [{ icon: "eye", title: "Detay (oteller, müşteriler)", onClick: openDetail }, { icon: "pencil", title: "Ayarlar", onClick: openEdit }]}
        empty={{ icon: "globe", title: "Site yok" }} />

      <FormModal open={!!edit} onClose={() => setEdit(null)} title={edit ? `${edit.label} ayarları` : ""} subtitle="Görünen ad müşteri portalında ve raporlarda kullanılır." onSubmit={save}>
        {edit && (<>
          <Field label="Görünen ad" hint={`Boş bırakılırsa varsayılan: ${edit.label}`}><Input value={edit.displayName} onChange={(e) => setEdit({ ...edit, displayName: e.target.value })} placeholder={edit.label} /></Field>
          <Field label="Sıra" hint="Küçük sayı önce gelir"><Input type="number" value={edit.sortOrder} onChange={(e) => setEdit({ ...edit, sortOrder: e.target.value })} style={{ width: 140 }} /></Field>
          <Field label="Not" hint="Yalnızca admin görür (bakım nedeni, iletişim vb.)"><Textarea value={edit.note} onChange={(e) => setEdit({ ...edit, note: e.target.value })} rows={3} /></Field>
        </>)}
      </FormModal>
    </div>
  );
}
