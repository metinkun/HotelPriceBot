#!/usr/bin/env python3
"""Tam koleksiyondan MUSTERIYE verilecek koleksiyonu uretir: yalnizca API musterisi uclari (X-API-Key).
Kullanim: python3 postman/musteri.py
Cikti: internal/api/clientpkg/ (Go binary'ye gomulur; admin > Musteri > "Musteri paketini indir" bu dosyalari
anahtar/adres doldurulmus olarak zip'ler) + docs/api-entegrasyon-rehberi.md kopyasi.
"""
import json, os, copy

ROOT = os.path.dirname(os.path.abspath(__file__))
PKG = os.path.join(ROOT, "..", "internal", "api", "clientpkg")
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
os.makedirs(PKG, exist_ok=True)
json.dump(out, open(os.path.join(PKG, "SmartRate-API.postman_collection.json"), "w"), ensure_ascii=False, indent=2)
import shutil
shutil.copyfile(os.path.join(ROOT, "..", "docs", "api-entegrasyon-rehberi.md"), os.path.join(PKG, "API-Entegrasyon-Rehberi.md"))
n = sum(len(f["item"]) for f in items)
print(f"musteri koleksiyonu: {len(items)} klasor, {n} istek")
