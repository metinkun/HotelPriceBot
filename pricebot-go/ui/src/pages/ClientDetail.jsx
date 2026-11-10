import React, { useEffect, useState } from "react";
import { api, fmtDate, parseReq, WEEKDAYS_SHORT } from "../api.js";
import { PageHeader, Card, Stat, DataTable, Button, Modal, FormModal, Field, Input, Select, FormGrid, Badge, Alert, Tabs, KeyBox, Switch, Skeleton, KV, SitePicker, SearchDetailModal, useConfirm, usePrompt, useToast, statusTone, statusLabel } from "@kit";
import { loginLogColumns } from "./LoginLogs.jsx";

const TYPE = { api: "API entegrasyonu", portal: "Web portal", both: "API + portal" };
const KIND = { acente: "ACENTE", otel: "OTEL" };
const LIMIT_ROWS = [
  ["maxSites", "Site sayısı", (u) => `${u.sites} seçili`],
  ["maxHotels", "Otel sayısı", (u) => `${u.hotels} otel`],
  ["maxUsers", "Kullanıcı sayısı", (u) => `${u.users} kullanıcı`],
  ["dailySearches", "Günlük anlık arama", (u) => `${u.today.searches} bugün`],
  ["dailyScheduledRuns", "Günlük planlı görev", (u) => `${u.today.scheduledRuns} bugün`],
  ["dailyQuota", "Site çağrısı / gün (iç)", (u) => `${u.today.siteCalls} bugün`],
  ["ratePerMinute", "İstek / dakika", () => ""],
  ["maxConcurrency", "Eşzamanlı sorgu", () => ""],
  ["maxDevicesPc", "Bilgisayar (kullanıcı başına)", () => ""],
  ["maxDevicesMobile", "Mobil cihaz (kullanıcı başına)", () => ""],
];
const inf = (v) => (v ? v : "∞");

