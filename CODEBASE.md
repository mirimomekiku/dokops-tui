# Codebase Deep Dive: dok-ops

Technical specification covering internal design patterns, state management lifecycles, and subview implementations across `dok-ops`.

---

## 1. Elm / Bubble Tea Architecture Implementation

The application strictly obeys the Elm architecture:

```
                  +-------------------------+
                  |                         |
                  |     tea.Model State     |
                  |                         |
                  +-------------------------+
                     |                   ^
                     | View()            | Update()
                     v                   |
             +---------------+   +---------------+
             | Formatted TUI |   |   Incoming    |
             | String Output |   | tea.Msg Event |
             +---------------+   +---------------+
                     |                   ^
                     v                   |
             +---------------+   +---------------+
             | Terminal Emul |-->| User Keyboard |
             | Rendering     |   | / Async Cmd   |
             +---------------+   +---------------+
```

### State Isolation Contract
- **No Shared Mutable State**: Subviews do not hold references to each other's models. Communication occurs exclusively through parent `app/update.go` routing.
- **Value Receiver Purity**: Methods receive copies of `Model` (`m Model`) and return modified copies (`(Model, tea.Cmd)`). This prevents race conditions and memory leaks.

---

## 2. Telemetry Subsystem Implementation

### `views/monitor/monitor.go`
- **CPU Profiling**: Calls `cpu.Percent(0, true)` on 1-second ticks to calculate individual core load slices. Renders per-core progress bars via `theme.RenderProgressBar`.
- **Memory & Swap**: Queries `mem.VirtualMemory()` and `mem.SwapMemory()`, generating visual gauges via `theme.RenderGauge`.
- **Process Table**: Fetches all system processes via `process.Processes()`, extracts CPU%, Memory%, PID, and Process Name, sorts dynamically (`c`, `m`, `p`, `n`), and triggers `syscall.SIGTERM` when `k` is pressed.

### `views/bandwidth/bandwidth.go`
- **Delta Sampling**: Samples `psnet.IOCounters(true)` every second.
- **Rate Calculation**: Compares current byte counters against `m.lastIOCounters[iface]`, dividing the delta by elapsed wall-clock seconds to yield precise ingress/egress transfer rates in KB/s and MB/s.

### `views/ports/ports.go`
- **Socket Enumeration**: Calls `psnet.Connections("all")` to discover active listening sockets and established TCP/UDP connections.
- **Process Mapping**: Maps bound PIDs to process command lines via `process.NewProcess(pid)`.
- **Conflict Killer**: Identifies conflicting PIDs occupying required ports and dispatches `syscall.SIGKILL`.

---

## 3. WebOps & Reverse Proxy Automation Implementation

### `views/autonginx/autonginx.go`
- **Signature Detection Engine**:
  - `artisan` present $\rightarrow$ Laravel $\rightarrow$ root set to `/var/www/{project}/public`, FastCGI PHP socket generated.
  - `package.json` present $\rightarrow$ Node/Next.js $\rightarrow$ reverse proxy upstream `http://127.0.0.1:{port}`.
  - `dist/` or `build/` present $\rightarrow$ Static SPA $\rightarrow$ fallback `try_files $uri $uri/ /index.html`.
  - `wp-config.php` present $\rightarrow$ WordPress $\rightarrow$ rewrite rules and uploads directory protection.
- **Template Rendering**: Uses standard library `text/template` with embedded production configurations.
- **Automated Pipeline**: Atomically writes `/etc/nginx/sites-available/{{.Domain}}.conf`, symlinks `/etc/nginx/sites-enabled/`, runs `nginx -t`, and executes `systemctl reload nginx`.

### `views/deploy/deploy.go`
- **Multi-Repo Tracking**: Inspects Git repositories under `/var/www/`, reading `.git/HEAD` and running `git status --porcelain` to identify uncommitted changes and synchronization status.
- **Zero-Downtime Pipeline**: Streams multi-step deployment output sequentially:
  1. `php artisan down --retry=60`
  2. `git pull origin {{branch}}`
  3. `composer install --no-dev --optimize-autoloader`
  4. `php artisan migrate --force`
  5. `php artisan config:cache`
  6. `npm run build`
  7. `php artisan up`

### `views/certbot/certbot.go`
- **DNS Pre-Validation**: Performs asynchronous `net.LookupIP(domain)` to verify DNS resolution before contacting ACME servers.
- **Dry-Run Testing**: Runs `certbot --nginx -d {{domain}} --dry-run` to test challenge authorization without consuming Let's Encrypt rate limits.
- **Live Provisioning**: Dispatches `certbot --nginx -d {{domain}} --non-interactive --agree-tos -m {{admin_email}} --redirect`.

---

## 4. Diagnostics & Security Subsystem Implementation

### `views/scanner/scanner.go`
- **Worker Pool**: Instantiates a bounded channel semaphore (`make(chan struct{}, 30)`) and `sync.WaitGroup` to scan up to 30 ports simultaneously.
- **Banner Grabbing**: Establishes `net.DialTimeout("tcp", addr, 1200ms)`, sends `\r\n`, and reads preliminary protocol greetings within a 500ms deadline.

### `views/httpclient/httpclient.go`
- **Latency Breakdown**: Injects `httptrace.ClientTrace` into outgoing `http.Request` contexts:
  - `DNS`: `DNSStart` to `DNSDone`
  - `TCP`: `ConnectStart` to `ConnectDone`
  - `TLS`: `TLSHandshakeStart` to `TLSHandshakeDone`
  - `TTFB`: Request sent to `GotFirstResponseByte`
  - `Transfer`: First byte to `io.ReadAll(resp.Body)` completion

### `views/dns/dns.go`
- **Miekg DNS Query Dispatch**: Constructs standard `dns.Msg` packets for `A`, `AAAA`, `CNAME`, `MX`, `TXT`, `NS`, `SOA`, `SRV`, and `PTR` records, transmitting them via `dns.Client` to specified resolver endpoints (Cloudflare, Google, Quad9, Local).

### `views/terminal/terminal.go`
- **POSIX PTY Allocation**: Uses `creack/pty.Start(cmd)` on `$SHELL` or `/bin/bash`.
- **Raw Stream Loop**: Spawns a background read goroutine streaming PTY output buffers into `TerminalDataMsg` events. Resizes the child PTY window synchronously upon receiving `tea.WindowSizeMsg`.
