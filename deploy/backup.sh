#!/usr/bin/env bash
# RFPlay Airport — Automated Database Backup Script
# ==================================================
# Dumps the SQLite database from the manager-data Docker volume,
# saves to deploy/backups/ with timestamp,
# and keeps only the last 7 backups.
#
# Usage:
#   ./deploy/backup.sh                    # Run backup
#   ./deploy/backup.sh /custom/path       # Override backup directory
#
# Cron (daily at 3AM):
#   0 3 * * * /path/to/airport/deploy/backup.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Backup destination (overridable via first argument)
BACKUP_DIR="${1:-$SCRIPT_DIR/backups}"

# Docker volume and container info
DOCKER_VOLUME="rfplay_airport_manager-data"
DB_CONTAINER="airport-manager-1"
DB_PATH="/app/data/manager.db"

# Retention: keep last N backups
RETENTION=7

# Timestamp format
TIMESTAMP="$(date +%Y%m%d_%H%M%S)"

# Ensure backup dir exists
mkdir -p "$BACKUP_DIR"

echo "============================================"
echo " RFPlay Airport — Database Backup"
echo "============================================"
echo "Timestamp:  $TIMESTAMP"
echo "Backup dir: $BACKUP_DIR"
echo ""

# --- Try Docker-based backup first ---
BACKUP_FILE="${BACKUP_DIR}/manager_${TIMESTAMP}.db"
BACKUP_GZ="${BACKUP_FILE}.gz"

if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "^${DB_CONTAINER}$"; then
    echo "[docker] Container '${DB_CONTAINER}' is running."
    echo "[docker] Copying database from container..."
    docker cp "${DB_CONTAINER}:${DB_PATH}" "$BACKUP_FILE" 2>/dev/null || {
        echo "[docker] Direct copy failed, trying Docker volume..."
        # Fallback: use a temporary container to copy from volume
        docker run --rm \
            -v "${DOCKER_VOLUME}:/data:ro" \
            -v "${BACKUP_DIR}:/backup" \
            alpine:3.21 \
            sh -c "cp /data/manager.db /backup/manager_${TIMESTAMP}.db" 2>/dev/null || {
            echo "[ERROR] Could not copy database. Is the volume name correct?"
            echo "  Tried container: ${DB_CONTAINER}"
            echo "  Tried volume:    ${DOCKER_VOLUME}"
            exit 1
        }
    }
elif docker volume ls --format '{{.Name}}' 2>/dev/null | grep -q "^${DOCKER_VOLUME}$"; then
    echo "[docker] Container not running, but volume '${DOCKER_VOLUME}' exists."
    echo "[docker] Copying from volume..."
    docker run --rm \
        -v "${DOCKER_VOLUME}:/data:ro" \
        -v "${BACKUP_DIR}:/backup" \
        alpine:3.21 \
        sh -c "cp /data/manager.db /backup/manager_${TIMESTAMP}.db"
else
    # --- Fallback: check local data directory ---
    LOCAL_DATA="${PROJECT_DIR}/data"
    if [ -f "${LOCAL_DATA}/manager.db" ]; then
        echo "[local] Found database at ${LOCAL_DATA}/manager.db"
        cp "${LOCAL_DATA}/manager.db" "$BACKUP_FILE"
    else
        echo "[ERROR] No database found."
        echo "  Tried Docker container: ${DB_CONTAINER}"
        echo "  Tried Docker volume:    ${DOCKER_VOLUME}"
        echo "  Tried local path:       ${LOCAL_DATA}/manager.db"
        exit 1
    fi
fi

# --- Compress backup ---
if [ -f "$BACKUP_FILE" ]; then
    gzip -f "$BACKUP_FILE"
    echo "[ok] Created: ${BACKUP_GZ}"
    ls -lh "$BACKUP_GZ"
else
    echo "[ERROR] Backup file not found after copy!"
    exit 1
fi

# --- Retention: keep last N, delete older ---
echo ""
echo "[cleanup] Keeping last ${RETENTION} backups..."
COUNT_BEFORE=$(ls -1 "${BACKUP_DIR}"/manager_*.db.gz 2>/dev/null | wc -l)
ls -1t "${BACKUP_DIR}"/manager_*.db.gz 2>/dev/null | tail -n +$((RETENTION + 1)) | while read -r OLD_FILE; do
    rm -f "$OLD_FILE"
    echo "[cleanup] Deleted: $(basename "$OLD_FILE")"
done
COUNT_AFTER=$(ls -1 "${BACKUP_DIR}"/manager_*.db.gz 2>/dev/null | wc -l)
DELETED=$((COUNT_BEFORE - COUNT_AFTER))

echo ""
echo "============================================"
echo " ✅ Backup complete!"
echo "============================================"
echo " File:    ${BACKUP_GZ}"
echo " Size:    $(du -h "$BACKUP_GZ" | cut -f1)"
echo " Count:   ${COUNT_AFTER} backups retained (deleted ${DELETED} old)"
echo " Dir:     ${BACKUP_DIR}"
