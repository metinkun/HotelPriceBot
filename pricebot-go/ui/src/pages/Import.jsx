import React, { useEffect, useState } from "react";
import { api } from "../api.js";
import { PageHeader, Card, DataTable, Button, Field, Input, Textarea, ChipSelect, Badge, Progress, Alert, useConfirm, useToast } from "@kit";

// Toplu otel ekleme: isim listesi -> acentelerde arama -> eslesme raporu -> onayla ve ekle
export default function Import({ navigate }) {
  const [providers, setProviders] = useState([]);
  const [text, setText] = useState("");
  const [minScore, setMinScore] = useState(0.6);
  const [country, setCountry] = useState("");
  const [sel, setSel] = useState([]);
  const [rep, setRep] = useState(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const confirm = useConfirm(); const toast = useToast();

  useEffect(() => { api("/api/admin/hotels/import/providers").then((d) => { setProviders(d.providers); setSel(d.providers); }).catch((e) => setErr(e.message)); }, []);
  const loadTRNC = async () => { try { const d = await api("/api/admin/hotels/import/trnc"); setText(d.map((h) => `${h.externalId}\t${h.name}`).join("\n")); setCountry("TRNC"); toast.info(`${d.length} otel yüklendi.`); } catch (e) { setErr(e.message); } };
  const parse = () => text.split("\n").map((l) => l.trim()).filter(Boolean).map((l, i) => { const m = l.match(/^(\d+)\s*[\t;,]\s*(.+)$/); const h = m ? { externalId: Number(m[1]), name: m[2].trim() } : { externalId: i + 1, name: l }; if (country.trim()) h.country = country.trim(); return h; });
  const count = text.split("\n").filter((l) => l.trim()).length;

  const run = async (apply) => {
    if (apply && !(await confirm({ title: "Oteller eklensin mi?", message: `Eşik üstü ve belirsiz olmayan eşleşmelerle ${count} otel için kayıt oluşturulacak. Önce "Sadece ara" ile raporu incelemeniz önerilir.`, confirmText: "Ara ve ekle" }))) return;
    setBusy(true); setErr(""); setRep(null);
    try {
      const started = await api("/api/admin/hotels/import", { method: "POST", body: { hotels: parse(), providers: sel, minScore: Number(minScore), apply } });
      const poll = async () => {
        try {
          const r = await api("/api/admin/imports/" + started.importId);
          setRep(r);
          if (r.status === "running") setTimeout(poll, 3000); else { setBusy(false); toast.success(apply ? `Ekleme bitti: ${r.results.filter((x) => x.internalHotelId).length}/${r.total} otel oluşturuldu.` : `Rapor hazır: ${r.results.filter((x) => x.matches.length).length}/${r.total} otel eşleşti.`); }
        } catch (e) { setErr(e.message); setBusy(false); }
      };
      poll();
    } catch (e) { setErr(e.message); setBusy(false); }
  };

  const matched = rep ? rep.results.filter((x) => x.matches.length).length : 0;
  return (
    <div>
      <PageHeader title="Toplu otel ekle" description="Her satır bir otel: ID[tab]Otel adı veya sadece ad. Sistem her acentede isimle arar ve en iyi eşleşmeyi puanlar. Önce rapor alın, sonra ekleyin." />
      {err && <Alert tone="bad">{err}</Alert>}
      <Card>
        <div className="sr-row">
          <Button variant="secondary" icon="upload" onClick={loadTRNC}>KKTC listesini yükle (78 otel)</Button>
          <Field label="Ülke"><Input value={country} onChange={(e) => setCountry(e.target.value)} placeholder="TRNC" style={{ width: 110 }} /></Field>
          <Field label="Eşik (0-1)"><Input type="number" step="0.05" min="0.3" max="1" value={minScore} onChange={(e) => setMinScore(e.target.value)} style={{ width: 100 }} /></Field>
        </div>
        <p className="muted small" style={{ marginTop: 8 }}>Ülke alanına TRNC yazılırsa Türkiye ve yurt dışı şehirlerindeki adaylar elenir. Eşik, eşleşme puanının alt sınırıdır (varsayılan 0,6).</p>
        <Field label={`Otel listesi${count ? ` (${count} satır)` : ""}`} style={{ marginTop: 14 }}>
          <Textarea value={text} onChange={(e) => setText(e.target.value)} rows={10} className="mono" style={{ fontSize: 13 }} placeholder={"3234\tACAPULCO RESORT HOTEL\n5\tMERIT CRYSTAL COVE HOTEL & CASINO"} />
        </Field>
        <div style={{ marginTop: 14 }}>
          <div className="sr-row center between" style={{ marginBottom: 6 }}>
            <span className="sr-label">Aranacak acenteler ({sel.length}/{providers.length})</span>
            <span className="sr-row" style={{ gap: 4 }}><Button size="sm" variant="ghost" onClick={() => setSel(providers)}>Tümünü seç</Button><Button size="sm" variant="ghost" onClick={() => setSel([])}>Temizle</Button></span>
          </div>
          <ChipSelect value={sel} onChange={setSel} options={providers.map((p) => ({ value: p, label: p }))} />
          {providers.length === 0 && <span className="muted small">Henüz isimle arama destekleyen acente yok.</span>}
        </div>
        <div className="sr-row" style={{ marginTop: 18 }}>
          <Button variant="secondary" icon="search" disabled={busy || !text.trim() || !sel.length} loading={busy && !rep?.applied} onClick={() => run(false)}>Sadece ara (rapor)</Button>
          <Button icon="plus" disabled={busy || !text.trim() || !sel.length} onClick={() => run(true)}>Ara ve ekle</Button>
          {busy && <span className="muted small">Her otel {sel.length} acentede aranıyor; 78 otel yaklaşık 3-4 dakika sürer.</span>}
        </div>
      </Card>

      {rep && (
        <Card style={{ marginTop: 16 }} title={`Rapor · ${rep.done}/${rep.total} otel · ${rep.providers.length} acente${rep.duration ? " · " + rep.duration : ""}`}
          subtitle={rep.status === "done" ? `${matched} otel en az bir acentede eşleşti` : "Arama sürüyor…"}
          actions={rep.status === "running" ? <Badge tone="warn">çalışıyor</Badge> : rep.applied ? <Badge tone="ok">eklendi</Badge> : <Badge tone="info" dot={false}>rapor</Badge>}>
          {rep.status === "running" && <Progress value={rep.done} max={rep.total || 1} style={{ marginBottom: 14 }} />}
          <DataTable rows={rep.results} rowKey={(r) => r.externalId} pageSize={100} dense
            columns={[
              { key: "name", title: "Otel", primary: true, sub: (r) => `#${r.externalId}` },
              { key: "matches", title: "Eşleşen", render: (r) => r.matches.length ? <div className="sr-stack" style={{ gap: 4 }}>{r.matches.map((m) => <div key={m.provider} className="sr-row center" style={{ gap: 6, flexWrap: "nowrap" }}><Badge tone={m.ambiguous ? "warn" : "ok"} dot={false}>{m.provider}</Badge><a href={m.url} target="_blank" rel="noreferrer" className="truncate" style={{ maxWidth: 260 }}>{m.name}</a><span className="muted small nowrap">%{Math.round(m.score * 100)}{m.ambiguous ? " · belirsiz" : ""}</span></div>)}</div> : <span className="muted">—</span> },
              { key: "lowConfidence", title: "Düşük güven", render: (r) => r.lowConfidence.length ? <div className="sr-stack" style={{ gap: 2 }}>{r.lowConfidence.map((m) => <span key={m.provider} className="muted small">{m.provider}: {m.name} (%{Math.round(m.score * 100)})</span>)}</div> : <span className="muted">—</span>, hideOnMobile: true },
              { key: "result", title: "Sonuç", render: (r) => <div>{r.internalHotelId && <a href={"#hoteldetail/" + r.internalHotelId}><code>{r.internalHotelId}</code></a>}{r.errors?.map((e, i) => <div key={i} className="small" style={{ color: "var(--bad)" }}>{e}</div>)}{!r.internalHotelId && !r.errors?.length && <span className="muted">—</span>}</div> },
            ]}
            actions={(r) => [{ icon: "eye", title: "Detay", hidden: !r.internalHotelId, onClick: (x) => navigate("hoteldetail", x.internalHotelId) }]}
            onRowDoubleClick={(r) => r.internalHotelId && navigate("hoteldetail", r.internalHotelId)}
            empty={{ icon: "search", title: "Henüz sonuç yok" }} />
        </Card>
      )}
    </div>
  );
}
