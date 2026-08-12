# gocheck

[![CI](https://github.com/AstonMarty13/gocheck/actions/workflows/ci.yml/badge.svg)](https://github.com/AstonMarty13/gocheck/actions/workflows/ci.yml)

A small website uptime monitor with a web dashboard. No external dependencies —
Go standard library only.

![Dashboard](docs/screenshot.png)

## Features

- Monitors a list of URLs read from `sites.txt`
- One polling goroutine per site, each with a bounded HTTP timeout
- Web dashboard showing the current status of every site
- Zero third-party dependencies — stdlib only
- Single static binary, ~9 MB container image built `FROM scratch`
- Graceful shutdown on SIGINT/SIGTERM

## Install

```sh
go install github.com/AstonMarty13/gocheck/cmd@latest
```

Or build from source:

```sh
git clone https://github.com/AstonMarty13/gocheck.git
cd gocheck
go build -o gocheck ./cmd
```

## Usage

Create a `sites.txt`, one URL per line. Blank lines and `#` comments are ignored:

```
# production
https://example.com
https://github.com
```

Then run it:

```sh
./gocheck
```

The dashboard is available at http://localhost:8080.

### Options

| Flag | Default | Description |
| --- | --- | --- |
| `-sites` | `sites.txt` | Path to the file listing URLs to monitor |
| `-addr` | `:8080` | Address the dashboard listens on |
| `-interval` | `10s` | Delay between two checks of the same site |
| `-timeout` | `5s` | Per-request HTTP timeout |

### Docker

```sh
docker build -t gocheck .
docker run -p 8080:8080 -v "$PWD/sites.txt:/etc/gocheck/sites.txt:ro" gocheck
```

The image is built `FROM scratch` and runs as an unprivileged user.

## How it works

At startup, `gocheck` reads the sites file and starts one goroutine per URL.
Each goroutine ticks on `-interval`, issues an HTTP GET bounded by `-timeout`,
and writes the outcome into a shared map guarded by a `sync.RWMutex`. The HTTP
handler calls `Snapshot()`, which returns sorted *copies* of the statuses, so
the dashboard never reads memory that a polling goroutine may be writing.

Every goroutine is tied to a `context.Context` cancelled on SIGINT/SIGTERM, so
the process shuts down cleanly.

State is held in memory only — restarting resets history.

## Development

```sh
go test -race ./...    # unit tests under the race detector
go vet ./...
golangci-lint run
```

CI runs build, vet, gofmt, `go test -race`, golangci-lint, and a Docker image
smoke test on every push and pull request.

## Limitations

Deliberately minimal. There is no persistence, no alerting, and no
authentication on the dashboard. It is meant to run on a trusted network as a
lightweight at-a-glance check, not as a replacement for Prometheus or
Uptime Kuma.

## License

MIT
