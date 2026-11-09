// WebAuthn (passkey / Face ID / Touch ID / Windows Hello) tarayici yardimcilari.
// Sunucu go-webauthn ile konusur; tum ikili alanlar base64url (doldurmasiz).

export function supported() {
  return typeof window !== "undefined" && !!window.PublicKeyCredential && !!navigator.credentials && !!navigator.credentials.create;
}

// Platform dogrulayici (cihazin kendi biyometrisi) var mi? (Promise<boolean>)
export async function platformAvailable() {
  try { return supported() && (await window.PublicKeyCredential.isUserVerifyingPlatformAuthenticatorAvailable()); } catch { return false; }
}

const b64urlToBuf = (s) => {
  if (s instanceof ArrayBuffer) return s;
  const pad = "=".repeat((4 - (s.length % 4)) % 4);
  const bin = atob((s + pad).replace(/-/g, "+").replace(/_/g, "/"));
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out.buffer;
};
const bufToB64url = (buf) => {
  const bytes = new Uint8Array(buf);
  let bin = "";
  for (const b of bytes) bin += String.fromCharCode(b);
  return btoa(bin).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
};

function decodeCreation(pk) {
  const o = { ...pk, challenge: b64urlToBuf(pk.challenge), user: { ...pk.user, id: b64urlToBuf(pk.user.id) } };
  if (pk.excludeCredentials) o.excludeCredentials = pk.excludeCredentials.map((c) => ({ ...c, id: b64urlToBuf(c.id) }));
  return o;
}
function decodeRequest(pk) {
  const o = { ...pk, challenge: b64urlToBuf(pk.challenge) };
  if (pk.allowCredentials) o.allowCredentials = pk.allowCredentials.map((c) => ({ ...c, id: b64urlToBuf(c.id) }));
  return o;
}
function encodeCredential(cred, isCreate) {
  if (typeof cred.toJSON === "function") {
    const j = cred.toJSON();
    if (j && j.response && j.response.clientDataJSON) return j;
  }
  const r = cred.response;
  const out = {
    id: cred.id, rawId: bufToB64url(cred.rawId), type: cred.type,
    authenticatorAttachment: cred.authenticatorAttachment || undefined,
    clientExtensionResults: (cred.getClientExtensionResults && cred.getClientExtensionResults()) || {},
    response: { clientDataJSON: bufToB64url(r.clientDataJSON) },
  };
  if (isCreate) {
    out.response.attestationObject = bufToB64url(r.attestationObject);
    if (r.getTransports) { try { out.response.transports = r.getTransports(); } catch {} }
  } else {
    out.response.authenticatorData = bufToB64url(r.authenticatorData);
    out.response.signature = bufToB64url(r.signature);
    out.response.userHandle = r.userHandle ? bufToB64url(r.userHandle) : null;
  }
  return out;
}

// Yeni cihaz kaydi: sunucunun BeginRegistration ciktisi (options.publicKey) -> credential JSON
export async function create(options) {
  const pk = options.publicKey || options;
  const publicKey = window.PublicKeyCredential.parseCreationOptionsFromJSON ? window.PublicKeyCredential.parseCreationOptionsFromJSON(pk) : decodeCreation(pk);
  const cred = await navigator.credentials.create({ publicKey });
  return encodeCredential(cred, true);
}

// Kayitli cihazla dogrulama: BeginLogin ciktisi -> assertion JSON
export async function get(options) {
  const pk = options.publicKey || options;
  const publicKey = window.PublicKeyCredential.parseRequestOptionsFromJSON ? window.PublicKeyCredential.parseRequestOptionsFromJSON(pk) : decodeRequest(pk);
  const cred = await navigator.credentials.get({ publicKey });
  return encodeCredential(cred, false);
}

// Kullaniciya okunur hata metni
export function describeError(e) {
  const n = e && e.name;
  if (n === "NotAllowedError") return "Doğrulama iptal edildi ya da zaman aşımına uğradı. Bu cihazda kayıtlı bir anahtar yoksa 'Bu cihazı kaydet' seçeneğini kullanın.";
  if (n === "InvalidStateError") return "Bu cihaz zaten kayıtlı görünüyor; 'Doğrula' ile giriş yapmayı deneyin.";
  if (n === "SecurityError") return "Güvenlik hatası: alan adı veya bağlantı (HTTPS) doğrulamaya uygun değil.";
  if (n === "NotSupportedError") return "Bu cihaz/tarayıcı istenen doğrulama yöntemini desteklemiyor.";
  return (e && e.message) || "Cihaz doğrulaması başarısız.";
}
