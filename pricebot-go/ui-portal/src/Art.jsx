import React from "react";
// Konsept gorseller: otel / seyahat temali inline SVG (dis kaynak gerektirmez)
export const Logo = () => (
  <svg viewBox="0 0 48 48" fill="none"><rect x="4" y="12" width="40" height="30" rx="4" fill="#fff" opacity=".15"/><rect x="10" y="18" width="8" height="8" rx="1.5" fill="#fff"/><rect x="20" y="18" width="8" height="8" rx="1.5" fill="#fff"/><rect x="30" y="18" width="8" height="8" rx="1.5" fill="#fff"/><rect x="10" y="29" width="8" height="8" rx="1.5" fill="#fde68a"/><rect x="20" y="29" width="8" height="13" rx="1.5" fill="#fff"/><rect x="30" y="29" width="8" height="8" rx="1.5" fill="#fff"/><path d="M4 12l20-8 20 8" stroke="#fff" strokeWidth="2.5" strokeLinecap="round"/></svg>
);
export const HeroArt = () => (
  <svg className="art" viewBox="0 0 420 160" fill="none">
    <circle cx="330" cy="40" r="26" fill="#fde68a" opacity=".9"/>
    <path d="M0 140 Q60 95 120 120 T240 115 T360 125 T420 118 V160 H0Z" fill="#67e8f9" opacity=".35"/>
    <path d="M0 150 Q70 120 140 140 T280 135 T420 140 V160 H0Z" fill="#a5f3fc" opacity=".45"/>
    <rect x="150" y="60" width="90" height="80" rx="6" fill="#fff" opacity=".92"/>
    <rect x="162" y="72" width="14" height="14" rx="2" fill="#0e7490"/><rect x="188" y="72" width="14" height="14" rx="2" fill="#0e7490"/><rect x="214" y="72" width="14" height="14" rx="2" fill="#0e7490"/>
    <rect x="162" y="96" width="14" height="14" rx="2" fill="#f59e0b"/><rect x="188" y="96" width="14" height="44" rx="2" fill="#0e7490"/><rect x="214" y="96" width="14" height="14" rx="2" fill="#0e7490"/>
    <path d="M150 60 L195 38 L240 60" stroke="#fff" strokeWidth="5" strokeLinecap="round" strokeLinejoin="round"/>
    <path d="M60 80 l30-6 -4 10 z" fill="#fff"/><path d="M62 78 l-18 4" stroke="#fff" strokeWidth="3" strokeLinecap="round"/>
    <path d="M270 30 l14 -3 -2 6 z" fill="#fff" opacity=".8"/>
    <path d="M300 150 q10 -40 40 -30 q-5 20 -40 30z" fill="#16a34a" opacity=".7"/><path d="M320 150 q-3 -30 12 -45" stroke="#166534" strokeWidth="3"/>
  </svg>
);
export const LoginArt = () => (
  <svg className="big" viewBox="0 0 520 300" fill="none">
    <circle cx="430" cy="60" r="38" fill="#fde68a"/>
    <path d="M0 240 Q90 180 180 215 T360 205 T520 215 V300 H0Z" fill="#22d3ee" opacity=".45"/>
    <path d="M0 260 Q110 225 210 250 T420 240 T520 250 V300 H0Z" fill="#a5f3fc" opacity=".5"/>
    <rect x="170" y="90" width="180" height="150" rx="10" fill="#fff" opacity=".95"/>
    {[0,1,2,3].map(i => [0,1,2].map(j => <rect key={i+"-"+j} x={190+ j*50} y={110 + i*30} width="28" height="18" rx="3" fill={i===1&&j===1?"#f59e0b":"#0e7490"} opacity={i===3?.6:1}/>))}
    <rect x="245" y="200" width="30" height="40" rx="3" fill="#0e7490"/>
    <path d="M170 90 L260 50 L350 90" stroke="#fff" strokeWidth="9" strokeLinecap="round" strokeLinejoin="round"/>
    <path d="M60 120 l44 -9 -6 14 z" fill="#fff"/><path d="M62 118 l-30 7" stroke="#fff" strokeWidth="4" strokeLinecap="round"/>
    <path d="M400 240 q15 -60 60 -45 q-8 30 -60 45z" fill="#16a34a" opacity=".8"/><path d="M425 240 q-5 -45 20 -65" stroke="#14532d" strokeWidth="4"/>
    <g opacity=".9"><rect x="40" y="190" width="70" height="46" rx="6" fill="#fff"/><text x="75" y="213" textAnchor="middle" fontSize="13" fontWeight="700" fill="#0e7490">₺ 31.110</text><text x="75" y="228" textAnchor="middle" fontSize="9" fill="#64748b">en uygun</text></g>
  </svg>
);
export const Icon = ({ name }) => {
  const d = {
    home: "M3 11l9-8 9 8v9a1 1 0 0 1-1 1h-5v-6H9v6H4a1 1 0 0 1-1-1z",
    hotel: "M3 21V8l9-5 9 5v13M7 21v-6h4v6m2 0v-6h4v6M8 11h.01M12 11h.01M16 11h.01",
    search: "M11 4a7 7 0 1 1 0 14 7 7 0 0 1 0-14zm9 17l-4.3-4.3",
    history: "M3 12a9 9 0 1 0 3-6.7M3 3v5h5M12 7v5l3 2",
    clock: "M12 3a9 9 0 1 1 0 18 9 9 0 0 1 0-18zm0 4v5l3 3",
    settings: "M12 8a4 4 0 1 1 0 8 4 4 0 0 1 0-8zm8 4l2-1-2-3-2 1-2-1V5h-4v3l-2 1-2-1-2 3 2 1v2l-2 1 2 3 2-1 2 1v3h4v-3l2-1 2 1 2-3-2-1z",
    sites: "M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zm0 0c3 3 3 17 0 20M2 12h20M4 7h16M4 17h16",
  }[name];
  return <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d={d}/></svg>;
};
