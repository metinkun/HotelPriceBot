import React, { useEffect, useState } from "react";
import { api, fmtDate } from "../api.js";
import { AuthLayout, AuthVisual, Button, Field, Input, Alert } from "@kit";

// #reset/<token>: admin'in urettigi sureli linkle yeni sifre belirleme.
export default function ResetPassword({ token, onDone }) {
  const [info, setInfo] = useState(null);
  const [err, setErr] = useState("");
  const [p1, setP1] = useState("");
  const [p2, setP2] = useState("");
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);
  useEffect(() => { api("/api/portal/auth/reset/check", { method: "POST", body: { token } }).then(setInfo).catch((e) => setErr(e.data?.error || e.message)); }, [token]);
  const submit = async (e) => {
    e.preventDefault(); setErr("");
    if (p1.length < 8) return setErr("Şifre en az 8 karakter olmalı.");
    if (p1 !== p2) return setErr("Şifreler aynı değil.");
    setBusy(true);
    try { await api("/api/portal/auth/reset/complete", { method: "POST", body: { token, password: p1 } }); setDone(true); } catch (ex) { setErr(ex.data?.error || ex.message); } finally { setBusy(false); }
  };
  return (
    <AuthLayout product="Portal" headline="Yeni şifrenizi belirleyin." text="Bu bağlantı tek kullanımlıktır ve süresi sınırlıdır. Şifreniz güncellendikten sonra tüm cihazlarınızdan yeni şifreyle giriş yaparsınız." points={["En az 8 karakter", "Bağlantı tek kullanımlık", "Süre dolunca yetkilinizden yeni link isteyin"]} visual={<AuthVisual />}>
      {done ? (
        <div className="sr-stack"><div><h2>Şifreniz güncellendi</h2><div className="sub">Yeni şifrenizle giriş yapabilirsiniz.</div></div><Button size="lg" block variant="gradient" onClick={onDone}>Giriş ekranına git</Button></div>
      ) : (
        <form onSubmit={submit} className="sr-stack">
          <div><h2>Şifre yenileme</h2><div className="sub">{info ? <>{info.email}{info.expiresAt ? ` · ${fmtDate(info.expiresAt)} tarihine kadar geçerli` : ""}</> : "Bağlantı kontrol ediliyor…"}</div></div>
          {err && <Alert tone="bad">{err}</Alert>}
          {info && (<>
            <Field label="Yeni şifre"><Input type="password" icon="lock" value={p1} onChange={(e) => setP1(e.target.value)} autoFocus autoComplete="new-password" minLength={8} required /></Field>
            <Field label="Yeni şifre (tekrar)"><Input type="password" icon="lock" value={p2} onChange={(e) => setP2(e.target.value)} autoComplete="new-password" minLength={8} required /></Field>
            <Button type="submit" size="lg" block loading={busy} variant="gradient">Şifreyi kaydet</Button>
          </>)}
          {!info && err && <Button variant="secondary" block onClick={onDone}>Giriş ekranına dön</Button>}
        </form>
      )}
    </AuthLayout>
  );
}
