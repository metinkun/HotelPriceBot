import React, { useEffect, useState } from "react";
import { api, fmtDate, today } from "../api.js";
import { PageHeader, Stat, DataTable, Button, Badge, Alert, Skeleton, useToast } from "@kit";

export default function Dashboard({ navigate }) {
  const [health, setHealth] = useState(null);
  const [jobs, setJobs] = useState(null);
  const [usage, setUsage] = useState(null);
  const [pending, setPending] = useState(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const toast = useToast();

  const load = async () => {
    try {
      const [h, j, u] = await Promise.all([api("/api/admin/providers/health"), api("/api/admin/jobs/stats"), api("/api/admin/usage?from=" + today() + "&to=" + today())]);
      setHealth(h); setJobs(j); setUsage(u); setErr("");
      api("/api/admin/approvals?status=pending&limit=1").then((a) => setPending(a.pending || 0)).catch(() => {});
    } catch (e) { setErr(e.message); }
  };
  useEffect(() => { load(); const t = setInterval(load, 15000); return () => clearInterval(t); }, []);

  const runCanary = async () => {
    setBusy(true);
    try { const r = await api("/api/admin/canary/run", { method: "POST" }); await load(); toast.success(`Kanarya tamamlandı: ${r.healthy}/${r.total} site sağlıklı.`); }
    catch (e) { toast.error(e.message); } finally { setBusy(false); }
  };
  const reset = async (p) => { try { await api(`/api/admin/providers/${p.provider}/reset-circuit`, { method: "POST" }); toast.success(`${p.provider} devre kesicisi sıfırlandı.`); await load(); } catch (e) { toast.error(e.message); } };

  if (err && !health) return <div><PageHeader title="Özet" /><Alert tone="bad">{err}</Alert></div>;
  if (!health) return <div><PageHeader title="Özet" /><div className="sr-grid">{[0, 1, 2, 3, 4].map((i) => <Skeleton key={i} h={78} />)}</div></div>;
  const canary = health.canary;
  const check = (p) => canary?.checks?.find((x) => x.provider === p.provider);
  const status = (p) => {
    if (p.circuitOpen) return <Badge tone="bad">devre açık</Badge>;
    const c = check(p);
    if (!c) return <Badge tone="neutral">bilinmiyor</Badge>;
    if (c.ok && c.found) return <Badge tone="ok">sağlıklı</Badge>;
    return <Badge tone="warn">{c.error ? "kanarya: hata" : "kanarya: fiyat yok"}</Badge>;
  };

  return (
    <div>
      <PageHeader title="Özet" description="Bugünkü kullanım, iş kuyruğu ve 13 sitenin sağlık durumu. Sayfa 15 saniyede bir yenilenir." />
      {err && <Alert tone="bad">{err}</Alert>}
      <div className="sr-grid">
        <Stat icon="zap" label="Bugünkü kredi" value={usage?.totals?.credits ?? 0} sub="site çağrısı" />
        <Stat icon="activity" label="Bugünkü istek" value={usage?.totals?.requests ?? 0} soft />
        <Stat icon="layers" label="Kuyrukta / çalışan iş" value={`${jobs?.queued ?? 0} / ${jobs?.running ?? 0}`} soft />
        <Stat icon="server" label="Cache" value={health.cache?.enabled ? health.cache.ttlSeconds : "—"} unit={health.cache?.enabled ? "sn" : "kapalı"} soft />
        <Stat icon="shield" label="Kanarya" value={canary ? `${canary.healthy}/${canary.total}` : "—"} sub={canary ? "son çalışma: " + fmtDate(canary.ranAt) : "henüz çalışmadı"} soft={!canary || canary.healthy < canary.total} />
        <Stat icon="clipboardCheck" label="Onay bekleyen" value={pending ?? "—"} sub="cihaz talepleri" soft={!pending} onClick={() => navigate && navigate("approvals")} />
      </div>

      <div style={{ marginTop: 16 }}>
        <DataTable rows={health.providers} rowKey={(p) => p.provider} dense pageSize={0}
          toolbar={<h3>Site sağlığı</h3>}
          toolbarRight={<Button size="sm" variant="secondary" icon="play" loading={busy} onClick={runCanary}>Kanaryayı şimdi çalıştır</Button>}
          columns={[
            { key: "provider", title: "Site", primary: true, render: (p) => <span className="sr-row center" style={{ gap: 6 }}>{p.provider}{health.capabilities?.[p.provider]?.requiresPuppeteer && <Badge tone="neutral" dot={false}>tarayıcı</Badge>}</span> },
            { key: "status", title: "Durum", render: status },
            { key: "inFlight", title: "Eşzamanlı", align: "num", render: (p) => `${p.inFlight}/${p.concurrency}`, hideOnMobile: true },
            { key: "total", title: "Toplam", align: "num", sortable: true },
            { key: "success", title: "Başarılı", align: "num", hideOnMobile: true },
            { key: "failed", title: "Hata", align: "num", sortable: true, render: (p) => (p.failed ? <span style={{ color: "var(--bad)", fontWeight: 600 }}>{p.failed}</span> : 0) },
            { key: "notFound", title: "Bulunamadı", align: "num", hideOnMobile: true },
            { key: "avgLatencyMs", title: "Ort. ms", align: "num", sortable: true, hideOnMobile: true },
            { key: "lastError", title: "Son hata", render: (p) => <span className="muted small truncate" title={p.lastError || ""} style={{ display: "block", maxWidth: 260 }}>{p.lastError || "—"}</span>, hideOnMobile: true },
          ]}
          actions={(p) => [{ icon: "refresh", title: "Devre kesiciyi sıfırla", hidden: !p.circuitOpen, onClick: reset }]}
          empty={{ icon: "server", title: "Site bilgisi yok" }} />
      </div>
    </div>
  );
}
