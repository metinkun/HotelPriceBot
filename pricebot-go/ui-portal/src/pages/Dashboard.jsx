import React, { useEffect, useState } from "react";
import { api, fmtDate } from "../api.js";
import { Icon } from "../Art.jsx";

export default function Dashboard({ me }) {
  const [hotels, setHotels] = useState([]);
  const [searches, setSearches] = useState([]);
  const [schedules, setSchedules] = useState([]);
  useEffect(() => {
    api("/api/portal/hotels").then(setHotels).catch(() => {});
    api("/api/portal/searches?limit=8").then(setSearches).catch(() => {});
    api("/api/portal/schedules").then(setSchedules).catch(() => {});
  }, []);
  const enabled = me.providers.filter((p) => p.enabled);
  const next = schedules.filter((s) => s.isActive && s.nextRunAt).sort((a, b) => new Date(a.nextRunAt) - new Date(b.nextRunAt))[0];
  return (
    <div>
      <div className="grid">
        <div className="stat"><div className="ic"><Icon name="hotel" /></div><div><div className="k">Takip edilen otel</div><div className="v">{hotels.length}</div></div></div>
        <div className="stat"><div className="ic"><Icon name="sites" /></div><div><div className="k">Etkin site</div><div className="v">{enabled.length}</div></div></div>
        <div className="stat"><div className="ic"><Icon name="clock" /></div><div><div className="k">Aktif plan</div><div className="v">{schedules.filter((s) => s.isActive).length}</div></div></div>
        <div className="stat"><div className="ic"><Icon name="history" /></div><div><div className="k">Sonraki planlı arama</div><div className="v" style={{ fontSize: 14 }}>{next ? fmtDate(next.nextRunAt) : "—"}</div></div></div>
      </div>
      <div className="row" style={{ alignItems: "flex-start", marginTop: 18 }}>
        <div className="card" style={{ flex: 1, minWidth: 360 }}>
          <h3>Son aramalar</h3>
          {searches.length === 0 && <div className="muted">Henüz arama yok. <a href="#search">İlk aramanızı yapın →</a></div>}
          <table><tbody>{searches.map((s) => <tr key={s.id} onClick={() => { location.hash = "history/" + s.id; }} style={{ cursor: "pointer" }}>
            <td className="muted">{fmtDate(s.createdAt)}</td><td>{s.kind === "packages" ? "Paket" : "Otel"} · {(JSON.parse(s.request).hotelIds || []).length} otel</td>
            <td><span className={"badge " + (s.status === "done" ? "ok" : s.status === "failed" ? "bad" : "warn")}>{s.status}</span></td><td>{s.scheduleId ? <span className="badge brand">planlı</span> : ""}</td></tr>)}</tbody></table>
        </div>
        <div className="card" style={{ flex: 1, minWidth: 320 }}>
          <h3>Etkin siteler</h3>
          <div className="chips">{enabled.map((p) => <span key={p.name} className="chip on">{p.label}</span>)}{me.providers.filter((p) => !p.enabled).map((p) => <span key={p.name} className="chip off">{p.label}</span>)}</div>
          <p className="muted" style={{ marginTop: 12 }}>Site eklemek veya kapatmak için hesap yöneticinizle iletişime geçin.</p>
        </div>
      </div>
    </div>
  );
}
