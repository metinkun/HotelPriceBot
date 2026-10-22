import React from "react";

export default function Sites({ me }) {
  return (
    <div className="card">
      <h3>Fiyat kaynakları</h3>
      <p className="muted">Hesabınız için etkin olan siteler aşağıda. Site eklemek/kapatmak için hesap yöneticinizle görüşün.</p>
      <table>
        <thead><tr><th>Site</th><th>Alan adı</th><th>Durum</th><th>Paket</th><th>Not</th></tr></thead>
        <tbody>{me.providers.map((p) => (
          <tr key={p.name}><td><b>{p.label}</b></td><td className="muted">{p.domain}</td>
            <td>{p.enabled ? <span className="badge ok">etkin</span> : <span className="badge">kapalı</span>}</td>
            <td>{p.supportsPackages ? "✓" : "—"}</td>
            <td className="muted">{p.requiresBrowser ? "Tarayıcı tabanlı; sorgu birkaç saniye daha uzun sürebilir." : ""}</td></tr>))}</tbody>
      </table>
    </div>
  );
}
