# Tazx v1.1 Implementation Document

## Overview

**Tazx** is a lightweight, read-only Go CLI utility for server health inspection, diagnostics, web access log analysis, listening port inspection, systemd service checks, network overview, and Docker container inspection.

---

## Architecture & File Structure

```text
tazx/
├── main.go                              # Entry point invoking cmd.Execute()
├── go.mod / go.sum                      # Dependencies (cobra, gopsutil/v4, moby/moby, yaml.v3)
├── README.md                            # User & installation documentation
├── IMPLEMENTATION.md                    # Technical architecture & module summary
├── LICENSE                              # MIT License
├── cmd/                                 # Cobra CLI commands & flag handling
│   ├── root.go                          # Root command, --config, -p/--path, --no-color, --json, signalContext
│   ├── status.go                        # `tazx status`
│   ├── doctor.go                        # `tazx doctor`
│   ├── cpu.go                           # `tazx cpu`
│   ├── memory.go                        # `tazx memory`
│   ├── storage.go                       # `tazx storage` (partitions + bounded --large dir scan)
│   ├── users.go                         # `tazx users`
│   ├── ports.go                         # `tazx ports`
│   ├── services.go                      # `tazx services`
│   ├── network.go                       # `tazx network`
│   ├── logs.go                          # `tazx logs`
│   ├── top.go                           # `tazx top`
│   ├── monitor.go                       # `tazx monitor`
│   ├── watch.go                         # `tazx watch` (cpu, logs, docker)
│   ├── docker.go                        # `tazx docker ps` & `tazx docker stats`
│   ├── init.go                          # `tazx init` (optional config creation, --force)
│   ├── config.go                        # `tazx config`
│   ├── update.go                        # `tazx update` / `upgrade` (--check, --version, --force)
│   ├── version.go                       # `tazx version`
│   └── cli_test.go                      # CLI integration & SIGINT tests
├── internal/
│   ├── config/
│   │   ├── config.go                    # YAML config load/save (0600), validation, regular-file checks
│   │   └── config_test.go
│   ├── system/
│   │   ├── system.go                    # CPU, load avg, RAM/available, swap, host, users, ProcessSampler
│   │   ├── storage.go                   # Multi-partition filtering + bounded FindLargeDirs
│   │   └── system_test.go
│   ├── logs/
│   │   ├── logs.go                      # Combined/Common log parser, redaction, ring-buffer tail, FollowLogs
│   │   └── logs_test.go
│   ├── docker/
│   │   ├── docker.go                    # Read-only ContainerList + concurrent ContainerStats calculations
│   │   └── docker_test.go
│   ├── ports/
│   │   ├── ports.go                     # Listening TCP/UDP sockets + PID/process mapping
│   │   └── ports_test.go
│   ├── services/
│   │   ├── services.go                  # Service name validation & systemctl output parsers
│   │   ├── services_linux.go            # Linux systemd queries via exec.CommandContext (no shell)
│   │   ├── services_other.go            # Non-Linux ErrSystemdUnavailable stub
│   │   └── services_test.go
│   ├── network/
│   │   ├── network.go                   # Interfaces, IPs, IO counters, DNS parsing, --check diagnostics
│   │   ├── gateway_linux.go             # /proc/net/route default gateway reader
│   │   ├── gateway_other.go             # Darwin route fallback / other stub
│   │   └── network_test.go
│   ├── updater/
│   │   ├── updater.go                   # HTTPS + TLS 1.2 + SHA-256 verified atomic binary updater
│   │   └── updater_test.go
│   └── doctor/
│       ├── doctor.go                    # Provider-injected read-only diagnostic engine
│       └── doctor_test.go
├── libs/
│   ├── colorize.go                      # ANSI color helpers, ColorPadRight, NO_COLOR/TTY check, PrintJSON
│   ├── format.go                        # Rune-safe Truncate/PadRight, FormatBytes, FormatDuration
│   ├── sanitize.go                      # ANSI/OSC escape & C0/C1 control-character stripping
│   ├── screen.go                        # Alternate-screen LiveScreen, ANSI clipping, synchronized frame rendering
│   ├── screen_unix.go                   # TIOCGWINSZ terminal size & SIGWINCH resize notifications
│   ├── screen_other.go                  # Non-Unix LiveScreen fallback stubs
│   ├── libs_test.go
│   └── screen_test.go
└── scripts/
    ├── install.sh                       # HTTPS + SHA-256 verified installer (--from-source, --uninstall)
    └── build-release.sh                 # Cross-platform CGO_ENABLED=0 release builder & checksums.txt
```

---

## Core Design Principles

1. **Real Data Only**: No silent sample/seed log creation anywhere. Missing log files return clear errors without touching the filesystem.
2. **Read-Only Operations**: Diagnostics, Docker inspection, systemd queries, and port/network checks never mutate system state, restart units, or modify containers.
3. **Security Hardening**:
   - Terminal escape sequence stripping (`libs.SanitizeTerminal`) on all external inputs (logs, process names, container names, unit descriptions).
   - Sensitive query parameter (`token`, `api_key`, `password`, `secret`, etc.) and bearer token redaction in log outputs.
   - Regular-file validation (`config.ValidateRegularFile`) before opening configs or logs, rejecting directories, FIFOs, and device files.
   - Strict unit-name validation (`services.ValidateServiceName`) and `exec.CommandContext` without shell interpretation.
   - `0600` file mode on generated config files and explicit `--force` required before overwriting existing config.
4. **Accurate Metrics & Graceful Cancellation**:
   - `ProcessSampler` preserves process handles across ticks for interval CPU% calculation and prunes exited PIDs.
   - `FollowLogs` handles log file rotation (`!os.SameFile`) and truncation (`size < offset`), and stops cleanly on context cancellation.
   - Full-screen live commands (`monitor`, `top`, `watch cpu`, `watch docker`, `docker stats --watch`) render via `libs.LiveScreen` in the terminal alternate screen buffer (`\033[?1049h`/`l`) with synchronized in-place frame updates (`\033[?2026h`/`l`), `SIGWINCH` resize redrawing, and clean restoration on `SIGINT`/`SIGTERM`.
5. **Consistent Output & JSON Mode**:
   - `--no-color`, `NO_COLOR`, and non-TTY detection disable ANSI escape codes automatically.
   - `--json` emits clean JSON with the ASCII banner suppressed.
