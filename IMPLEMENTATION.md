# Implementation Document (`impl.md`)

## Overview

**Tazx** is a lightweight, developer-first CLI tool for real-time server health monitoring, log analysis, smart anomaly diagnostics, and Docker container inspection.

As a Senior Software Engineer and DevOps Engineer, the project was transformed from initial command stubs into a fully functional, production-grade Go CLI application matching all specifications outlined in the project `README.md`.

---

## 🏛️ System Architecture & Package Structure

The codebase adheres to clean Go architecture guidelines, separating CLI commands from core domain logic:

```
tazx/
├── cmd/                # Cobra CLI command definitions & flag handling
│   ├── config.go       # tazx config command
│   ├── cpu.go          # tazx cpu metrics command
│   ├── docker.go       # tazx docker ps / stats command
│   ├── doctor.go       # tazx doctor smart diagnostics command
│   ├── init.go         # tazx init environment setup command
│   ├── logs.go         # tazx logs analysis & tailing command
│   ├── memory.go       # tazx memory metrics command
│   ├── monitor.go      # tazx monitor continuous dashboard
│   ├── root.go         # Cobra root command & persistent flags
│   ├── status.go       # tazx status overview command
│   ├── storage.go      # tazx storage & partition command
│   ├── top.go          # tazx top process viewer
│   ├── users.go        # tazx users session command
│   ├── version.go      # tazx version command
│   └── watch.go        # tazx watch resource command
├── internal/           # Core domain logic & business packages
│   ├── config/         # Config loading, defaults, YAML persistence
│   ├── docker/         # Moby / Docker SDK container inspection
│   ├── doctor/         # Smart diagnostic rule engine & anomaly detector
│   ├── logs/           # Access log parsing, route analysis, error tailing
│   └── system/         # gopsutil metrics collection (CPU, RAM, Disk, Procs)
├── libs/               # Shared utilities
│   └── colorize.go     # ANSI terminal color formatting & styling
├── main.go             # Entrypoint invoking cmd.Execute()
└── impl.md             # Technical implementation documentation
```

---

## ⚙️ Features Implemented

### 1. Instant Server Status (`tazx status`)
- Aggregates CPU utilization, RAM usage, and Disk partition status.
- Analyzes access logs for real-time traffic (Requests/min, total requests, error rates).
- Highlights active alerts (bot traffic, high disk usage, elevated error rates).

### 2. Smart Diagnostics Engine (`tazx doctor`)
- Detects system resource bottlenecks (CPU > 80/90%, RAM > 85/90%, Disk > 85/90%) and correlates them with top consumer processes.
- Analyzes log entries for security & traffic anomalies:
  - **Bot Scanning Attempt**: Identifies automated probes targeting `/wp-admin`, `/.env`, `/phpmyadmin`, `/.git`, `/xmlrpc.php`.
  - **Brute-Force Detection**: Identifies repeated failed logins (401/403 HTTP statuses).
  - **Error Rate Spikes**: Warns when HTTP 4xx/5xx responses exceed healthy thresholds.
- Delivers actionable developer explanations (`Likely cause: ...`, `→ Recommendation: ...`).

### 3. Log Analysis & Streaming (`tazx logs`)
- Supports Nginx / Apache Combined Access Log format, Common Log Format, and generic server logs.
- Flags implemented:
  - `--errors` (`-e`): Filters entries with HTTP status >= 400.
  - `--routes` (`-r`): Displays traffic breakdown by route, top client IPs, and scan targets.
  - `--path` (`-p`): Overrides target log file path.
  - `--lines` (`-n`): Custom line tail limit.
  - `--follow` (`-f`): Live log streaming (similar to `tail -f`).
- Auto-generates a realistic sample log file if none exists for out-of-the-box demo capability.

### 4. Live Process Monitor (`tazx top`)
- Interactive real-time process table rendering PIDs, Username, Command, CPU %, Memory %, and Status.
- Flags: `--sort-mem` (`-m`), `--limit` (`-l`), `--refresh` (`-r`), `--once`.

### 5. Docker Integration (`tazx docker`)
- Subcommands: `tazx docker ps`, `tazx docker stats`.
- Lists container ID, names, image, status, and port bindings.
- Handles environments without Docker gracefully with helpful troubleshooting advice.

### 6. Subcommands & System Tools
- `tazx cpu`: CPU utilization, logical core topology, clock speed, top CPU processes.
- `tazx memory`: Total/used/free RAM, swap utilization, memory consumer processes.
- `tazx storage`: Disk space on `/`, percentage usage, status indicator (`OK`, `WARNING`, `CRITICAL`).
- `tazx users`: Active user login sessions and terminal details.
- `tazx init`: Creates default configuration (`~/.tazx.yaml`) and sample access log.
- `tazx config`: Displays active configuration parameters.
- `tazx watch`: Continuous watcher for specific targets (`cpu`, `logs`, `docker`).
- `tazx monitor`: Real-time system and traffic dashboard.
- `tazx version`: Displays release version (`v1.0.0`), Go runtime, and target OS/Arch.

---

## 🧪 Testing & Verification

1. **Compilation & Build**:
   ```bash
   go build -o tazx
   ```
   Built cleanly with zero warnings or errors.

2. **Automated Unit Tests**:
   - `internal/config`: TestDefaultConfig, TestSaveAndLoadConfig, TestLoadConfigFallback.
   - `internal/logs`: TestParseLineCombinedLogFormat, TestAnalyzeLogs, TestTailLogs.
   - `internal/doctor`: TestRunDiagnostics (verifying bot scan & brute-force detection).
   
   Command:
   ```bash
   go test -v ./...
   ```
   Result: **PASS** across all packages.

3. **Runtime Verification**:
   - Verified `./tazx status`, `./tazx doctor`, `./tazx logs`, `./tazx top --once`, `./tazx docker`, `./tazx init`, `./tazx config`, `./tazx cpu`, `./tazx memory`, `./tazx storage`, `./tazx users`, `./tazx version`.
