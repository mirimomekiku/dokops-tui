# Package Dependencies Catalog: dok-ops

Manifest of all direct external libraries and Go standard library packages used across `dok-ops`.

---

## 1. Direct External Dependencies

| Package | Version | Primary Role in dok-ops |
|---|---|---|
| `github.com/charmbracelet/bubbletea` | `v0.25.0` | Core Elm architecture TUI framework, event loop & command runner. |
| `github.com/charmbracelet/lipgloss` | `v0.9.1` | Declarative terminal styling, color token system, layout borders & padding. |
| `github.com/charmbracelet/bubbles` | `v0.18.0` | Prebuilt UI components: `table.Model`, `textinput.Model`, `viewport.Model`, and `spinner.Model`. |
| `github.com/shirou/gopsutil/v3` | `v3.24.1` | Cross-platform host telemetry: CPU per-core metrics, Virtual/Swap memory, Process lists, Network I/O counters, Host info & Load averages. |
| `github.com/coreos/go-systemd/v22` | `v22.5.0` | Direct DBus client for Linux systemd unit management and journal logs. |
| `github.com/docker/docker` | `v25.0.3` | Docker Engine API client for container listing, lifecycle management & log streams. |
| `github.com/docker/go-connections` | `v0.5.0` | Socket dialer and connection helpers for Docker client daemons. |
| `github.com/creack/pty` | `v1.1.21` | POSIX pseudo-terminal (PTY) master/slave allocator for embedded subshells. |
| `github.com/miekg/dns` | `v1.1.58` | Low-level DNS packet construction, resolver queries & record parsing (`dig` alternative). |
| `github.com/golang-jwt/jwt/v5` | `v5.2.0` | JWT decoding, claims extraction, signature inspection & expiration validation. |
| `github.com/robfig/cron/v3` | `v3.0.1` | Cron expression parsing and next-execution timestamp evaluation for Cron & Timers views. |
| `golang.org/x/crypto` | `v0.21.0` | SSH public key parsing (`authorized_keys` auditor) and bcrypt hash computation. |
| `github.com/jackc/pgx/v5` | `v5.5.3` | High-performance PostgreSQL driver (`pgx/v5/stdlib`) for database query execution. |
| `github.com/go-sql-driver/mysql` | `v1.7.1` | MySQL & MariaDB driver for database health inspection and query runner. |
| `github.com/go-git/go-git/v5` | `v5.11.0` | In-memory Git repository inspections, worktree status & commit history. |
| `github.com/google/go-github/v60` | `v60.0.0` | Official GitHub REST API v3 client for workflow runs & CI pipeline monitoring. |
| `github.com/joho/godotenv` | `v1.5.1` | Parsing and comparative schema validation of `.env` files against `.env.example`. |

---

## 2. Standard Library Packages Utilized

| Standard Package | Purpose & Usage |
|---|---|
| `context` | Bounded timeouts (`context.WithTimeout`) for network calls, database queries, and subprocesses. |
| `crypto/tls`, `crypto/x509` | TLS handshake inspection, cipher suite negotiation, SAN extraction & certificate chain analysis. |
| `database/sql` | Generic relational database interface for PostgreSQL and MySQL query dispatch. |
| `encoding/json` | JSON response payload formatting, syntax tree serialization & prettification. |
| `net`, `net/http`, `net/http/httptrace` | TCP port sweeps, DNS resolution, and sub-millisecond connection phase waterfall tracing. |
| `os`, `os/exec` | Host command execution (`git`, `nginx`, `certbot`, `systemctl`, `supervisorctl`, `crontab`). |
| `path/filepath`, `io/fs` | Asynchronous hierarchical filesystem walking and directory signature analysis. |
| `sync` | Worker pool concurrency management (`sync.WaitGroup`, bounded channel semaphores). |
| `text/template` | Production Nginx virtual host configuration generation from embedded templates. |
| `time` | Ticker loops, latency measurements, uptime calculations & human-readable duration formatting. |
