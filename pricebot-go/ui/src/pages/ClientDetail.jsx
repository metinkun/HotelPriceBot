import React, { useEffect, useState } from "react";
import { api, fmtDate, parseReq, WEEKDAYS_SHORT } from "../api.js";
import { PageHeader, Card, Stat, DataTable, Button, IconButton, FormModal, Field, Input, Select, FormGrid, Badge, Alert, Tabs, Chip, KeyBox, Switch, Skeleton, useConfirm, usePrompt, useToast, statusTone, statusLabel } from "@kit";

const TYPE = { api: "API entegrasyonu", portal: "Web portal", both: "API + portal" };

// Musteri detay sayfasi: ozet kartlari + sekmeler (Siteler, API anahtarlari, Portal kullanicilari, Oteller, Planli aramalar, Son aramalar)
export default function ClientDetail({ param: clientId, navigate }) {
  const [sel, setSel] = useState(null); // {client, keys, usedToday}
  const [data, setData] = useState(null); // settings: providers, settings, portalUsers, hotels, schedules
  const [searches, setSearches] = useState([]);
  const [plans, setPlans] = useState([]);
  const [tab, setTab] = useState("sites");
  const [err, setErr] = useState("");
  const [newKey, setNewKey] = useState(null);
  const [userModal, setUserModal] = useState(false);
  const [newUser, setNewUser] = useState({ email: "", password: "", name: "" });
  const confirm = useConfirm(); const prompt = usePrompt(); const toast = useToast();

  const load = async () => {
    try {
      const [s, d, sr, p] = await Promise.all([api("/api/admin/clients/" + clientId), api(`/api/admin/clients/${clientId}/settings`), api(`/api/admin/clients/${clientId}/searches?limit=30`), api("/api/admin/plans")]);
      setSel(s); setData(d); setSearches(sr); setPlans(p); setErr("");
    } catch (e) { setErr(e.message); }
  };
  useEffect(() => { load(); }, [clientId]);

  if (err) return <Alert tone="bad">{err}</Alert>;
  if (!sel || !data) return <div className="sr-stack"><Skeleton h={28} w="40%" /><Skeleton h={90} /><Skeleton h={240} /></div>;
  const c = sel.client; const st = data.settings;
  const allOn = st.enabledProviders.length === 0;
  const enabled = new Set(st.enabledProviders);

  const update = async (patch, msg) => { try { await api("/api/admin/clients/" + c.id, { method: "PUT", body: patch }); await load(); if (msg) toast.success(msg); } catch (e) { toast.error(e.message); } };
  const saveSettings = async (patch) => { try { await api(`/api/admin/clients/${clientId}/settings`, { method: "PUT", body: patch }); await load(); } catch (e) { toast.error(e.message); } };
  const toggleProv = (n) => { let list = allOn ? [...data.providers] : [...st.enabledProviders]; list = list.includes(n) ? list.filter((x) => x !== n) : [...list, n]; saveSettings({ enabledProviders: list }); };
  const remove = async () => {
    if (!(await confirm({ title: "Müşteri silinsin mi?", message: `${c.name} ve tüm API anahtarları kalıcı olarak silinecek.`, confirmText: "Sil", danger: true }))) return;
    try { await api("/api/admin/clients/" + c.id, { method: "DELETE" }); toast.success("Müşteri silindi."); navigate("clients"); } catch (e) { toast.error(e.message); }
  };
  const createKey = async () => {
    const label = await prompt({ title: "Yeni API anahtarı", label: "Etiket (opsiyonel)", placeholder: "örn. üretim, test" });
    if (label === null) return;
    try { const r = await api(`/api/admin/clients/${c.id}/keys`, { method: "POST", body: { label: label || null } }); setNewKey(r.key); setTab("keys"); await load(); } catch (e) { toast.error(e.message); }
  };
  const revoke = async (k) => { if (!(await confirm({ title: "Anahtar iptal edilsin mi?", message: `${k.keyPrefix}… anahtarı bir daha kullanılamaz.`, confirmText: "İptal et", danger: true }))) return; try { await api("/api/admin/keys/" + k.id, { method: "DELETE" }); toast.success("Anahtar iptal edildi."); await load(); } catch (e) { toast.error(e.message); } };
  const createUser = async () => { await api(`/api/admin/clients/${clientId}/portal-users`, { method: "POST", body: { ...newUser, name: newUser.name || null } }); setUserModal(false); setNewUser({ email: "", password: "", name: "" }); toast.success("Portal kullanıcısı eklendi."); await load(); };
  const userAction = async (u, patch, msg) => { try { await api(`/api/admin/portal-users/${u.id}`, { method: "PUT", body: patch }); await load(); toast.success(msg); } catch (e) { toast.error(e.message); } };
  const resetPw = async (u) => { const p = await prompt({ title: "Şifre sıfırla", label: `${u.email} için yeni şifre`, type: "password", minLength: 8, required: true }); if (p && p.length >= 8) userAction(u, { password: p }, "Şifre güncellendi."); else if (p) toast.error("Şifre en az 8 karakter olmalı."); };
  const delUser = async (u) => { if (!(await confirm({ title: "Kullanıcı silinsin mi?", message: u.email, confirmText: "Sil", danger: true }))) return; try { await api(`/api/admin/portal-users/${u.id}`, { method: "DELETE" }); toast.success("Kullanıcı silindi."); await load(); } catch (e) { toast.error(e.message); } };

  const tabs = [
    { value: "sites", label: "Siteler", icon: "globe", count: allOn ? data.providers.length : st.enabledProviders.length },
    { value: "keys", label: "API anahtarları", icon: "key", count: sel.keys.filter((k) => !k.revokedAt).length },
    { value: "users", label: "Portal kullanıcıları", icon: "user", count: data.portalUsers.length },
    { value: "hotels", label: "Oteller", icon: "building", count: data.hotels.length },
    { value: "schedules", label: "Planlı aramalar", icon: "calendar", count: data.schedules.length },
    { value: "searches", label: "Son aramalar", icon: "history" },
  ];

  return (
    <div>
      <PageHeader title={c.name} description={c.email || "e-posta yok"} actions={<>
        <Button variant="secondary" icon="chevronLeft" onClick={() => navigate("clients")}>Müşteriler</Button>
        <Button variant="secondary" icon="power" onClick={() => update({ isActive: !c.isActive }, c.isActive ? "Pasife alındı." : "Aktifleştirildi.")}>{c.isActive ? "Pasife al" : "Aktifleştir"}</Button>
        <Button variant="danger" icon="trash" onClick={remove}>Sil</Button></>}>
        <div className="sr-row center" style={{ gap: 8, marginTop: 8 }}>{c.isActive ? <Badge tone="ok">aktif</Badge> : <Badge tone="bad">pasif</Badge>}<Badge tone="info" dot={false}>#{c.id}</Badge><Badge tone="neutral" dot={false}>{TYPE[st.clientType]}</Badge></div>
      </PageHeader>

      <div className="sr-grid">
        <Card title="Plan ve tip" flat>
          <FormGrid>
            <Field label="Plan"><Select value={c.planId ?? ""} onChange={(e) => update({ planId: e.target.value ? Number(e.target.value) : null }, "Plan güncellendi.")}><option value="">(plansız = sınırsız)</option>{plans.map((p) => <option key={p.id} value={p.id}>{p.name} ({p.code})</option>)}</Select></Field>
            <Field label="Müşteri tipi"><Select value={st.clientType} onChange={(e) => saveSettings({ clientType: e.target.value })}><option value="api">API entegrasyonu</option><option value="portal">Web portal</option><option value="both">API + portal</option></Select></Field>
          </FormGrid>
          <p className="muted small" style={{ marginTop: 10 }}>Portal: <a href="/portal/" target="_blank" rel="noreferrer">/portal/</a> · API: <code>X-API-Key</code> ile <code>/api/v1/*</code></p>
        </Card>
        <Stat icon="zap" label="Bugünkü kredi" value={sel.usedToday} sub={c.planCode ? `plan: ${c.planCode}` : "plansız"} />
        <Stat icon="key" label="Aktif anahtar" value={sel.keys.filter((k) => !k.revokedAt).length} soft />
        <Stat icon="building" label="Otel" value={data.hotels.length} soft />
      </div>

      <Card style={{ marginTop: 16 }} pad0>
        <div style={{ padding: "6px 20px 0" }}><Tabs value={tab} onChange={setTab} items={tabs} /></div>
        <div style={{ padding: "0 20px 20px" }}>
          {tab === "sites" && (<>
            <div className="sr-row between center" style={{ marginBottom: 12 }}><p className="muted">Kapalı siteler portalda görünmez, API'de 403 döner. Plan kısıtları ayrıca uygulanır.</p>{allOn && <Badge tone="ok">tüm siteler açık</Badge>}</div>
            <div className="sr-chips">{data.providers.map((n) => { const on = allOn || enabled.has(n); return <Chip key={n} on={on} onClick={() => toggleProv(n)}>{on ? "✓ " : ""}{n}</Chip>; })}</div>
          </>)}
          {tab === "keys" && (<>
            {newKey && <div style={{ marginBottom: 12 }}><KeyBox label="Yeni anahtar — yalnızca bir kez gösterilir, şimdi kopyalayın:" value={newKey} /><div className="sr-row" style={{ marginTop: 8 }}><Button size="sm" variant="secondary" icon="copy" onClick={() => { navigator.clipboard?.writeText(newKey); toast.success("Kopyalandı."); }}>Kopyala</Button><Button size="sm" variant="ghost" onClick={() => setNewKey(null)}>Gizle</Button></div></div>}
            <DataTable rows={sel.keys} dense toolbarRight={<Button size="sm" icon="plus" onClick={createKey}>Yeni anahtar</Button>}
              columns={[{ key: "keyPrefix", title: "Anahtar", primary: true, render: (k) => <code>{k.keyPrefix}…</code> }, { key: "label", title: "Etiket", render: (k) => k.label || <span className="muted">—</span> }, { key: "lastUsedAt", title: "Son kullanım", render: (k) => fmtDate(k.lastUsedAt), hideOnMobile: true }, { key: "createdAt", title: "Oluşturma", render: (k) => fmtDate(k.createdAt), hideOnMobile: true }, { key: "revokedAt", title: "Durum", render: (k) => (k.revokedAt ? <Badge tone="bad">iptal</Badge> : <Badge tone="ok">aktif</Badge>) }]}
              actions={(k) => [{ icon: "trash", title: "İptal et", tone: "danger", hidden: !!k.revokedAt, onClick: revoke }]} empty={{ icon: "key", title: "Anahtar yok" }} />
          </>)}
          {tab === "users" && (
            <DataTable rows={data.portalUsers} dense toolbarRight={<Button size="sm" icon="plus" onClick={() => setUserModal(true)}>Kullanıcı ekle</Button>}
              columns={[{ key: "email", title: "E-posta", primary: true, sub: (u) => u.name || "" }, { key: "lastLoginAt", title: "Son giriş", render: (u) => fmtDate(u.lastLoginAt) }, { key: "isActive", title: "Durum", render: (u) => (u.isActive ? <Badge tone="ok">aktif</Badge> : <Badge tone="bad">pasif</Badge>) }]}
              actions={(u) => [{ icon: "key", title: "Şifre sıfırla", onClick: resetPw }, { icon: "power", title: u.isActive ? "Pasife al" : "Aktifleştir", onClick: (x) => userAction(x, { isActive: !x.isActive }, x.isActive ? "Pasife alındı." : "Aktifleştirildi.") }, { icon: "trash", title: "Sil", tone: "danger", onClick: delUser }]}
              empty={{ icon: "user", title: "Portal kullanıcısı yok", text: "Müşteri portala girecekse en az bir kullanıcı ekleyin." }} />
          )}
          {tab === "hotels" && (
            <DataTable rows={data.hotels} dense columns={[{ key: "internalHotelId", title: "Otel", primary: true, render: (h) => h.label || h.internalHotelId, sub: (h) => <code>{h.internalHotelId}</code> }]}
              actions={(h) => [{ icon: "eye", title: "Detay", onClick: (x) => navigate("hoteldetail", x.internalHotelId) }]} onRowDoubleClick={(h) => navigate("hoteldetail", h.internalHotelId)} empty={{ icon: "building", title: "Müşteriye bağlı otel yok", text: "Portal kullanıcısı Otellerim sayfasından ekler." }} />
          )}
          {tab === "schedules" && (
            <DataTable rows={data.schedules} dense columns={[{ key: "name", title: "Plan", primary: true }, { key: "time", title: "Zaman", render: (s) => `${s.weekdays.length ? s.weekdays.map((d) => WEEKDAYS_SHORT[d]).join(", ") : "Her gün"} ${s.runTime}` }, { key: "nextRunAt", title: "Sonraki", render: (s) => fmtDate(s.nextRunAt) }, { key: "isActive", title: "Durum", render: (s) => (s.isActive ? <Badge tone="ok">aktif</Badge> : <Badge tone="neutral">pasif</Badge>) }]} empty={{ icon: "calendar", title: "Planlı arama yok" }} />
          )}
          {tab === "searches" && (
            <DataTable rows={searches} dense columns={[{ key: "createdAt", title: "Zaman", primary: true, render: (s) => fmtDate(s.createdAt) }, { key: "kind", title: "Tür", render: (s) => (s.kind === "packages" ? "Paket" : "Otel") + " · " + (parseReq(s.request).hotelIds || []).length + " otel" }, { key: "status", title: "Durum", render: (s) => <Badge tone={statusTone(s.status)}>{statusLabel(s.status)}</Badge> }, { key: "scheduleId", title: "Kaynak", render: (s) => (s.scheduleId ? <Badge tone="info" dot={false}>planlı</Badge> : <Badge tone="neutral" dot={false}>anlık</Badge>) }]} empty={{ icon: "history", title: "Arama kaydı yok" }} />
          )}
        </div>
      </Card>

      <FormModal open={userModal} onClose={() => setUserModal(false)} title="Portal kullanıcısı ekle" onSubmit={createUser} submitText="Ekle">
        <Field label="E-posta" required><Input type="email" value={newUser.email} onChange={(e) => setNewUser({ ...newUser, email: e.target.value })} required autoFocus /></Field>
        <FormGrid>
          <Field label="Şifre" required hint="En az 8 karakter"><Input type="password" minLength={8} value={newUser.password} onChange={(e) => setNewUser({ ...newUser, password: e.target.value })} required /></Field>
          <Field label="Ad"><Input value={newUser.name} onChange={(e) => setNewUser({ ...newUser, name: e.target.value })} /></Field>
        </FormGrid>
      </FormModal>
    </div>
  );
}
