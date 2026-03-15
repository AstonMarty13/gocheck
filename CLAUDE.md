# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run the application
go run cmd/main.go

# Build
go build -o gocheck cmd/main.go

# Run tests
go test ./...

# Run a single test
go test ./... -run TestName
```

## Architecture

`gocheck` is a website uptime monitor with a web dashboard. It has no external dependencies (stdlib only).

**Flow:**
1. `readSites("sites.txt")` reads URLs from the file at startup
2. Each URL gets a goroutine running `checkLink`, which performs an HTTP GET and writes the result (`UP`/`DOWN`) to a shared `StatusData` map (mutex-protected)
3. `checkLink` sends the URL back on a channel `c`; a drain goroutine picks it up, sleeps 10s, and re-checks — creating a continuous polling loop per site
4. An HTTP server on `:8080` serves a single route (`/`) that renders an inline Go HTML template showing the current status map

**Key design notes:**
- `sites.txt` (one URL per line) is read once at startup — editing it requires a restart
- Status is stored only in memory; there is no persistence
- The web view is a manual refresh (no auto-refresh or SSE)
