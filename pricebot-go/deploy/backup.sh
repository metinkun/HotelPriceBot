#!/bin/sh
# Gunluk pg_dump; KEEP_DAYS gunden eski yedekleri siler.
set -eu
KEEP_DAYS="${KEEP_DAYS:-14}"
while true; do
  TS=$(date +%Y%m%d-%H%M%S)
  OUT="/backups/pricebot-$TS.sql.gz"
  if pg_dump -h postgres -U botuser -d pricebot | gzip > "$OUT"; then
    echo "[backup] $OUT yazildi ($(du -h "$OUT" | cut -f1))"
  else
    echo "[backup] HATA: yedek alinamadi" >&2
    rm -f "$OUT"
  fi
  find /backups -name 'pricebot-*.sql.gz' -mtime +"$KEEP_DAYS" -delete
  sleep 86400
done
