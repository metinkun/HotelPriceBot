import React, { useEffect, useState } from "react";
import { api } from "../api.js";
import { PageHeader, DataTable, FormModal, Field, Input, Textarea, Switch, Badge, Alert, useConfirm, useToast } from "@kit";

// Site yonetimi: genel ac/kapa (bakim), gorunen ad, sira, not. Siteler kodla entegre edilir; buradan eklenmez.
export default function Sites() {
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");
  const [edit, setEdit] = useState(null);
  const confirm = useConfirm(); const toast = useToast();

  const load = async () => { try { setRows(await api("/api/admin/sites")); setErr(""); } catch (e) { setErr(e.message); } finally { setLoading(false); } };
  useEffect(() => { load(); }, []);

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
      <PageHeader title="Site yönetimi" description="Siteler kodla entegre edilir; buradan genel açık/kapalı, görünen ad ve sıra yönetilir. Kapalı site tüm müşterilerde 'bakımda' görünür ve sorgulanmaz." />
      {err && <Alert tone="bad">{err}</Alert>}
      <DataTable rows={rows} rowKey={(s) => s.name} loading={loading} pageSize={0} onRowDoubleClick={openEdit}
        columns={[
          { key: "label", title: "Site", primary: true, render: (s) => <span style={{ opacity: s.enabled ? 1 : 0.6 }}>{s.label}</span>, sub: (s) => <><code>{s.name}</code> <span className="muted">{s.domain}</span></> },
          { key: "enabled", title: "Durum", render: (s) => <span onClick={(e) => e.stopPropagation()} onDoubleClick={(e) => e.stopPropagation()}><Switch checked={s.enabled} onChange={(e) => toggle(s, e.target.checked)} label={s.enabled ? "açık" : "bakımda"} /></span> },
          { key: "health", title: "Sağlık", render: health },
          { key: "caps", title: "Yetenekler", hideOnMobile: true, render: (s) => <div className="sr-chips" style={{ gap: 4 }}>{s.capabilities?.supportsPackages && <Badge tone="info" dot={false}>Paket</Badge>}{s.capabilities?.requiresPuppeteer && <Badge tone="neutral" dot={false}>Tarayıcı</Badge>}{s.supportsDetails && <Badge tone="neutral" dot={false}>Detay</Badge>}</div> },
          { key: "clientsUsing", title: "Kullanan müşteri", align: "num", sortable: true, hideOnMobile: true },
          { key: "sortOrder", title: "Sıra", align: "num", sortable: true, hideOnMobile: true },
          { key: "note", title: "Not", hideOnMobile: true, render: (s) => <span className="muted truncate" style={{ maxWidth: 220, display: "inline-block" }} title={s.note || ""}>{s.note || "—"}</span> },
        ]}
        actions={(s) => [{ icon: "pencil", title: "Düzenle", onClick: openEdit }]}
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
