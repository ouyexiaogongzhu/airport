# RFPlay Airport — Database Backup

## Overview

The backup script (`deploy/backup.sh`) creates compressed SQLite database backups
from the `manager-data` Docker volume and stores them in `deploy/backups/`.

**Backup process:**
1. Copies `manager.db` from the running Docker container (`airport-manager-1`)
2. If the container is not running, copies directly from the Docker volume
3. Falls back to local `data/manager.db` if neither Docker source is available
4. Compresses with gzip
5. Retains only the last 7 backups (older ones are deleted)

## Usage

### Manual backup

```bash
# From project root:
cd /path/to/airport
./deploy/backup.sh
```

### Custom backup directory

```bash
./deploy/backup.sh /mnt/backups/airport
```

### Backup file naming

Backups are saved as:
```
deploy/backups/manager_YYYYMMDD_HHMMSS.db.gz
```

Example: `manager_20250624_030000.db.gz`

### Restoring from backup

```bash
# 1. Decompress the backup
gunzip -k deploy/backups/manager_20250624_030000.db.gz

# 2. Stop the manager container
docker compose stop manager

# 3. Copy the database into the Docker volume
docker run --rm \
  -v rfplay_airport_manager-data:/data \
  -v $(pwd)/deploy/backups:/backup \
  alpine:3.21 \
  sh -c "cp /backup/manager_20250624_030000.db /data/manager.db"

# 4. Restart the manager
docker compose start manager
```

## Automated Backup (Cron)

### Option 1: Direct cron entry

Edit your crontab:
```bash
crontab -e
```

Add this line to run backup daily at 3:00 AM:
```
0 3 * * * /path/to/airport/deploy/backup.sh >> /var/log/airport-backup.log 2>&1
```

### Option 2: System-wide cron (recommended for production)

Create `/etc/cron.d/airport-backup`:
```
# RFPlay Airport — Daily database backup at 3:00 AM
0 3 * * * root /path/to/airport/deploy/backup.sh >> /var/log/airport-backup.log 2>&1
```

Then ensure cron is running:
```bash
sudo systemctl enable cron --now
```

## Retention Policy

- **Last 7 daily backups** are retained automatically
- Older backups are deleted each time the script runs
- Adjust `RETENTION=7` in `deploy/backup.sh` to change this

## Verification

Check that backups are being created:
```bash
ls -lh deploy/backups/
```

Test a backup integrity check:
```bash
# Verify the gzip file is valid
gunzip -t deploy/backups/manager_*.db.gz && echo "✅ Backup is valid"
```

## Troubleshooting

| Problem | Likely Cause | Solution |
|---------|-------------|----------|
| "Container not found" | Manager container has different name | Run `docker ps` to find the container name and update `DB_CONTAINER` in the script |
| "Volume not found" | Docker Compose project has different prefix | Run `docker volume ls` and update `DOCKER_VOLUME` in the script |
| Permission denied | Backup directory not writable | `chmod 755 deploy/backups/` |
