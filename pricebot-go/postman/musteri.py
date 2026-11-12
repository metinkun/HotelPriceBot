#!/usr/bin/env python3
"""Tam koleksiyondan MUSTERIYE verilecek koleksiyonu uretir: yalnizca API musterisi uclari (X-API-Key).
Kullanim: python3 postman/musteri.py
Cikti: postman/musteri/SmartRate-API.postman_collection.json + SmartRate-API.postman_environment.json
"""
import json, os, copy

ROOT = os.path.dirname(os.path.abspath(__file__))
src = json.load(open(os.path.join(ROOT, "pricebot.postman_collection.json")))
KEEP = {"Asenkron Is (v1)": "1. Hesap, oteller ve fiyat sorgusu (önerilen)",
        "Fiyat Sorgulama": "2. Senkron fiyat uçları (isteğe bağlı)",
        "Paket (Otel + Ucak + Transfer)": "3. Paket fiyatı (otel + uçak + transfer, paket izni gerekir)",
        "System": "4. Sağlık kontrolü"}
ORDER = {"api/v1/me": 0, "api/v1/hotels": 1, "api/v1/jobs": 2, "api/v1/jobs/:id": 3}

def path_of(it):
    u = it.get("request", {}).get("url", {})
    return "/".join(u.get("path", [])) if isinstance(u, dict) else str(u)

out = copy.deepcopy(src)
out["info"]["name"] = "SmartRate API (müşteri)"
out["info"]["description"] = ("Kimlik: her istekte X-API-Key başlığı (koleksiyon düzeyinde {{apiKey}} değişkeninden gelir). "
    "Önerilen akış: GET /api/v1/me -> GET /api/v1/hotels -> POST /api/v1/jobs -> GET /api/v1/jobs/{{requestId}}?wait=20. "
    "Ayrıntı: docs/api-entegrasyon-rehberi.md veya {{baseUrl}}/docs")
items = []
for folder in src["item"]:
    if folder["name"] not in KEEP:
        continue
    f = copy.deepcopy(folder)
    f["name"] = KEEP[folder["name"]]
    f["item"] = [i for i in f.get("item", []) if not path_of(i).startswith("ready")]
    f["item"].sort(key=lambda i: ORDER.get(path_of(i), 50))
    items.append(f)
items.sort(key=lambda f: f["name"])
out["item"] = items
# Yalnizca apiKey ve requestId degiskenleri gerekir
out["variable"] = [{"key": "baseUrl", "value": "{{baseUrl}}"}]
os.makedirs(os.path.join(ROOT, "musteri"), exist_ok=True)
json.dump(out, open(os.path.join(ROOT, "musteri", "SmartRate-API.postman_collection.json"), "w"), ensure_ascii=False, indent=2)
env = {"name": "SmartRate API", "_postman_variable_scope": "environment", "values": [
    {"key": "baseUrl", "value": "https://hotel.alpkun.com", "enabled": True},
    {"key": "apiKey", "value": "pb_live_... (SmartRate tarafından verilen anahtar)", "enabled": True, "type": "secret"},
    {"key": "requestId", "value": "", "enabled": True}]}
json.dump(env, open(os.path.join(ROOT, "musteri", "SmartRate-API.postman_environment.json"), "w"), ensure_ascii=False, indent=2)
n = sum(len(f["item"]) for f in items)
print(f"musteri koleksiyonu: {len(items)} klasor, {n} istek")
