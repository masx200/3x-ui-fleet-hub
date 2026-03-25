# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

3X-UI is a web-based control panel for managing Xray-core servers. It's a Go application using the Gin web framework with embedded static assets, SQLite database, and Telegram bot integration.

## Development Commands

```bash
# Build
go build -o bin/3x-ui.exe ./main.go

# Run (with debug logging)
XUI_DEBUG=true go run ./main.go

# Test
go test ./...

# Vet
go vet ./...
```

CLI commands available (via `./3x-ui` or `go run ./main.go`):
- `run` - Start the web panel
- `migrate` - Migrate database from old x-ui
- `setting` - Modify panel settings (`-port`, `-username`, `-password`, `-webBasePath`, `-listenIP`, `-reset`, `-show`, `-getCert`, etc.)
- `cert` - Update SSL certificates

## Architecture

### Directory Structure

```
main.go                 # Entry point, signal handling (SIGHUP/SIGTERM/SIGUSR1)
config/                 # Configuration management (embedded version/name files)
database/               # GORM models and SQLite initialization
  ├── model/model.go    # All DB models with auto-migration
web/                    # Main web server (Gin)
  ├── controller/       # HTTP handlers (use *gin.Context)
  ├── service/          # Business logic layer
  ├── job/              # Cron-based background tasks
  ├── entity/           # Request/response DTOs
  ├── middleware/       # Gin middlewares
  ├── locale/           # i18n helpers
  └── websocket/        # WebSocket hub for real-time updates
xray/                   # Xray-core process management and gRPC API client
sub/                    # Subscription server (runs on separate port)
util/                   # Shared utilities (crypto, LDAP, system info)
```

### Core Architectural Patterns

**Embedded Resources**: All web assets are embedded at compile time using `//go:embed`:
- `web/assets` → `assetsFS` (CSS, JS, fonts)
- `web/html` → `htmlFS` (Vue.js templates)
- `web/translation` → `i18nFS` (TOML translation files)

Changes to HTML/CSS/JS require recompilation - there is no hot-reload in production mode.

**Dual Server Design**: Two servers run concurrently:
1. Main web panel (configurable port, default 2053)
2. Subscription server (separate port for client subscription URLs)

Both managed via `web/global` package singleton pattern.

**Xray Integration Pattern**:
- Panel generates `config.json` dynamically from database inbounds/outbounds
- Xray binary is downloaded separately by install scripts to `{bin_folder}/xray-{os}-{arch}`
- Communication via gRPC API for real-time traffic stats (`xray/api.go`)
- Process lifecycle managed in `xray/process.go`

**Signal-Based Restart**:
- SIGHUP triggers graceful restart of both web and sub servers
- SIGUSR1 restarts only Xray-core
- **Critical**: Always call `service.StopBot()` before restart to prevent Telegram bot 409 conflicts
- Signal handlers in `main.go` lines 72-129

**Service Layer Pattern**: Services inject dependencies and operate on GORM models:
```go
type InboundService struct {
    xrayApi xray.XrayAPI
}
func (s *InboundService) GetInbounds(userId int) ([]*model.Inbound, error)
```

**Controller Pattern**: Controllers use Gin context with `I18nWeb(c, "key")` for translations and inherit from `BaseController`.

**Job Scheduling**: Uses `robfig/cron/v3` registered in `web/web.go`. Jobs include:
- Traffic monitoring (`xray_traffic_job.go`)
- CPU alerts (`check_cpu_usage.go`)
- Client IP tracking (`check_client_ip_job.go`)
- LDAP sync (`ldap_sync_job.go`)

**Database Migrations**: Uses `HistoryOfSeeders` model to track one-time migrations. Check this table before adding new migrations.

### Configuration

Environment variables:
- `XUI_DEBUG` - Enable debug logging
- `XUI_LOG_LEVEL` - Set log level (debug, info, notice, warning, error)
- `XUI_MAIN_FOLDER` - Override installation folder path

Database location: `config.GetDBPath()` (typically `/etc/x-ui/x-ui.db`)

### Internationalization

Translation files in `web/translation/translate.*.toml`. Access in controllers via `I18nWeb(c, "pages.login.loginAgain")`. Use `locale.I18nType` enum (Web, Api, etc.).

### Critical Gotchas

1. **Telegram Bot Restart**: Before any server restart (SIGHUP or manual), call `service.StopBot()` to avoid 409 bot conflicts (see `main.go:82-84`)
2. **Embedded Assets**: Frontend changes require full recompilation
3. **IP Limitation**: Implements "last IP wins" - when client exceeds LimitIP, oldest connections are dropped via Xray API
4. **Session Storage**: Uses `gin-contrib/sessions` with cookie store - not distributed-ready
5. **Xray Binary**: Must match OS/arch exactly - managed by installer scripts (`install.sh`)

### External Dependencies

- **Xray-core**: v1.260206.0 (github.com/xtls/xray-core)
- **Gin**: Web framework
- **GORM**: SQLite ORM
- **Telegram Bot**: github.com/mymmrac/telego (long polling)
- **Cron**: github.com/robfig/cron/v3

### Docker

Multi-stage Dockerfile builds with CGO enabled. The `DockerInit.sh` script downloads Xray binary during build. Default port: 2053. Fail2ban pre-configured.
