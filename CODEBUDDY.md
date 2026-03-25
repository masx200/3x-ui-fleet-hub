# CODEBUDDY.md

This file provides guidance to CodeBuddy Code when working with code in this repository.

## Project Overview

3X-UI is a web-based control panel for managing Xray-core proxy servers. It is a Go application using the Gin web framework with embedded static assets, SQLite (GORM) database, and gRPC communication with the Xray-core process.

## Build & Development Commands

```bash
# Build
go build -o bin/3x-ui.exe ./main.go

# Run with debug mode (serves assets from disk for hot-reload)
XUI_DEBUG=true go run ./main.go

# Local setup: create "x-ui" directory in project root, rename .env.example to .env

# Lint
go vet ./...

# Test
go test ./...

# Docker build
docker build -t 3x-ui .

# CLI operations (run the built binary)
./bin/3x-ui setting --show          # Display current settings
./bin/3x-ui setting --reset         # Reset settings to defaults
```

**Note**: No test files (`*_test.go`) currently exist in the codebase. The CI pipeline runs `gofmt` and `go vet` for static analysis.

## Architecture

### Dual Server Design

Two HTTP servers run concurrently from `main.go`:
- **Main panel** (Gin, default port 2053): Web UI, REST API, WebSocket
- **Subscription server** (`sub/`): Separate port, handles client subscription endpoints

### Core Components

| Component | Location | Purpose |
|-----------|----------|---------|
| Entry point | `main.go` | Init DB, web server, sub server; handles SIGHUP/SIGTERM/SIGUSR1 signals |
| Web server | `web/web.go` | Gin router setup, embeds assets, registers middleware/controllers/jobs |
| Controllers | `web/controller/` | HTTP handlers using `*gin.Context` |
| Services | `web/service/` | Business logic layer (InboundService, TgBot, XrayService, etc.) |
| Background jobs | `web/job/` | Cron tasks via `robfig/cron/v3` (traffic monitoring, CPU checks, IP enforcement, log cleanup) |
| Xray integration | `xray/` | Manages Xray binary process lifecycle, gRPC API for traffic stats and inbound/user management |
| Database | `database/` | GORM + SQLite; models in `database/model/model.go`; auto-migrated on startup |
| Subscription | `sub/` | Separate HTTP server for client subscription endpoints |
| Config | `config/` | Reads embedded version/name files and env vars (`XUI_DEBUG`, `XUI_LOG_LEVEL`, `XUI_BIN_FOLDER`, `XUI_DB_FOLDER`, `XUI_LOG_FOLDER`) |
| Utilities | `util/` | `common/`, `crypto/`, `ldap/`, `random/`, `sys/` (OS-specific operations) |

### Embedded Resources

All frontend assets are embedded into the binary via `//go:embed` in `web/web.go`:
- `web/assets` -> CSS, JS (Vue.js + Ant Design Vue components)
- `web/html` -> Go html/template files (65+ pages/modals/forms)
- `web/translation` -> TOML i18n files (13 languages)

In debug mode (`XUI_DEBUG=true`), assets are served from disk instead.

### Signal Handling

- `SIGHUP`: Graceful restart (stops and recreates servers)
- `SIGUSR1`: Restart only Xray core
- `SIGTERM`: Full shutdown

### Xray Process Management

The panel spawns an external `xray` binary. It dynamically generates `config.json` from database-stored inbound/outbound settings. Runtime operations (add/remove users, query traffic) use gRPC API.

Key paths (production):
- Xray binary: `{bin_folder}/xray-{os}-{arch}`
- Xray config: `{bin_folder}/config.json`
- GeoIP/GeoSite: `{bin_folder}/geoip.dat`, `geosite.dat`

### Database

SQLite via GORM. Key models: `User`, `Inbound`, `OutboundTraffics`, `Setting`, `InboundClientIps`, `ClientTraffic`, `HistoryOfSeeders`. One-time migrations use `HistoryOfSeeders` to prevent re-execution. Default credentials: admin/admin (bcrypt hashed).

## Code Conventions

### Service Layer

Services receive injected dependencies (e.g., `xray.XrayAPI`) and operate on GORM models:
```go
type InboundService struct {
    xrayApi xray.XrayAPI
}
func (s *InboundService) GetInbounds(userId int) ([]*model.Inbound, error) { ... }
```

### Controllers

Use Gin context and `I18nWeb(c, "key")` for translations. Auth is enforced via `checkLogin` middleware.

### Internationalization

Translation files: `web/translation/translate.*.toml`. Access with `I18nWeb(c, "pages.login.loginAgain")`. Types defined in `locale.I18nType` (Web, Api, etc.).

## Critical Gotchas

1. **Telegram bot restart**: Always call `service.StopBot()` before any server restart to prevent 409 bot conflicts
2. **Embedded assets**: Changes to HTML/CSS/JS require recompilation unless running with `XUI_DEBUG=true`
3. **Password migration**: The seeder system (`HistoryOfSeeders`) tracks bcrypt migration — check this table
4. **Subscription server port**: Different from the main panel port
5. **Session management**: Uses `gin-contrib/sessions` with cookie store

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `XUI_DEBUG` | Enable debug mode (serve assets from disk, verbose logging) |
| `XUI_LOG_LEVEL` | Set log level |
| `XUI_BIN_FOLDER` | Xray binary and config location |
| `XUI_DB_FOLDER` | Database file location |
| `XUI_LOG_FOLDER` | Log file location |

## Supported Protocols

VMESS, VLESS, Trojan, Shadowsocks, HTTP, SOCKS, Mixed
