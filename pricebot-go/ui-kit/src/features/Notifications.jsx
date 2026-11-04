import React, { useEffect, useState } from "react";
import { Card, Badge, Button, DataTable, Alert, EmptyState } from "../index.js";
import { fmtDate, fmtRel } from "../format.js";

/*
NotificationsView: admin ve portal icin ortak bildirim listesi.
  api(path, opts), apiBase: "/api/admin" | "/api/portal"
  labelOf(providerName) -> gosterim adi; onOpenHotel(hotelId, provider)
*/
export function NotificationsView({ api, apiBase, labelOf = (n) => n, onOpenHotel }) {
  const [data, setData] = useState({ unread: 0, items: [] });
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");
  const [onlyUnread, setOnlyUnread] = useState(false);
  const load = () => api(`${apiBase}/notifications?limit=200`).then(setData).catch((e) => setErr(e.message)).finally(() => setLoading(false));
  useEffect(() => { load(); }, []);
  const readAll = async () => { try { await api(`${apiBase}/notifications/read`, { method: "POST", body: {} }); await load(); } catch (e) { setErr(e.message); } };
  const readOne = async (n) => { if (n.readAt) return; try { await api(`${apiBase}/notifications/read`, { method: "POST", body: { ids: [n.id] } }); await load(); } catch (e) { setErr(e.message); } };
  const rows = onlyUnread ? data.items.filter((n) => !n.readAt) : data.items;
  const open = (n) => { readOne(n); if (n.internalHotelId && onOpenHotel) onOpenHotel(n.internalHotelId, n.provider || ""); };
  return (
    <div>
      {err && <Alert tone="bad">{err}</Alert>}
      <DataTable rows={rows} loading={loading} pageSize={30}
        toolbar={<div className="sr-row center" style={{ gap: 8 }}><Badge tone={data.unread ? "warn" : "neutral"}>{data.unread} okunmamış</Badge><Button size="sm" variant={onlyUnread ? "soft" : "ghost"} onClick={() => setOnlyUnread(!onlyUnread)}>{onlyUnread ? "Tümünü göster" : "Sadece okunmamış"}</Button></div>}
        toolbarRight={data.unread > 0 && <Button size="sm" variant="secondary" icon="check" onClick={readAll}>Tümünü okundu yap</Button>}
        columns={[
          { key: "createdAt", title: "Zaman", width: 150, render: (n) => <span title={fmtDate(n.createdAt)} style={{ fontWeight: n.readAt ? 400 : 600 }}>{fmtRel(n.createdAt)}</span>, sortable: true, sortValue: (n) => n.createdAt },
          { key: "provider", title: "Acente", width: 130, render: (n) => (n.provider ? <Badge tone="info" dot={false}>{labelOf(n.provider)}</Badge> : "—") },
          { key: "title", title: "Bildirim", primary: true, render: (n) => <span style={{ fontWeight: n.readAt ? 500 : 700 }}>{n.title}</span>, sub: (n) => n.body },
          { key: "readAt", title: "Durum", width: 110, render: (n) => (n.readAt ? <Badge tone="neutral" dot={false}>okundu</Badge> : <Badge tone="warn">yeni</Badge>), hideOnMobile: true },
        ]}
        actions={(n) => [{ icon: "eye", title: "Detayı aç", hidden: !n.internalHotelId, onClick: open }, { icon: "check", title: "Okundu işaretle", hidden: !!n.readAt, onClick: readOne }]}
        onRowDoubleClick={open} empty={{ icon: "bell", title: "Bildirim yok", text: "Acentelerde otel bilgisi değiştiğinde burada görünür." }} />
    </div>
  );
}
