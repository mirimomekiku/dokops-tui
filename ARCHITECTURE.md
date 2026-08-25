# Architecture Design Document: dok-ops

## 1. System Philosophy & Design Principles

`dok-ops` is engineered as a high-density, low-overhead Operational Cockpit for DevOps engineers, Site Reliability Engineers (SREs), and systems administrators. 

### Core Principles
1. **Zero Context Switching**: Consolidate disparate command-line tools (`htop`, `ss`, `nginx -t`, `certbot`, `lazygit`, `systemctl`, `journalctl`, `dig`, `curl`, `supervisorctl`) into a unified reactive TUI.
2. **Deterministic Concurrency & Value Immutability**: Built strictly on the Elm / Bubble Tea model (`Init`, `Update`, `View`). All background polling, network sweeps, and process inspections execute via non-blocking asynchronous `tea.Cmd` closures that return typed messages back into the main event loop.
3. **Graceful Fallbacks & Offline Resilience**: When optional system daemons (Docker, Systemd, Nginx, Supervisor) or elevated permissions are unavailable, views gracefully enter offline fallback modes without crashing the core runtime.
4. **Isolated Raw PTY Multiplexing**: Embedded subshells run inside a true POSIX pseudo-terminal (`creack/pty`) with complete keystroke isolation, preventing shell key captures from corrupting TUI state transitions.

---

## 2. Component Hierarchy & Flow

```
+-----------------------------------------------------------------------------------+
|                                     main.go                                       |
|                  Entry point: initializes Bubble Tea Program                       |
+-----------------------------------------------------------------------------------+
                                         |
                                         v
+-----------------------------------------------------------------------------------+
|                                 app/model.go                                      |
|                  Master App Model (Holds 24 Subview Models)                       |
+-----------------------------------------------------------------------------------+
                                         |
            +----------------------------+----------------------------+
            |                                                         |
            v                                                         v
+-----------------------+                                 +-----------------------+
|     app/update.go     |                                 |      app/view.go      |
|  - Global Hotkeys     |                                 |  - Top Tab Bar        |
|  - Window Resizing    |                                 |  - Host Load & Uptime |
|  - Message Dispatch   |                                 |  - Active Subview     |
|  - Value Receivers    |                                 |  - Contextual Footer  |
+-----------------------+                                 +-----------------------+
            |
            +---------------------------------------------------------+
            |                                                         |
            v                                                         v
+------------------------------------+   +------------------------------------+
|       Telemetry Subsystems         |   |       Orchestration & WebOps       |
| - views/monitor (CPU/Mem/Procs)    |   | - views/containers (Docker SDK)    |
| - views/bandwidth (Traffic Rates)  |   | - views/services (Systemd DBus)    |
| - views/disk (Filesystem Indexer)  |   | - views/nginx (VHost Manager)      |
| - views/ports (Socket Mapper)      |   | - views/autonginx (Auto-Detector)  |
| - views/scanner (TCP Sweep Pool)   |   | - views/deploy (Multi-Repo Hub)    |
| - views/httpclient (Latency Trace) |   | - views/phpfpm (FPM & OPcache)     |
| - views/dns (DNS Multi-Resolver)   |   | - views/workers (Supervisor/Pulse) |
| - views/ssh (Session Auditor)      |   | - views/certbot (ACME SSL Wizard)  |
| - views/database (Postgres/MySQL)  |   | - views/timers (Cron & Timers)     |
| - views/git (Branch & Diff Viewer) |   | - views/knife (DevOps CyberChef)   |
| - views/ci (GitHub Actions Stream) |   | - views/env (.Env Drift Validator) |
| - views/ssl (TLS Trust Inspector)  |   | - views/terminal (Embedded PTY)    |
+------------------------------------+   +------------------------------------+
```

---

## 3. Message Routing & Asynchronous Polling Protocol

### Event Loop Architecture
The application runs a single-threaded message queue inside Bubble Tea. No background thread directly modifies shared UI state. All concurrent routines dispatch typed structs that are processed sequentially in `app/update.go`:

