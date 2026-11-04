#!/usr/bin/env node
// Ekran goruntusu araci (gelistirme): headless Chrome + CDP, puppeteer gerekmez (Node 22 WebSocket).
// Kullanim: node ui-kit/shot.mjs <url> <out.png> [--token KEY=VALUE] [--width 1400] [--height 900] [--mobile] [--dark] [--wait 1500]
//   --token pricebot_admin_token=<jwt>  -> localStorage'a yazilir (giris atlanir)
import { spawn } from "node:child_process";
import { writeFileSync } from "node:fs";

const args = process.argv.slice(2);
const url = args[0], out = args[1];
const opt = (k, d) => { const i = args.indexOf(k); return i >= 0 ? args[i + 1] : d; };
const flag = (k) => args.includes(k);
const width = Number(opt("--width", flag("--mobile") ? 390 : 1400)), height = Number(opt("--height", flag("--mobile") ? 844 : 900));
const token = opt("--token", ""); const wait = Number(opt("--wait", 1600));
const chrome = process.env.CHROME || "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const port = 9222 + Math.floor(Math.random() * 500);
const proc = spawn(chrome, [`--remote-debugging-port=${port}`, "--headless=new", "--no-first-run", "--no-default-browser-check", `--user-data-dir=/tmp/claude-501/shot-${port}`, `--window-size=${width},${height}`, "--hide-scrollbars", "about:blank"], { stdio: "ignore" });
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
let ws, id = 0; const pending = new Map();
const send = (method, params = {}) => new Promise((res, rej) => { const i = ++id; pending.set(i, { res, rej }); ws.send(JSON.stringify({ id: i, method, params })); });
try {
  let targets;
  for (let i = 0; i < 40; i++) { try { targets = await (await fetch(`http://127.0.0.1:${port}/json`)).json(); if (targets?.length) break; } catch {} await sleep(250); }
  const page = targets.find((t) => t.type === "page");
  ws = new WebSocket(page.webSocketDebuggerUrl);
  await new Promise((r) => (ws.onopen = r));
  ws.onmessage = (ev) => { const m = JSON.parse(ev.data); if (m.id && pending.has(m.id)) { const p = pending.get(m.id); pending.delete(m.id); m.error ? p.rej(new Error(m.error.message)) : p.res(m.result); } };
  await send("Page.enable"); await send("Runtime.enable");
  await send("Emulation.setDeviceMetricsOverride", { width, height, deviceScaleFactor: 2, mobile: flag("--mobile") });
  if (flag("--mobile")) await send("Emulation.setTouchEmulationEnabled", { enabled: true });
  const origin = new URL(url).origin;
  // Once ayni origin'de bos bir sayfaya git ve origin oturana kadar bekle (localStorage dogru origin'e yazilsin)
  await send("Page.navigate", { url: origin + "/favicon.svg" });
  for (let i = 0; i < 40; i++) { const r = await send("Runtime.evaluate", { expression: "location.origin", returnByValue: true }); if (r.result?.value === origin) break; await sleep(100); }
  const setup = [];
  if (token) { const [k, ...v] = token.split("="); setup.push(`localStorage.setItem(${JSON.stringify(k)}, ${JSON.stringify(v.join("="))});`); }
  setup.push(`localStorage.setItem("smartrate_theme", ${JSON.stringify(flag("--dark") ? "dark" : "light")});`);
  await send("Runtime.evaluate", { expression: setup.join("") });
  await send("Page.navigate", { url }); await sleep(wait);
  const metrics = await send("Page.getLayoutMetrics");
  const fullH = flag("--full") ? Math.min(Math.ceil(metrics.cssContentSize.height), 6000) : height;
  if (flag("--full")) await send("Emulation.setDeviceMetricsOverride", { width, height: fullH, deviceScaleFactor: 2, mobile: flag("--mobile") });
  await sleep(200);
  const shot = await send("Page.captureScreenshot", { format: "png", captureBeyondViewport: true });
  writeFileSync(out, Buffer.from(shot.data, "base64"));
  console.log("ok", out, `${width}x${fullH}`);
} catch (e) { console.error("HATA:", e.message); process.exitCode = 1; }
finally { try { ws?.close(); } catch {} proc.kill("SIGKILL"); }
