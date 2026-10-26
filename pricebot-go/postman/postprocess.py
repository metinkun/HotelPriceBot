#!/usr/bin/env python3
"""openapi-to-postmanv2 ciktisini ortam degiskenlerine baglar ve test scriptlerini ekler.
Kullanim: python3 postman/postprocess.py postman/pricebot.postman_collection.json
"""
import json, sys

path = sys.argv[1]
d = json.load(open(path))

def script(lines):
    return {"listen": "test", "script": {"type": "text/javascript", "exec": lines}}

LOGIN_ADMIN = script([
    "pm.test('giris basarili', () => pm.response.to.have.status(200));",
    "const j = pm.response.json(); if (j.token) pm.environment.set('adminToken', j.token);",
])
LOGIN_PORTAL = script([
    "pm.test('giris basarili', () => pm.response.to.have.status(200));",
    "const j = pm.response.json(); if (j.token) pm.environment.set('portalToken', j.token);",
])
JOB_CREATE = script([
    "pm.test('is olusturuldu', () => pm.expect(pm.response.code).to.be.oneOf([200, 202]));",
    "const j = pm.response.json(); if (j.requestId) pm.environment.set('requestId', j.requestId);",
])
JOB_GET = script([
    "const j = pm.response.json();",
    "if (j.status && j.status !== 'done' && j.status !== 'failed') { console.log('is suruyor:', j.status); postman.setNextRequest(pm.info.requestName); }",
    "else pm.test('is tamamlandi', () => pm.expect(j.status).to.eql('done'));",
])
IMPORT_START = script([
    "pm.test('ice aktarma baslatildi', () => pm.response.to.have.status(202));",
    "const j = pm.response.json(); if (j.importId) pm.environment.set('importId', j.importId);",
])
IMPORT_GET = script([
    "const j = pm.response.json();",
    "if (j.status === 'running') { console.log('ice aktarma suruyor:', j.done + '/' + j.total); postman.setNextRequest(pm.info.requestName); }",
    "else pm.test('ice aktarma bitti', () => pm.expect(j.status).to.eql('done'));",
])
SEARCH_CREATE = script([
    "const j = pm.response.json(); const id = j.id || j.searchId || (j.search && j.search.id); if (id) pm.environment.set('searchId', String(id));",
])

BY_PATH = {
    ("POST", "api/admin/auth/login"): LOGIN_ADMIN,
    ("POST", "api/portal/auth/login"): LOGIN_PORTAL,
    ("POST", "api/v1/jobs"): JOB_CREATE,
    ("GET", "api/v1/jobs/:id"): JOB_GET,
    ("POST", "api/admin/hotels/import"): IMPORT_START,
    ("GET", "api/admin/imports/:id"): IMPORT_GET,
    ("POST", "api/portal/searches"): SEARCH_CREATE,
}
PATH_VARS = {"api/v1/jobs/:id": "{{requestId}}", "api/admin/imports/:id": "{{importId}}", "api/portal/searches/:id": "{{searchId}}"}

def token_for(folder):
    return "{{portalToken}}" if folder.lower().startswith("portal") else "{{adminToken}}"

def walk(items, folder=""):
    for it in items:
        if "item" in it:
            walk(it["item"], it["name"]); continue
        req = it["request"]; url = req.get("url") or {}
        p = "/".join(url.get("path", [])) if isinstance(url, dict) else ""
        auth = req.get("auth") or {}
        if auth.get("type") == "bearer":
            for kv in auth["bearer"]:
                if kv["key"] == "token": kv["value"] = token_for(folder)
        if p in PATH_VARS and isinstance(url, dict):
            for v in url.get("variable", []):
                if v.get("key") == "id": v["value"] = PATH_VARS[p]
        sc = BY_PATH.get((req.get("method"), p))
        if sc:
            it["event"] = [e for e in it.get("event", []) if e.get("listen") != "test"] + [sc]

walk(d["item"])
d["auth"] = {"type": "apikey", "apikey": [
    {"key": "key", "value": "X-API-Key", "type": "string"},
    {"key": "value", "value": "{{apiKey}}", "type": "string"},
    {"key": "in", "value": "header", "type": "string"}]}
d["variable"] = [{"key": "baseUrl", "value": "{{baseUrl}}"}]
json.dump(d, open(path, "w"), ensure_ascii=False, indent=1); open(path, "a").write("\n")
print("ok:", path)
