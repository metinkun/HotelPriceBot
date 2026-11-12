import React, { useEffect, useState } from "react";
import { Icon } from "./Icon.jsx";
import { IconButton } from "./Button.jsx";
import { Logo, LogoMark, Wordmark } from "./Logo.jsx";
import { Avatar } from "./Card.jsx";
import { useTheme } from "./theme.js";
import { cx } from "./format.js";

// Sunucudan surum bilgisi (/health): tek kaynak, dagitilan binary'nin surumunu gosterir.
let versionCache = null;
export function useVersion() {
  const [v, setV] = useState(versionCache);
  useEffect(() => {
    if (versionCache) return;
    fetch("/health").then((r) => r.json()).then((j) => { versionCache = { version: j.version || "dev", commit: j.commit || "", builtAt: j.builtAt || "" }; setV(versionCache); }).catch(() => setV({ version: "" }));
  }, []);
  return v;
}

// Silik surum etiketi; title'da commit + derleme zamani.
export function VersionTag({ className, light }) {
  const v = useVersion();
  if (!v || !v.version) return null;
  const tip = ["Sürüm " + v.version, v.commit && "commit " + v.commit, v.builtAt && "derleme " + v.builtAt.replace("T", " ").replace("Z", " UTC")].filter(Boolean).join(" · ");
  return <div className={cx("sr-version", light && "light", className)} title={tip}>v{v.version}{v.commit ? <span> · {v.commit}</span> : null}</div>;
}

/*
AppShell: sol menu (masaustu) / cekmece + alt sekme cubugu (mobil), ust cubuk.
  product: "Admin" | "Portal"
  nav: [{ key, label, icon, badge?, section? }]   section: grup basligi (ilk ogede verilir)
  current: aktif key; onNavigate(key)
  user: { name, sub }; onLogout
  title / crumbs: ust cubukta sayfa adi; topbarExtra: ek dugmeler (bildirim vb.)
  mobileNavKeys: alt cubukta gosterilecek 4-5 key (varsayilan ilk 4 + menu)
*/
export function AppShell({ product, nav, current, onNavigate, user, onLogout, title, topbarExtra, children, mobileNavKeys, footerLinks }) {
  const [open, setOpen] = useState(false);
  const [theme, toggleTheme] = useTheme();
  useEffect(() => { setOpen(false); }, [current]);
  const go = (k) => { onNavigate(k); setOpen(false); };
  const bottom = (mobileNavKeys ? nav.filter((n) => mobileNavKeys.includes(n.key)) : nav.slice(0, 4));
  return (
    <div className="sr-shell">
      <div className={cx("sr-drawer-backdrop", open && "open")} onClick={() => setOpen(false)} />
      <aside className={cx("sr-side", open && "open")}>
        <div className="sr-brand"><Logo size={38} product={product} /></div>
        <nav className="sr-nav">
          {nav.map((n) => (
            <React.Fragment key={n.key}>
              {n.section && <div className="sr-nav-label">{n.section}</div>}
              <button className={n.key === current ? "active" : ""} onClick={() => go(n.key)}><Icon name={n.icon} />{n.label}{n.badge > 0 && <span className="sr-count">{n.badge}</span>}</button>
            </React.Fragment>
          ))}
        </nav>
        <div className="sr-side-foot">
          {footerLinks}
          {user && <div className="sr-user"><Avatar name={user.name} /><div className="sr-user-text"><b>{user.name}</b><small title={user.sub}>{user.sub}</small></div><IconButton icon="logout" title="Çıkış" onClick={onLogout} style={{ color: "#B6C0D9" }} /></div>}
          <VersionTag light />
        </div>
      </aside>
      <div className="sr-main">
        <header className="sr-topbar">
          <IconButton icon="menu" title="Menü" className="sr-menu-btn" onClick={() => setOpen(true)} />
          <div className="sr-crumb"><b>{title}</b></div>
          <div className="sr-spacer" />
          {topbarExtra}
          <IconButton icon={theme === "dark" ? "sun" : "moon"} title={theme === "dark" ? "Açık tema" : "Koyu tema"} onClick={toggleTheme} />
        </header>
        <main className="sr-content">{children}</main>
      </div>
      <nav className="sr-bottom-nav">
        {bottom.map((n) => <button key={n.key} className={n.key === current ? "active" : ""} onClick={() => go(n.key)}><Icon name={n.icon} />{n.label}{n.badge > 0 && <span className="sr-count">{n.badge}</span>}</button>)}
        <button onClick={() => setOpen(true)}><Icon name="menu" />Menü</button>
      </nav>
    </div>
  );
}

// Giris ekrani duzeni: solda marka paneli, sagda form.
export function AuthLayout({ product, headline, text, points = [], visual, children }) {
  return (
    <div className="sr-auth">
      <div className="sr-auth-art">
        <Logo size={44} product={product} textSize={26} />
        <h1>{headline}</h1>
        <p>{text}</p>
        {points.length > 0 && <div className="sr-auth-points">{points.map((p) => <div key={p}><Icon name="checkCircle" size={18} />{p}</div>)}</div>}
        {visual && <div className="sr-auth-visual">{visual}</div>}
      </div>
      <div className="sr-auth-form">
        <div style={{ width: "100%", maxWidth: 380 }}>
          <div className="sr-auth-mobile-brand"><Logo size={40} product={product} onDark={false} /></div>
          {children}
        </div>
        <VersionTag className="sr-auth-version" />
      </div>
    </div>
  );
}

// Giris panelinde kullanilan kucuk "fiyat karsilastirma" gorseli (marka renkleriyle, dis kaynak yok)
export function AuthVisual({ rows = [["Etstur", 84800], ["MNG Turizm", 68900, true], ["Jolly", 84800], ["Prontotour", 73882]] }) {
  const max = Math.max(...rows.map((r) => r[1]));
  return (
    <div style={{ background: "rgba(255,255,255,.06)", border: "1px solid rgba(255,255,255,.1)", borderRadius: 16, padding: 18, maxWidth: 440, backdropFilter: "blur(6px)" }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12 }}><span style={{ fontSize: 12, fontWeight: 700, letterSpacing: ".08em", color: "#8EA0C8" }}>MERIT CRYSTAL COVE · 3 GECE</span><span style={{ fontSize: 11, color: "#4ADE80", fontWeight: 700 }}>● canlı</span></div>
      {rows.map(([n, v, best]) => (
        <div key={n} style={{ display: "grid", gridTemplateColumns: "110px 1fr 80px", gap: 10, alignItems: "center", marginBottom: 8, fontSize: 13 }}>
          <span style={{ color: "#DCE3F5" }}>{n}</span>
          <div style={{ height: 8, background: "rgba(255,255,255,.08)", borderRadius: 999 }}><div style={{ width: (100 * v / max) + "%", height: "100%", borderRadius: 999, background: best ? "linear-gradient(90deg,#00D4FF,#3B82FF)" : "rgba(255,255,255,.28)" }} /></div>
          <b style={{ textAlign: "right", color: best ? "#fff" : "#B6C0D9", fontVariantNumeric: "tabular-nums" }}>{new Intl.NumberFormat("tr-TR").format(v)} ₺</b>
        </div>
      ))}
    </div>
  );
}
