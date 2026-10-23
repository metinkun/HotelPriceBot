import React, { useEffect, useState } from "react";
import { api, fmtDate, parseReq } from "../api.js";
import ResultTable from "./ResultTable.jsx";

export default function History({ me }) {
  const [list, setList] = useState([]);
  const [sel, setSel] = useState(null);
  const [err, setErr] = useState("");
  const load = () => api("/api/portal/searches?limit=100").then(setList).catch((e) => setErr(e.message));
  useEffect(() => { load(); const pre = location.hash.split("/")[1]; if (pre) open(pre); }, []);
  const open = async (id) => { try { setSel(await api("/api/portal/searches/" + id)); } catch (e) { setErr(e.message); } };
  useEffect(() => {
    if (!sel || sel.status === "done" || sel.status === "failed") return;
    const t = setTimeout(() => open(sel.id), 2000); return () => clearTimeout(t);
  }, [sel]);
  const req = (s) => parseReq(s.request);
  return (
    <div>
      {err && <div className="err">{err}</div>}
      <div className="row" style={{ alignItems: "flex-start" }}>
        <div className="card" style={{ flex: "0 0 420px" }}>
          <h3>Aramalar</h3>
          <table><tbody>{list.map((s) => { const r = req(s); return (
            <tr key={s.id} onClick={() => open(s.id)} style={{ cursor: "pointer", background: sel?.id === s.id ? "#ecfeff" : "" }}>
              <td className="muted" style={{ whiteSpace: "nowrap" }}>{fmtDate(s.createdAt)}</td>
              <td>{s.kind === "packages" ? "Paket" : "Otel"} · {(r.hotelIds || []).length} otel<div className="muted" style={{ fontSize: 12 }}>{r.checkIn ? r.checkIn + " → " + r.checkOut : `+${r.daysAhead} gün, ${r.nights} gece`}</div></td>
              <td><span className={"badge " + (s.status === "done" ? "ok" : s.status === "failed" ? "bad" : "warn")}>{s.status}</span>{s.scheduleId ? <div><span className="badge brand">planlı</span></div> : null}</td>
            </tr>); })}</tbody></table>
          {list.length === 0 && <div className="muted">Kayıt yok.</div>}
        </div>
        <div className="card" style={{ flex: 1, minWidth: 420 }}>
          {!sel && <div className="muted">Detay için soldan bir arama seçin.</div>}
          {sel && (<>
            <div className="row" style={{ justifyContent: "space-between" }}><h3>Arama #{sel.id} <span className="muted" style={{ fontWeight: 400 }}>{sel.result?.checkIn} → {sel.result?.checkOut}</span></h3><span className={"badge " + (sel.status === "done" ? "ok" : sel.status === "failed" ? "bad" : "warn")}>{sel.status}</span></div>
            {sel.error && <div className="err">{sel.error}</div>}
            {sel.result && <ResultTable result={sel.result} providers={me.providers} kind={sel.kind} />}
          </>)}
        </div>
      </div>
    </div>
  );
}