// Musteri detay sayfasi: ozet kartlari + sekmeler (Limitler, Siteler, API anahtarlari, Portal kullanicilari, Oteller, Planli aramalar, Son aramalar, Giris loglari)
export default function ClientDetail({ param: clientId, navigate }) {
  const [sel, setSel] = useState(null); // {client, keys, usedToday}
  const [data, setData] = useState(null); // settings: providers, settings, portalUsers, hotels, schedules, client, limits, devices, sitesOpen
  const [searches, setSearches] = useState([]);
  const [plans, setPlans] = useState([]);
  const [allHotels, setAllHotels] = useState([]);
  const [lim, setLim] = useState(null); // GET limits
  const [limEdit, setLimEdit] = useState(null);
  const [logs, setLogs] = useState(null);
  const [tab, setTab] = useState("limits");
  const [err, setErr] = useState("");
  const [newKey, setNewKey] = useState(null);
  const [userModal, setUserModal] = useState(false);
  const [newUser, setNewUser] = useState({ email: "", password: "", name: "" });
  const [devUser, setDevUser] = useState(null); // {user, devices}
  const [hotelModal, setHotelModal] = useState(false);
  const [linkHotel, setLinkHotel] = useState({ internalHotelId: "", label: "" });
  const [logUser, setLogUser] = useState(""); // giris loglari: kullanici filtresi (email)
  const [resetInfo, setResetInfo] = useState(null); // sifre sifirlama linki sonucu
  const [userEdit, setUserEdit] = useState(null); // {id, email, name, pc, mobile}
  const [searchDetail, setSearchDetail] = useState(null);
  const [sites, setSites] = useState([]);
  const confirm = useConfirm(); const prompt = usePrompt(); const toast = useToast();

  const load = async () => {
    try {
      const [s, d, sr, p, l, hs] = await Promise.all([api("/api/admin/clients/" + clientId), api(`/api/admin/clients/${clientId}/settings`), api(`/api/admin/clients/${clientId}/searches?limit=30`), api("/api/admin/plans"), api(`/api/admin/clients/${clientId}/limits`), api("/api/admin/hotels")]);
      setSel(s); setData(d); setSearches(sr); setPlans(p); setLim(l); setLimEdit({ ...l.limits }); setAllHotels(hs); setErr("");
    } catch (e) { setErr(e.message); }
  };
  useEffect(() => { load(); api("/api/admin/sites").then(setSites).catch(() => {}); }, [clientId]);
  useEffect(() => { if (tab === "logins" && logs === null) api(`/api/admin/login-logs?clientId=${clientId}&limit=100`).then(setLogs).catch((e) => toast.error(e.message)); }, [tab]);

  if (err) return <Alert tone="bad">{err}</Alert>;
  if (!sel || !data || !lim) return <div className="sr-stack"><Skeleton h={28} w="40%" /><Skeleton h={90} /><Skeleton h={240} /></div>;
  const c = sel.client; const st = data.settings;
  const allOn = st.enabledProviders.length === 0;
  const enabled = new Set(st.enabledProviders);
  const sitesOpen = new Set(data.sitesOpen || data.providers);
  const selectedSites = allOn ? data.providers.length : st.enabledProviders.length;
  const hotelName = (id) => { const h = allHotels.find((x) => x.internalHotelId === id); return h?.hotelName || data.hotels.find((x) => x.internalHotelId === id)?.label || id; };
  const demoExpired = c.demoExpiresAt && new Date(c.demoExpiresAt) < new Date();

  const update = async (patch, msg) => { try { await api("/api/admin/clients/" + c.id, { method: "PUT", body: patch }); await load(); if (msg) toast.success(msg); } catch (e) { toast.error(e.message); } };
  const saveSettings = async (patch) => { try { await api(`/api/admin/clients/${clientId}/settings`, { method: "PUT", body: patch }); await load(); } catch (e) { toast.error(e.message); } };
  const toggleProv = (n) => { let list = allOn ? [...data.providers] : [...st.enabledProviders]; list = list.includes(n) ? list.filter((x) => x !== n) : [...list, n]; saveSettings({ enabledProviders: list }); };
  const createKey = async () => {
    const label = await prompt({ title: "Yeni API anahtarı", label: "Etiket (opsiyonel)", placeholder: "örn. üretim, test" });
    if (label === null) return;
    try { const r = await api(`/api/admin/clients/${c.id}/keys`, { method: "POST", body: { label: label || null } }); setNewKey(r.key); setTab("keys"); await load(); } catch (e) { toast.error(e.message); }
  };
  const revoke = async (k) => { if (!(await confirm({ title: "Anahtar iptal edilsin mi?", message: `${k.keyPrefix}… anahtarı bir daha kullanılamaz.`, confirmText: "İptal et", danger: true }))) return; try { await api("/api/admin/keys/" + k.id, { method: "DELETE" }); toast.success("Anahtar iptal edildi."); await load(); } catch (e) { toast.error(e.message); } };
  const createUser = async () => { await api(`/api/admin/clients/${clientId}/portal-users`, { method: "POST", body: { ...newUser, name: newUser.name || null } }); setUserModal(false); setNewUser({ email: "", password: "", name: "" }); toast.success("Portal kullanıcısı eklendi."); await load(); };
  const userAction = async (u, patch, msg) => { try { await api(`/api/admin/portal-users/${u.id}`, { method: "PUT", body: patch }); await load(); toast.success(msg); } catch (e) { toast.error(e.message); } };
  const resetPw = async (u) => { const p = await prompt({ title: "Şifre belirle", message: "Şifre yönetimi bizdedir; müşteriye iletilecek yeni şifreyi girin. Linkle yenileme için 'Şifre sıfırlama linki' kullanın.", label: `${u.email} için yeni şifre`, type: "password", minLength: 8, required: true }); if (p && p.length >= 8) userAction(u, { password: p }, "Şifre güncellendi."); else if (p) toast.error("Şifre en az 8 karakter olmalı."); };
  const resetLink = async (u) => {
    if (!(await confirm({ title: "Şifre sıfırlama linki oluşturulsun mu?", message: `${u.email} için tek kullanımlık, süreli bir link üretilir; e-posta ayarlıysa kullanıcıya gönderilir. Önceki linkler geçersiz olur.`, confirmText: "Oluştur" }))) return;
    try { const r = await api(`/api/admin/portal-users/${u.id}/reset-link`, { method: "POST", body: {} }); setResetInfo({ ...r, user: u }); } catch (e) { toast.error(e.message); }
  };
  const openUserEdit = (u) => setUserEdit({ id: u.id, email: u.email, name: u.name || "", pc: u.maxDevicesPc ?? "", mobile: u.maxDevicesMobile ?? "" });
  const saveUserEdit = async () => {
    const body = { name: userEdit.name || null };
    if (userEdit.pc === "" && userEdit.mobile === "") body.clearDeviceLimits = true;
    else body.deviceLimits = { pc: userEdit.pc === "" ? null : Number(userEdit.pc), mobile: userEdit.mobile === "" ? null : Number(userEdit.mobile) };
    await api(`/api/admin/portal-users/${userEdit.id}`, { method: "PUT", body }); setUserEdit(null); toast.success("Kullanıcı güncellendi."); await load();
  };
  const showLogs = (u) => { setLogUser(u.email); setTab("logins"); };
  const openSearch = async (x) => { try { setSearchDetail(await api(`/api/admin/searches/${x.id}`)); } catch (e) { toast.error(e.message); } };
  const devLimit = (u) => { const pc = u.maxDevicesPc ?? lim.limits.maxDevicesPc, mb = u.maxDevicesMobile ?? lim.limits.maxDevicesMobile; return `${inf(pc)} pc · ${inf(mb)} mobil${u.maxDevicesPc != null || u.maxDevicesMobile != null ? " (özel)" : ""}`; };
  const openDevices = async (u) => { try { setDevUser({ user: u, devices: await api(`/api/admin/portal-users/${u.id}/devices`) }); } catch (e) { toast.error(e.message); } };
  const removeDevice = async (d) => {
    if (!(await confirm({ title: "Cihaz kaldırılsın mı?", message: `${d.name} (${d.kind === "mobile" ? "mobil" : "pc"}) kaydı silinecek; kullanıcı bu cihazdan yeniden kayıt yapmak zorunda kalır.`, confirmText: "Kaldır", danger: true }))) return;
    try { await api(`/api/admin/devices/${d.id}`, { method: "DELETE" }); toast.success("Cihaz kaldırıldı."); await openDevices(devUser.user); await load(); } catch (e) { toast.error(e.message); }
  };
  const devCount = (u) => { const ds = data.devices?.[u.id] || []; const pc = ds.filter((d) => d.kind !== "mobile").length, mb = ds.length - pc; return ds.length ? `${pc} pc · ${mb} mobil` : "—"; };
  const saveLimits = async () => {
    const body = {}; for (const k of Object.keys(limEdit)) body[k] = typeof limEdit[k] === "boolean" ? limEdit[k] : Number(limEdit[k]) || 0;
    try { const r = await api(`/api/admin/clients/${clientId}/limits`, { method: "PUT", body }); toast.success(r.customized ? "Limitler kaydedildi (özelleştirilmiş)." : "Limitler kaydedildi (paket ile aynı)."); await load(); } catch (e) { toast.error(e.message); }
  };
  const resetLimits = async () => {
    if (!(await confirm({ title: "Pakete sıfırlansın mı?", message: "Özelleştirilmiş değerler silinecek, paketin güncel değerleri kopyalanacak.", confirmText: "Sıfırla", danger: true }))) return;
    try { await api(`/api/admin/clients/${clientId}/limits/reset`, { method: "POST" }); toast.success("Limitler pakete sıfırlandı."); await load(); } catch (e) { toast.error(e.message); }
  };
  const linkSystemHotel = async () => {
    if (!linkHotel.internalHotelId) throw new Error("Bir otel seçin.");
    await api(`/api/admin/clients/${clientId}/hotels`, { method: "POST", body: { internalHotelId: linkHotel.internalHotelId, label: linkHotel.label || null } });
    setHotelModal(false); setLinkHotel({ internalHotelId: "", label: "" }); toast.success("Otel müşteriye bağlandı."); await load();
  };
  const unlinkHotel = async (h) => {
    if (!(await confirm({ title: "Bağlantı kaldırılsın mı?", message: `${hotelName(h.internalHotelId)} bu müşterinin listesinden çıkarılacak (sistemdeki otel silinmez).`, confirmText: "Kaldır", danger: true }))) return;
    try { await api(`/api/admin/clients/${clientId}/hotels/${h.internalHotelId}`, { method: "DELETE" }); toast.success("Bağlantı kaldırıldı."); await load(); } catch (e) { toast.error(e.message); }
  };
  const usage = { ...lim.usage, sites: selectedSites };

  const tabs = [
    { value: "limits", label: "Limitler", icon: "layers" },
    { value: "sites", label: "Siteler", icon: "globe", count: selectedSites },
    { value: "keys", label: "API anahtarları", icon: "key", count: sel.keys.filter((k) => !k.revokedAt).length },
    { value: "users", label: "Portal kullanıcıları", icon: "user", count: data.portalUsers.length },
    { value: "hotels", label: "Oteller", icon: "building", count: data.hotels.length },
    { value: "schedules", label: "Planlı aramalar", icon: "calendar", count: data.schedules.length },
    { value: "searches", label: "Son aramalar", icon: "history" },
    { value: "logins", label: "Giriş logları", icon: "logIn" },
  ];
  const linkable = allHotels.filter((h) => !data.hotels.some((x) => x.internalHotelId === h.internalHotelId));

  return (
    <div>
      <PageHeader title={c.name} description={c.email || "e-posta yok"} actions={<>
        <Button variant="secondary" icon="chevronLeft" onClick={() => navigate("clients")}>Müşteriler</Button>
        <Button variant={c.isActive ? "danger" : "primary"} icon="power" onClick={async () => { if (c.isActive && !(await confirm({ title: "Müşteri pasife alınsın mı?", message: `${c.name}: portal girişi ve API anahtarları çalışmaz; veriler korunur, istendiğinde yeniden aktifleştirilir.`, confirmText: "Pasife al", danger: true }))) return; update({ isActive: !c.isActive }, c.isActive ? "Pasife alındı." : "Aktifleştirildi."); }}>{c.isActive ? "Pasife al" : "Aktifleştir"}</Button></>}>
        <div className="sr-row center" style={{ gap: 8, marginTop: 8 }}>
          {c.isActive ? <Badge tone="ok">aktif</Badge> : <Badge tone="bad">pasif</Badge>}
          <Badge tone="info" dot={false}>#{c.id}</Badge>
          <Badge tone={c.kind === "otel" ? "info" : "brand"} dot={false}>{KIND[c.kind] || "ACENTE"}</Badge>
          <Badge tone="neutral" dot={false}>{TYPE[st.clientType]}</Badge>
          {c.demoExpiresAt && <Badge tone={demoExpired ? "bad" : "warn"}>{demoExpired ? "Demo süresi doldu" : `Demo · ${fmtDate(c.demoExpiresAt)}'a kadar`}</Badge>}
          {c.limitsCustomized && <Badge tone="warn">özelleştirilmiş limitler</Badge>}
        </div>
      </PageHeader>

      <div className="sr-grid">
        <Card title="Paket ve tip" flat>
          <FormGrid>
            <Field label="Paket"><Select value={c.planId ?? ""} onChange={(e) => update({ planId: e.target.value ? Number(e.target.value) : null }, "Paket güncellendi.")}><option value="">(paketsiz = sınırsız)</option>{plans.map((p) => <option key={p.id} value={p.id}>{p.name} ({p.code}){p.kind && p.kind !== "both" ? ` · ${KIND[p.kind]}` : ""}{p.isDemo ? " · demo" : ""}</option>)}</Select></Field>
            <Field label="Müşteri türü" hint={c.kind === "otel" ? "Otel: kendi otelini rakiplerle karşılaştırır" : "Acente: takip ettiği otelleri siteler arasında karşılaştırır"}><Select value={c.kind || "acente"} onChange={(e) => update({ kind: e.target.value }, "Müşteri türü güncellendi.")}><option value="acente">ACENTE</option><option value="otel">OTEL</option></Select></Field>
            <Field label="Erişim tipi"><Select value={st.clientType} onChange={(e) => saveSettings({ clientType: e.target.value })}><option value="api">API entegrasyonu</option><option value="portal">Web portal</option><option value="both">API + portal</option></Select></Field>
            {c.kind === "otel" && <Field label="Kendi oteli" hint="Portalda bu otel sabitlenir ve rakiplerle farkı gösterilir"><Select value={c.ownHotelId || ""} onChange={(e) => update({ ownHotelId: e.target.value }, e.target.value ? "Kendi oteli ayarlandı." : "Kendi oteli temizlendi.")}><option value="">(seçilmedi)</option>{data.hotels.map((h) => <option key={h.internalHotelId} value={h.internalHotelId}>{hotelName(h.internalHotelId)}</option>)}</Select></Field>}
          </FormGrid>
          <div style={{ marginTop: 12 }}><Switch label="Cihaz doğrulama (Face ID / parmak izi)" checked={!!c.requireDeviceAuth} onChange={(e) => update({ requireDeviceAuth: e.target.checked }, e.target.checked ? "Cihaz doğrulama açıldı." : "Cihaz doğrulama kapatıldı.")} /><div className="muted small" style={{ marginTop: 4 }}>Kapalıysa yalnızca e-posta/şifre ile giriş yapılır (test ve sunum hesapları için).</div></div>
          <p className="muted small" style={{ marginTop: 10 }}>Portal: <a href="/portal/" target="_blank" rel="noreferrer">/portal/</a> · API: <code>X-API-Key</code> ile <code>/api/v1/*</code></p>
        </Card>
        <Stat icon="search" label="Bugünkü arama" value={lim.usage.today.searches} unit={lim.limits.dailySearches ? `/ ${lim.limits.dailySearches}` : ""} sub={`${lim.usage.today.siteCalls} site çağrısı · ${lim.usage.today.scheduledRuns} planlı`} />
        <Stat icon="key" label="Aktif anahtar" value={sel.keys.filter((k) => !k.revokedAt).length} soft sub={lim.limits.allowApi ? "API açık" : "API kapalı"} />
        <Stat icon="building" label="Otel" value={data.hotels.length} unit={lim.limits.maxHotels ? `/ ${lim.limits.maxHotels}` : ""} soft />
      </div>

      <Card style={{ marginTop: 16 }} pad0>
        <div style={{ padding: "6px 20px 0" }}><Tabs value={tab} onChange={setTab} items={tabs} /></div>
        <div style={{ padding: "0 20px 20px" }}>
          {tab === "limits" && (<>
            {lim.customized && <Alert tone="warn">Bu müşterinin limitleri paketten farklı; paket değişse de bu değerler korunur. Portalda müşteriye "paketinizde özel değişiklikler var, yetkilinizle iletişime geçin" uyarısı gösterilir.</Alert>}
            <div className="sr-table-wrap" style={{ marginBottom: 12 }}><div className="sr-table-scroll"><table className="sr-table dense">
              <thead><tr><th>Limit</th><th className="num">Paket değeri</th><th>Geçerli değer</th><th>Bugünkü kullanım</th></tr></thead>
              <tbody>{LIMIT_ROWS.map(([k, label, use]) => <tr key={k} className={limEdit[k] !== lim.planLimits[k] ? "selected" : ""}>
                <td>{label}</td><td className="num muted">{inf(lim.planLimits[k])}</td>
                <td><Input type="number" min="0" value={limEdit[k]} onChange={(e) => setLimEdit({ ...limEdit, [k]: e.target.value })} style={{ width: 120 }} /></td>
                <td className="muted small">{use(usage)}</td></tr>)}
                {[["allowApi", "API kullanıcısı"], ["allowPackages", "Paket (otel + uçak) sorgusu"], ["allowFresh", "Önbelleği atlama (fresh)"]].map(([k, label]) => <tr key={k} className={limEdit[k] !== lim.planLimits[k] ? "selected" : ""}>
                  <td>{label}</td><td className="num muted">{lim.planLimits[k] ? "açık" : "kapalı"}</td>
                  <td><Switch checked={!!limEdit[k]} onChange={(e) => setLimEdit({ ...limEdit, [k]: e.target.checked })} label={limEdit[k] ? "açık" : "kapalı"} /></td><td /></tr>)}
              </tbody></table></div></div>
            <div className="sr-row between center">
              <KV items={[["Paket atanma", fmtDate(lim.planAssignedAt)], ["Demo bitişi", lim.demoExpiresAt ? fmtDate(lim.demoExpiresAt) : null]]} />
              <div className="sr-row" style={{ gap: 8 }}><Button variant="secondary" icon="refresh" onClick={resetLimits} disabled={!c.planId}>Pakete sıfırla</Button><Button icon="check" onClick={saveLimits}>Kaydet</Button></div>
            </div>
            <p className="muted small" style={{ marginTop: 10 }}>0 = sınırsız. Vurgulu satırlar paketten farklı değerlerdir. Kaydettiğinizde değerler paketle aynıysa özelleştirme işareti kalkar.</p>
          </>)}
          {tab === "sites" && (<>
            <div className="sr-row between center" style={{ marginBottom: 12 }}>
              <p className="muted">Kapalı siteler portalda görünmez, API'de 403 döner. Paket kısıtları ayrıca uygulanır.</p>
              <div className="sr-row center" style={{ gap: 8 }}>{allOn && <Badge tone="ok">tüm siteler açık</Badge>}<Badge tone={lim.limits.maxSites && selectedSites >= lim.limits.maxSites ? "warn" : "neutral"} dot={false}>{selectedSites} / {inf(lim.limits.maxSites)} site</Badge></div>
            </div>
            <SitePicker value={allOn ? data.providers.filter((n) => sitesOpen.has(n)) : st.enabledProviders} onChange={(list) => saveSettings({ enabledProviders: list })} max={lim.limits.maxSites || 0}
              options={data.providers.map((n) => { const site = sites.find((x) => x.name === n); const closed = !sitesOpen.has(n); return { value: n, label: site?.label || n, hint: site?.domain, disabled: closed, title: closed ? "Site yönetiminden kapatılmış (bakımda)" : "", badge: closed ? <Badge tone="warn" dot={false}>bakımda</Badge> : null }; })} />
            {lim.limits.maxSites > 0 && selectedSites >= lim.limits.maxSites && <p className="muted small" style={{ marginTop: 10 }}>Site limiti dolu ({lim.limits.maxSites}); yeni bir site açmak için önce birini kapatın veya limiti artırın.</p>}
          </>)}
          {tab === "keys" && (<>
            {!lim.limits.allowApi && <Alert tone="warn">Bu müşterinin paketi API erişimi içermiyor; anahtar üretilemez. Limitler sekmesinden "API kullanıcısı" açılabilir.</Alert>}
            {newKey && <div style={{ marginBottom: 12 }}><KeyBox label="Yeni anahtar — yalnızca bir kez gösterilir, şimdi kopyalayın:" value={newKey} /><div className="sr-row" style={{ marginTop: 8 }}><Button size="sm" variant="secondary" icon="copy" onClick={() => { navigator.clipboard?.writeText(newKey); toast.success("Kopyalandı."); }}>Kopyala</Button><Button size="sm" variant="ghost" onClick={() => setNewKey(null)}>Gizle</Button></div></div>}
            <DataTable rows={sel.keys} dense toolbarRight={<Button size="sm" icon="plus" onClick={createKey} disabled={!lim.limits.allowApi}>Yeni anahtar</Button>}
              columns={[{ key: "keyPrefix", title: "Anahtar", primary: true, render: (k) => <code>{k.keyPrefix}…</code>, filterValue: (k) => k.keyPrefix }, { key: "label", title: "Etiket", render: (k) => k.label || <span className="muted">—</span> }, { key: "lastUsedAt", title: "Son kullanım", render: (k) => fmtDate(k.lastUsedAt), hideOnMobile: true }, { key: "createdAt", title: "Oluşturma", render: (k) => fmtDate(k.createdAt), hideOnMobile: true }, { key: "revokedAt", title: "Durum", render: (k) => (k.revokedAt ? <Badge tone="bad">iptal</Badge> : <Badge tone="ok">aktif</Badge>) }]}
              actions={(k) => [{ icon: "ban", title: "İptal et", tone: "danger", hidden: !!k.revokedAt, onClick: revoke }]} empty={{ icon: "key", title: "Anahtar yok" }} />
          </>)}
          {tab === "users" && (
            <DataTable rows={data.portalUsers} dense toolbarRight={<Button size="sm" icon="plus" onClick={() => setUserModal(true)} disabled={lim.limits.maxUsers > 0 && data.portalUsers.length >= lim.limits.maxUsers} title={lim.limits.maxUsers > 0 && data.portalUsers.length >= lim.limits.maxUsers ? "Kullanıcı limiti dolu" : ""}>Kullanıcı ekle</Button>}
              toolbar={<span className="muted small">{data.portalUsers.length} / {inf(lim.limits.maxUsers)} kullanıcı</span>}
              columns={[{ key: "email", title: "E-posta", primary: true, sub: (u) => u.name || "" }, { key: "devices", title: "Cihazlar", render: devCount, sub: devLimit }, { key: "lastLoginAt", title: "Son giriş", render: (u) => fmtDate(u.lastLoginAt), hideOnMobile: true, filterValue: (u) => fmtDate(u.lastLoginAt) }, { key: "isActive", title: "Durum", render: (u) => (u.isActive ? <Badge tone="ok">aktif</Badge> : <Badge tone="bad">pasif</Badge>), filterValue: (u) => (u.isActive ? "aktif" : "pasif") }]}
              actions={(u) => [{ icon: "logIn", title: "Giriş logları", onClick: showLogs }, { icon: "smartphone", title: "Cihazlar", onClick: openDevices }, { icon: "pencil", title: "Düzenle (ad, cihaz limiti)", onClick: openUserEdit }, { icon: "key", title: "Şifre belirle", onClick: resetPw }, { icon: "mail", title: "Şifre sıfırlama linki", onClick: resetLink }, { icon: "power", title: u.isActive ? "Pasife al" : "Aktifleştir", tone: u.isActive ? "danger" : "primary", onClick: (x) => userAction(x, { isActive: !x.isActive }, x.isActive ? "Pasife alındı." : "Aktifleştirildi.") }]}
              onRowDoubleClick={openDevices} empty={{ icon: "user", title: "Portal kullanıcısı yok", text: "Müşteri portala girecekse en az bir kullanıcı ekleyin." }} />
          )}
          {tab === "hotels" && (
            <DataTable rows={data.hotels} dense toolbar={<span className="muted small">{data.hotels.length} / {inf(lim.limits.maxHotels)} otel</span>}
              toolbarRight={<Button size="sm" icon="link" onClick={() => setHotelModal(true)} disabled={lim.limits.maxHotels > 0 && data.hotels.length >= lim.limits.maxHotels}>Sistem otelinden ekle</Button>}
              columns={[{ key: "internalHotelId", title: "Otel", primary: true, render: (h) => <>{hotelName(h.internalHotelId)}{c.ownHotelId === h.internalHotelId && <Badge tone="info" dot={false} className="" style={{ marginLeft: 8 }}>kendi oteli</Badge>}</>, sub: (h) => <code>{h.internalHotelId}</code> }, { key: "label", title: "Etiket", render: (h) => h.label || <span className="muted">—</span>, hideOnMobile: true }, { key: "createdAt", title: "Bağlanma", render: (h) => fmtDate(h.createdAt), hideOnMobile: true }]}
              actions={(h) => [{ icon: "eye", title: "Detay", onClick: (x) => navigate("hoteldetail", x.internalHotelId) }, { icon: "link", title: "Listeden çıkar (bağlantıyı kaldır)", tone: "danger", onClick: unlinkHotel }]} onRowDoubleClick={(h) => navigate("hoteldetail", h.internalHotelId)} empty={{ icon: "building", title: "Müşteriye bağlı otel yok", text: "Müşterinin otel listesini buradan yönetirsiniz (portaldan otel eklenemez); 'Sistem otelinden ekle' ile bağlayın." }} />
          )}
          {tab === "schedules" && (
            <DataTable rows={data.schedules} dense onRowDoubleClick={(s) => navigate("schedules")} columns={[{ key: "name", title: "Planlı arama", primary: true, sub: (s) => (s.startsOn || s.endsOn ? `${s.startsOn || "…"} → ${s.endsOn || "…"}` : "") }, { key: "time", title: "Zaman", render: (s) => `${s.weekdays.length ? s.weekdays.map((d) => WEEKDAYS_SHORT[d]).join(", ") : "Her gün"} ${s.runTime}` }, { key: "nextRunAt", title: "Sonraki", render: (s) => fmtDate(s.nextRunAt), filterValue: (s) => fmtDate(s.nextRunAt) }, { key: "lastRunAt", title: "Son", render: (s) => fmtDate(s.lastRunAt), hideOnMobile: true, filterValue: (s) => fmtDate(s.lastRunAt) }, { key: "isActive", title: "Durum", render: (s) => (s.isActive ? <Badge tone="ok">aktif</Badge> : <Badge tone="neutral">pasif</Badge>), filterValue: (s) => (s.isActive ? "aktif" : "pasif") }]}
              actions={(s) => [{ icon: "history", title: "Geçmiş çalışmalar (Planlı Aramalar sayfası)", onClick: () => navigate("schedules") }]} empty={{ icon: "calendar", title: "Planlı arama yok" }} />
          )}
          {tab === "searches" && (
            <DataTable rows={searches} dense onRowDoubleClick={openSearch} columns={[{ key: "createdAt", title: "Zaman", primary: true, render: (s) => fmtDate(s.createdAt), filterValue: (s) => fmtDate(s.createdAt) }, { key: "kind", title: "Tür", render: (s) => (s.kind === "packages" ? "Paket" : "Otel") + " · " + (parseReq(s.request).hotelIds || []).length + " otel" }, { key: "status", title: "Durum", render: (s) => <Badge tone={statusTone(s.status)}>{statusLabel(s.status)}</Badge>, filterValue: (s) => statusLabel(s.status) }, { key: "scheduleId", title: "Kaynak", render: (s) => (s.scheduleId ? <Badge tone="info" dot={false}>planlı</Badge> : <Badge tone="neutral" dot={false}>anlık</Badge>), filterValue: (s) => (s.scheduleId ? "planlı" : "anlık") }, { key: "finishedAt", title: "Bitiş", render: (s) => fmtDate(s.finishedAt), hideOnMobile: true, filterValue: (s) => fmtDate(s.finishedAt) }]}
              actions={(s) => [{ icon: "eye", title: "Arama detayı", onClick: openSearch }]} empty={{ icon: "history", title: "Arama kaydı yok" }} />
          )}
          {tab === "logins" && (
            <DataTable rows={(logs || []).filter((l) => !logUser || l.email === logUser)} loading={logs === null} dense pageSize={50} columns={loginLogColumns()}
              toolbar={<Select value={logUser} onChange={(e) => setLogUser(e.target.value)} style={{ width: 240 }}><option value="">Tüm kullanıcılar</option>{data.portalUsers.map((u) => <option key={u.id} value={u.email}>{u.email}</option>)}</Select>}
              toolbarRight={<Button size="sm" variant="ghost" icon="refresh" onClick={() => api(`/api/admin/login-logs?clientId=${clientId}&limit=200`).then(setLogs).catch((e) => toast.error(e.message))}>Yenile</Button>} empty={{ icon: "logIn", title: "Giriş kaydı yok" }} />
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

      <FormModal open={hotelModal} onClose={() => setHotelModal(false)} title="Sistem otelinden ekle" subtitle="Sistemde kayıtlı bir oteli bu müşterinin listesine bağlar; yeni eşleştirme yapılmaz." onSubmit={linkSystemHotel} submitText="Bağla">
        <Field label="Otel" required><Select value={linkHotel.internalHotelId} onChange={(e) => setLinkHotel({ ...linkHotel, internalHotelId: e.target.value })} required><option value="">Seçin…</option>{linkable.map((h) => <option key={h.internalHotelId} value={h.internalHotelId}>{h.hotelName || h.internalHotelId} ({h.internalHotelId})</option>)}</Select></Field>
        <Field label="Etiket" hint="Opsiyonel; müşterinin kendi adlandırması"><Input value={linkHotel.label} onChange={(e) => setLinkHotel({ ...linkHotel, label: e.target.value })} /></Field>
      </FormModal>

      <Modal open={!!resetInfo} onClose={() => setResetInfo(null)} title="Şifre sıfırlama linki" subtitle={resetInfo ? `${resetInfo.email} · ${fmtDate(resetInfo.expiresAt)} tarihine kadar geçerli, tek kullanımlık` : ""} size="md"
        footer={<><Button variant="secondary" icon="copy" onClick={() => { navigator.clipboard?.writeText(resetInfo.link); toast.success("Link kopyalandı."); }}>Linki kopyala</Button><Button onClick={() => setResetInfo(null)}>Tamam</Button></>}>
        {resetInfo && (<div className="sr-stack">
          {resetInfo.emailSent ? <Alert tone="ok">E-posta gönderildi: {resetInfo.email}</Alert> : resetInfo.emailError ? <Alert tone="bad">E-posta gönderilemedi: {resetInfo.emailError}. Linki elle iletin.</Alert> : <Alert tone="warn">E-posta sunucusu (SMTP) henüz ayarlı değil; linki müşteriye elle iletin. SMTP bilgileri girildiğinde link otomatik e-postayla gider.</Alert>}
          <div className="sr-link-box">{resetInfo.link}</div>
          <p className="muted small">Kullanıcı linki açınca yeni şifresini belirler; süre dolunca veya kullanılınca link geçersiz olur.</p>
        </div>)}
      </Modal>
      <FormModal open={!!userEdit} onClose={() => setUserEdit(null)} title="Kullanıcıyı düzenle" subtitle={userEdit?.email} onSubmit={saveUserEdit}>
        {userEdit && (<>
          <Field label="Ad"><Input value={userEdit.name} onChange={(e) => setUserEdit({ ...userEdit, name: e.target.value })} /></Field>
          <FormGrid>
            <Field label="Bilgisayar cihaz limiti" hint={`Boş = paket/müşteri değeri (${inf(lim.limits.maxDevicesPc)}); 0 = sınırsız`}><Input type="number" min="0" max="50" value={userEdit.pc} onChange={(e) => setUserEdit({ ...userEdit, pc: e.target.value })} /></Field>
            <Field label="Mobil cihaz limiti" hint={`Boş = paket/müşteri değeri (${inf(lim.limits.maxDevicesMobile)}); 0 = sınırsız`}><Input type="number" min="0" max="50" value={userEdit.mobile} onChange={(e) => setUserEdit({ ...userEdit, mobile: e.target.value })} /></Field>
          </FormGrid>
        </>)}
      </FormModal>
      <SearchDetailModal data={searchDetail} onClose={() => setSearchDetail(null)} />
      <Modal open={!!devUser} onClose={() => setDevUser(null)} title={devUser ? `Cihazlar · ${devUser.user.email}` : ""} subtitle="Admin tarafında kaldırma sınırsızdır. Kaldırılan cihazla girişte kullanıcı yeniden kayıt yapmak zorunda kalır." size="md"
        footer={<Button variant="secondary" onClick={() => setDevUser(null)}>Kapat</Button>}>
        {devUser && <DataTable rows={devUser.devices} dense pageSize={0}
          columns={[{ key: "name", title: "Cihaz", primary: true, render: (d) => <span style={{ opacity: d.removedAt ? 0.55 : 1 }}>{d.name}</span>, sub: (d) => d.userAgent ? <span className="truncate" style={{ maxWidth: 260, display: "inline-block" }} title={d.userAgent}>{d.userAgent}</span> : "" }, { key: "kind", title: "Tür", render: (d) => <Badge tone="neutral" dot={false}>{d.kind === "mobile" ? "mobil" : "pc"}</Badge> }, { key: "lastUsedAt", title: "Son kullanım", render: (d) => fmtDate(d.lastUsedAt) }, { key: "createdAt", title: "Kayıt", render: (d) => fmtDate(d.createdAt), hideOnMobile: true }, { key: "removedAt", title: "Durum", render: (d) => (d.removedAt ? <Badge tone="neutral">kaldırıldı · {d.removedBy === "approval" ? "onayla" : d.removedBy === "admin" ? "admin" : "kullanıcı"}</Badge> : <Badge tone="ok">aktif</Badge>) }]}
          actions={(d) => [{ icon: "ban", title: "Kaldır (cihaz kaydı pasife alınır)", tone: "danger", hidden: !!d.removedAt, onClick: removeDevice }]}
          empty={{ icon: "smartphone", title: "Kayıtlı cihaz yok", text: "Kullanıcı ilk girişinde cihazını kaydeder." }} />}
      </Modal>
    </div>
  );
}
