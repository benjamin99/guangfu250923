# Deployment Guide (systemd)

This directory provides sample systemd unit files for the main API service and the self-updater companion.

## Files
- `guangfu250923.service`: Runs the primary API server (Gin + PostgreSQL backend).
- `guangfu-updater.service`: Optional HTTP updater which downloads latest GitHub release binary and restarts the main service on demand.

## Install Binaries
```
# Build locally
GOOS=linux GOARCH=amd64 go build -o /usr/local/bin/guangfu250923 ./cmd/server
GOOS=linux GOARCH=amd64 go build -o /usr/local/bin/guangfu-updater ./cmd/updater

# Create working dir for runtime (logs, temp, etc.)
mkdir -p /var/lib/guangfu250923
chown www-data:www-data /var/lib/guangfu250923
```

## Install Unit Files
```
cp deploy/systemd/guangfu250923.service /etc/systemd/system/
cp deploy/systemd/guangfu-updater.service /etc/systemd/system/

# (Optional) Store secrets separately
# echo 'DB_PASSWORD=***' > /etc/guangfu250923.env
# chmod 600 /etc/guangfu250923.env
# Then add in unit: EnvironmentFile=/etc/guangfu250923.env
```

## Reload & Enable
```
systemctl daemon-reload
systemctl enable --now guangfu250923.service
systemctl enable --now guangfu-updater.service
```

## Test
```
curl -f http://localhost:8080/healthz
curl -f http://localhost:9090/healthz
```

## Trigger Update
```
# Dry run
curl -H "X-API-Key: <your-key>" 'http://localhost:9090/upgrade-service?dry_run=true'
# Real update
curl -H "X-API-Key: <your-key>" -X POST 'http://localhost:9090/upgrade-service'
```

## Nginx Proxy (example)
```
location /internal/upgrade-service {
    proxy_pass http://127.0.0.1:9090/upgrade-service;
    proxy_set_header X-API-Key <your-key>;
    proxy_read_timeout 300s;
}
```

## Rollback (Manual)
If update fails to start:
```
ls -1 /usr/local/bin/guangfu250923.bak.* | tail -n1
cp /usr/local/bin/guangfu250923.bak.<timestamp> /usr/local/bin/guangfu250923
systemctl restart guangfu250923.service
```

## Hardening Suggestions
- Move secrets to EnvironmentFile with correct permissions.
- Add `ProtectSystem=full`, `NoNewPrivileges=yes`, `PrivateTmp=yes` to main service.
- Use a dedicated non-root user for updater if it does not need root (else keep minimal privileges).
- Validate release asset hash against a published checksum file.

## Environment Variables (Main Service)
| Variable | Default | Description |
|----------|---------|-------------|
| DB_HOST | localhost | PostgreSQL host |
| DB_PORT | 5432 | PostgreSQL port |
| DB_USER | postgres | PostgreSQL user |
| DB_PASSWORD | postgres | PostgreSQL password |
| DB_NAME | relief | Database name |
| DB_SSLMODE | disable | SSL mode |
| PORT | 8080 | API listen port |
| SHEET_ID | (empty) | Google Sheet ID (optional) |
| SHEET_TAB | (empty) | Sheet tab name |
| SHEET_REFRESH_SEC | 300 | Sheet polling interval seconds |
| ALLOWED_COUNTRIES | (empty) | IP/Country filter allow countries |
| ALLOWED_IPS | (empty) | IP/CIDR allowlist |
| UPDATE_API_KEY | (empty) | Optional: if embedding updater logic |

## Environment Variables (Updater)
| Variable | Default | Description |
|----------|---------|-------------|
| UPDATE_API_KEY | (none) | Required API key to authorize updates |
| UPDATER_LISTEN | :9090 | Updater HTTP listen address |
| SERVICE_NAME | guangfu250923 | Target systemd service to restart |
| INSTALL_PATH | /usr/local/bin/guangfu250923 | Binary path to replace |
| GITHUB_REPO | PichuChen/guangfu250923 | Repository for releases |
| ASSET_PATTERN | (empty) | Substring to match desired asset |