```
[Background Goroutine / tea.Cmd]
               |
               | Returns typed tea.Msg (e.g., monitor.StatsMsg, certbot.CertbotResultMsg)
               v
     [Main Bubble Tea Program Loop]
               |
               v
       [app.Model.Update(msg)]
               |
       +-------+-------+
       |               |
   [Global Cmd]   [Route to Active View]
       |               |
       v               v
 [Re-render]    [SubView.Update(msg)]
```

### Key Message Types

1. **Telemetry & Timers**:
   - `monitor.StatsMsg`: Contains per-core CPU slices, RAM allocation bytes, and sorted process lists.
   - `bandwidth.NetBandwidthMsg`: Ingress/egress delta bytes calculated against prior sample timestamps.
   - `timers.TimersLoadedMsg`: Aggregated cron and systemd timer schedules with next execution timestamps.

2. **Network & System Responses**:
   - `scanner.ScanFinishedMsg`: Results from concurrent worker pool sweeps with banner strings and round-trip times.
   - `httpclient.ResponseResultMsg`: HTTP connection breakdown phases measured via `httptrace.ClientTrace`.
   - `database.QueryResultMsg`: Tabular SQL results and execution latencies.

3. **Pipelines & Operations**:
   - `deploy.PipelineProgressMsg`: Real-time log chunks from deployment scripts.
   - `certbot.CertbotResultMsg`: Verification logs from ACME challenge execution.
   - `terminal.TerminalDataMsg`: Byte buffers emitted from the child pseudo-terminal slave.

---

## 4. Concurrency & Memory Safety Model

1. **Value Receiver Semantics**: Bubble Tea models are copied by value during update transitions. Non-pointer models containing mutex locks (`sync.Mutex`) cause runtime vet failures if copied. To maintain total safety:
   - Concurrency locks are confined strictly to one-off worker routines (`sync.Mutex` inside `tea.Cmd`).
   - Terminal PTY streaming uses pure event-driven message dispatching (`TerminalDataMsg`) rather than embedded mutex locks.
2. **Context Cancellation**: Long-running network queries (Database Pings, DNS lookups, Certbot ACME runs) instantiate bounded contexts (`context.WithTimeout`) to prevent hanging processes.
3. **Terminal PTY Isolation**: The PTY subshell allocates a master/slave pseudo-terminal pair. When focused (`i`), all raw keystrokes are written to the PTY master. When unfocused (`Ctrl+]` or `F1`-`F12`), keystrokes route back to the global TUI navigation handler.

---

## 5. Visual Hierarchy & Theme Tokens (`internal/theme`)

The user interface follows Tokyo Night / Cyberpunk design tokens rendered using `lipgloss`:

```
+--------------------------------------------------------------------------+
| Color Tokens                                                             |
| - ColorPrimary:   #7aa2f7 (Tokyo Blue - Brand Accent & Headers)          |
| - ColorSecondary: #bb9af7 (Purple - Secondary Actions & Subheaders)      |
| - ColorSuccess:   #9ece6a (Green - Active, Healthy, Synced)              |
| - ColorDanger:    #f7768e (Red - Failed, Conflict, High Load)            |
| - ColorWarning:   #e0af68 (Yellow/Amber - In Progress, Modified, Unsaved)|
| - ColorInfo:      #7dcfff (Cyan - DNS, Metrics, Network Metadata)        |
| - ColorHighlight: #ff9e64 (Orange - Selection Focus & Alerts)            |
| - ColorMuted:     #565f89 (Slate - Table Borders & Keybinding Hints)     |
| - ColorSurface:   #1a1b26 (Deep Dark - Background Container Surfaces)    |
+--------------------------------------------------------------------------+
```

---

## 6. Error Handling & Privilege Isolation

- **Non-Privileged Degradation**: If executed without `sudo` / `root`, commands that write to `/etc/nginx/` or reload system daemons capture `exit status 1` and display clear elevation notices without terminating the UI.
- **Subprocess Sanitization**: Shell command arguments are passed via discrete argv slices (`exec.Command("git", "pull", "--ff-only")`) rather than string interpolation to eliminate shell injection vulnerabilities.
