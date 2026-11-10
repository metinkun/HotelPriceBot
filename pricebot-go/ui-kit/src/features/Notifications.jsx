import React, { useEffect, useState } from "react";
import { Badge, Button, DataTable, Alert, Segmented } from "../index.js";
import { fmtDate, fmtRel } from "../format.js";

/*
NotificationsView: admin ve portal icin ortak bildirim listesi.
  api(path, opts), apiBase: "/api/admin" | "/api/portal"
  labelOf(providerName) -> gosterim adi; onOpenHotel(hotelId, provider)
  Turler: detail_change (otel bilgisi), review (yorum), score (puan), event (etkinlik), campaign (kampanya)
*/
export const NOTIF_KIND = { detail_change: ["info", "Otel bilgisi", "diff"], review: ["ok", "Yorum", "message"], score: ["brand", "Puan", "award"], event: ["warn", "Etkinlik", "music"], campaign: ["bad", "Kampanya", "megaphone"] };
export const kindBadge = (k) => { const [t, l] = NOTIF_KIND[k] || ["neutral", k]; return <Badge tone={t} dot={false}>{l}</Badge>; };

export function NotificationsView({ api, apiBase, labelOf = (n) => n, onOpenHotel }) {
  const [data, setData] = useState({ unread: 0, items: [] });
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");
  const [onlyUnread, setOnlyUnread] = useState(false);
  const [kind, setKind] = useState("all");
  const load = () => api(`${apiBase}/notifications?limit=200`).then(setData).catch((e) => setErr(e.message)).finally(() => setLoading(false));
  useEffect(() => { load(); }, []);
  const readAll = async () => { try { await api(`${apiBase}/notifications/read`, { method: "POST", body: {} }); await load(); } catch (e) { setErr(e.message); } };
  const readOne = async (n) => { if (n.readAt) return; try { await api(`${apiBase}/notifications/read`, { method: "POST", body: { ids: [n.id] } }); await load(); } catch (e) { setErr(e.message); } };
  const rows = data.items.filter((n) => (!onlyUnread || !n.readAt) && (kind === "all" || n.kind === kind));
  const open = (n) => { readOne(n); if (n.internalHotelId && onOpenHotel) onOpenHotel(n.internalHotelId, n.provider || ""); };
  const count = (k) => data.items.filter((n) => n.kind === k).length;
  return (
    <div>
      {err && <Alert tone="bad">{err}</Alert>}
      <DataTable rows={rows} loading={loading} pageSize={30}
        toolbar={<div className="sr-row center" style={{ gap: 8 }}>
          <Segmented value={kind} onChange={setKind} options={[{ value: "all", label: "Tümü" }, ...Object.entries(NOTIF_KIND).map(([k, [, l]]) => ({ value: k, label: `${l}${count(k) ? ` (${count(k)})` : ""}` }))]} />
          <Badge tone={data.unread ? "warn" : "neutral"}>{data.unread} okunmamış</Badge>
          <Button size="sm" variant={onlyUnread ? "soft" : "ghost"} onClick={() => setOnlyUnread(!onlyUnread)}>{onlyUnread ? "Tümünü göster" : "Sadece okunmamış"}</Button></div>}
        toolbarRight={data.unread > 0 && <Button size="sm" variant="secondary" icon="check" onClick={readAll}>Tümünü okundu yap</Button>}
        columns={[
          { key: "createdAt", title: "Zaman", width: 150, render: (n) => <span title={fmtDate(n.createdAt)} style={{ fontWeight: n.readAt ? 400 : 600 }}>{fmtRel(n.createdAt)}</span>, sortable: true, sortValue: (n) => n.createdAt, filterValue: (n) => fmtDate(n.createdAt) },
          { key: "kind", title: "Tür", width: 120, render: (n) => kindBadge(n.kind), filterValue: (n) => (NOTIF_KIND[n.kind] || [0, n.kind])[1] },
          { key: "provider", title: "Acente", width: 130, render: (n) => (n.provider ? <Badge tone="info" dot={false}>{labelOf(n.provider)}</Badge> : "—"), filterValue: (n) => labelOf(n.provider) },
          { key: "title", title: "Bildirim", primary: true, render: (n) => <span style={{ fontWeight: n.readAt ? 500 : 700 }}>{n.title}</span>, sub: (n) => n.body },
          { key: "readAt", title: "Durum", width: 110, render: (n) => (n.readAt ? <Badge tone="neutral" dot={false}>okundu</Badge> : <Badge tone="warn">yeni</Badge>), hideOnMobile: true, filterValue: (n) => (n.readAt ? "okundu" : "yeni") },
        ]}
        actions={(n) => [{ icon: "eye", title: "Detayı aç", hidden: !n.internalHotelId, onClick: open }, { icon: "check", title: "Okundu işaretle", hidden: !!n.readAt, onClick: readOne }]}
        onRowDoubleClick={open} empty={{ icon: "bell", title: "Bildirim yok", text: "Acentelerde otel bilgisi, yorum, puan, etkinlik veya kampanya değiştiğinde burada görünür." }} />
    </div>
  );
}
