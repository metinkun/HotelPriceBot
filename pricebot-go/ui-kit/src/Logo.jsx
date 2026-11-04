import React from "react";

// SmartRate logo: vektor yeniden cizim (marka PDF'indeki ana logo). Isaret = iki egik yaprak (S) + yukselen 3 cubuk.
let uid = 0;
export function LogoMark({ size = 36, mono = false, color = "#fff" }) {
  const id = React.useMemo(() => "srg" + (++uid), []);
  const fill = mono ? color : `url(#${id})`;
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none" aria-hidden="true">
      {!mono && (
        <defs>
          <linearGradient id={id} x1="8" y1="6" x2="56" y2="58" gradientUnits="userSpaceOnUse">
            <stop offset="0" stopColor="#00D4FF" /><stop offset="0.52" stopColor="#3B82FF" /><stop offset="1" stopColor="#7B2CF7" />
          </linearGradient>
        </defs>
      )}
      <path d="M22.5 4.5h26.3c6 0 9.3 6.9 5.6 11.6L42.2 31.5c-1.5 1.9-3.8 3-6.2 3H9.7c-6 0-9.3-6.9-5.6-11.6L16.3 7.5c1.5-1.9 3.8-3 6.2-3z" fill={fill} opacity={mono ? 1 : 0.95} />
      <path d="M41.5 59.5H15.2c-6 0-9.3-6.9-5.6-11.6L21.8 32.5c1.5-1.9 3.8-3 6.2-3h26.3c6 0 9.3 6.9 5.6 11.6L47.7 56.5c-1.5 1.9-3.8 3-6.2 3z" fill={fill} />
      <rect x="24" y="31" width="4.2" height="8" rx="2" fill={mono ? "#0B1026" : "#fff"} opacity={mono ? 0.9 : 1} />
      <rect x="30.5" y="26" width="4.2" height="13" rx="2" fill={mono ? "#0B1026" : "#fff"} opacity={mono ? 0.9 : 1} />
      <rect x="37" y="21" width="4.2" height="18" rx="2" fill={mono ? "#0B1026" : "#fff"} opacity={mono ? 0.9 : 1} />
    </svg>
  );
}

// Yazi isareti: "Smart" duz, "Rate" gradyan (koyu zeminde Smart beyaz, acik zeminde lacivert)
export function Wordmark({ onDark = true, size = 20 }) {
  const id = React.useMemo(() => "srw" + (++uid), []);
  return (
    <span style={{ display: "inline-flex", alignItems: "baseline", fontWeight: 800, fontSize: size, letterSpacing: "-0.02em", lineHeight: 1, fontFamily: "var(--font)" }}>
      <span style={{ color: onDark ? "#fff" : "#0B1026" }}>Smart</span>
      <svg width="0" height="0" style={{ position: "absolute" }} aria-hidden="true"><defs><linearGradient id={id} x1="0" y1="0" x2="1" y2="1"><stop offset="0" stopColor="#00D4FF" /><stop offset="0.5" stopColor="#3B82FF" /><stop offset="1" stopColor="#7B2CF7" /></linearGradient></defs></svg>
      <span style={{ background: "linear-gradient(135deg,#00D4FF 0%,#3B82FF 50%,#7B2CF7 100%)", WebkitBackgroundClip: "text", backgroundClip: "text", color: "transparent" }}>Rate</span>
    </span>
  );
}

export function Logo({ size = 34, onDark = true, product, textSize }) {
  return (
    <span style={{ display: "inline-flex", alignItems: "center", gap: 10 }}>
      <LogoMark size={size} />
      <span style={{ display: "flex", flexDirection: "column", lineHeight: 1 }}>
        <Wordmark onDark={onDark} size={textSize || Math.round(size * 0.58)} />
        {product && <span className="sr-product" style={{ color: onDark ? "#8EA0C8" : "#64748B", fontSize: 10.5, fontWeight: 700, letterSpacing: ".1em", textTransform: "uppercase", marginTop: 4 }}>{product}</span>}
      </span>
    </span>
  );
}
