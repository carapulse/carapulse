#!/usr/bin/env bash
set -euo pipefail

# Backup Carapulse Postgres database
# Usage: ./scripts/backup.sh [output-dir]

OUTPUT_DIR="${1:-./backups}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${OUTPUT_DIR}/carapulse_${TIMESTAMP}.sql.gz"

DSN="${CARAPULSE_DSN:-postgres://carapulse:carapulse@localhost:5432/carapulse?sslmode=disable}"

# Parse DSN components
DB_HOST=$(echo "$DSN" | sed -n 's|.*@\(.*\):.*|\1|p' | cut -d/ -f1)
DB_PORT=$(echo "$DSN" | sed -n 's|.*:\([0-9]*\)/.*|\1|p')
DB_NAME=$(echo "$DSN" | sed -n 's|.*/\([^?]*\).*|\1|p')
DB_USER=$(echo "$DSN" | sed -n 's|.*//\(.*\):.*@.*|\1|p')

mkdir -p "$OUTPUT_DIR"

echo "Backing up ${DB_NAME}@${DB_HOST}:${DB_PORT} -> ${BACKUP_FILE}"
PGPASSWORD=$(echo "$DSN" | sed -n 's|.*://[^:]*:\([^@]*\)@.*|\1|p') \
  pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" "$DB_NAME" | gzip > "$BACKUP_FILE"

echo "Backup complete: ${BACKUP_FILE} ($(du -h "$BACKUP_FILE" | cut -f1))"
