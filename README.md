# Tazx

> Tazama your server's pulse instantly.

**Tazx** is a lightweight, read-only Go CLI utility for inspecting Linux server vitals, diagnosing common operational problems, analyzing web server access logs, checking listening ports, inspecting systemd services, checking network settings, and viewing Docker container status and statistics.

---

## 1. What Tazx Is

Tazx is a single-binary command-line tool built for everyday server administration and troubleshooting. Instead of chaining together `top`, `free`, `df`, `ss`, `systemctl`, `ip`, `tail`, and `docker stats`, Tazx surfaces the most useful operational information in a consistent, readable terminal format (with optional `--json` output for scripting).

Tazx is **not** a monitoring platform or observability stack. It requires no daemon, no database, no agent, and no web dashboard.

## 2. Why It Exists

When a server slows down or starts throwing errors, developers and sysadmins usually want answers fast:

- Is CPU, RAM, swap, or disk saturated? Which process is responsible?
- Which service is listening on port `8080`?
- Did a systemd unit fail?
- Are web logs showing 5xx spikes, brute-force attempts, or vulnerability scanners?
- Is a Docker container crashing or saturating memory?

Tazx gathers those answers locally in seconds using native OS APIs and read-only checks.

## 3. Who It Is For

- Backend and full-stack developers debugging staging or production servers over SSH.
- System administrators and DevOps engineers who want quick, repeatable health checks.
- Teams managing single VPS hosts, bare-metal Linux servers, or Docker hosts without wanting to remember dozens of flags across separate utilities.

## 4. Features

- **Instant Server Overview (`status`)**: CPU, RAM, root disk status, load average, access log summary, and active alerts.
- **Read-Only Diagnostics (`doctor`)**: Rule-based health and security checks across CPU, RAM, swap, disk partitions, systemd failed units, Docker containers, DNS config, config file permissions, and web access logs — with severity, impact, details, and what to investigate.
- **CPU, Memory, Storage & Sessions (`cpu`, `memory`, `storage`, `users`)**:
  - Per-core CPU usage and load averages.
  - Physical RAM (including Linux available memory), swap usage, and top consumers.
  - Multi-partition disk table plus opt-in bounded large-directory discovery (`storage --large`).
  - Active interactive login sessions.
- **Process & Live Monitoring (`top`, `monitor`, `watch`)**: Non-destructive, flicker-free alternate-screen rendering (`SIGWINCH` resize-aware, atomic in-place frame updates, original terminal scrollback restored cleanly on `Ctrl+C`) and interval-based process CPU/RAM sampling.
- **Web Log Analysis & Tailing (`logs`)**: Parses Nginx/Apache Combined and Common log formats, detects 4xx/5xx spikes, suspicious scanner paths, failed auth attempts, redacts sensitive query tokens/secrets, strips terminal escape sequences, and supports `--follow` with log rotation/truncation handling.
- **Listening Ports (`ports`)**: Shows port, protocol (`tcp`/`tcp6`/`udp`/`udp6`), bind address, PID, and process name with `--port` and `--protocol` filters.
- **Systemd Services (`services`)**: Inspects Linux systemd units, unit-file enablement, and `--failed` services.
- **Network Overview (`network`)**: Shows interfaces, IPv4/IPv6 addresses, RX/TX byte counters, default gateway, DNS resolvers, and optional non-intrusive `--check` (DNS + single TCP/443 dial).
- **Docker Inspection (`docker ps`, `docker stats`)**: Read-only container listing and live CPU %, memory usage/limit/%, network I/O, block I/O, and PID counts.
- **Scripting Support**: Global `--no-color` and `--json` flags (for snapshot commands).

---

## 5. Installation

Supported targets (verified clean builds with `CGO_ENABLED=0`):
- **Linux**: `amd64`, `arm64`
- **macOS (Darwin)**: `amd64`, `arm64`

### Option A: Installation Script (Recommended)

The installer downloads the release binary over HTTPS, verifies its SHA-256 checksum against `checksums.txt`, and installs to `$HOME/.local/bin` by default (no `sudo` required):

```bash
curl -fsSL https://raw.githubusercontent.com/Hezekiah-Elisha/tazx/main/scripts/install.sh -o install-tazx.sh
less install-tazx.sh
bash install-tazx.sh
```

Options supported by `scripts/install.sh`:
- `--version v1.1.0`: Install a specific release tag.
- `--install-dir /usr/local/bin`: Override install location (requires write permission or `sudo` for system directories).
- `--from-source`: Build and install the latest version from a local repository clone.
- `--update`: Update an existing Tazx installation in-place.
- `--uninstall`: Remove the installed binary.

### Option B: Precompiled Binary (Manual)

Download the binary for your OS/architecture and `checksums.txt` from the GitHub Releases page, verify the checksum, and install it:

```bash
# Example for Linux amd64
curl -fsSLO https://github.com/Hezekiah-Elisha/tazx/releases/latest/download/tazx-linux-amd64
curl -fsSLO https://github.com/Hezekiah-Elisha/tazx/releases/latest/download/checksums.txt

sha256sum -c checksums.txt --ignore-missing
chmod +x tazx-linux-amd64
mkdir -p ~/.local/bin
mv tazx-linux-amd64 ~/.local/bin/tazx
```

If `~/.local/bin` is not on your `PATH`, add `export PATH="$HOME/.local/bin:$PATH"` to your `~/.bashrc` or `~/.zshrc`.

### Option C: Install with Go

Requires Go 1.25+:

```bash
go install github.com/Hezekiah-Elisha/tazx@latest
```

### Option D: Build from Source

```bash
git clone https://github.com/Hezekiah-Elisha/tazx.git
cd tazx
go build -trimpath -o tazx .
./tazx version
```

---

## 6. Quick Start

`tazx init` is **optional**. Tazx works immediately with sensible defaults without creating any files:

1. Verify installation:
   ```bash
   tazx version
   ```
2. Check server vitals and alerts:
   ```bash
   tazx status
   ```
3. Run read-only diagnostics:
   ```bash
   tazx doctor
   ```
4. Inspect listening ports and services:
   ```bash
   tazx ports
   tazx services --failed
   ```
5. Explore all commands and flags:
   ```bash
   tazx --help
   ```

---

## 7. Available Commands

Global flags (available on every command):
- `--config <file>`: Load custom YAML configuration (default `$HOME/.tazx.yaml`, falling back to `./.tazx.yaml`).
- `-p, --path <file>`: Override the access log file path.
- `--no-color`: Disable ANSI color output (also respects the `NO_COLOR` environment variable).
- `--json`: Output valid machine-readable JSON without banners or ANSI decorations (supported on all snapshot commands; rejected on continuous streaming commands).

| Command | Description |
| :--- | :--- |
| `tazx status` | Combined server vitals, access log summary, and alerts |
| `tazx doctor` | Read-only diagnostic engine with explanations and next steps |
| `tazx cpu` | Overall CPU %, per-core usage, load averages, hardware model, top processes |
| `tazx memory` | RAM total/used/available/free, swap metrics, top memory processes |
| `tazx storage` | Mounted filesystems table and optional `--large` directory usage scan |
| `tazx users` | Active logged-in user sessions |
| `tazx ports` | Listening TCP/UDP sockets, addresses, PIDs, and process names |
| `tazx services [name]` | Systemd unit status and failed services (Linux) |
| `tazx network` | Interfaces, IPs, RX/TX traffic, default gateway, DNS, `--check` |
| `tazx logs [service\|path]` | View, filter, analyze routes, or tail web access logs |
| `tazx top` | Live process monitor sorted by CPU or RAM (`--once` supported) |
| `tazx monitor` | Real-time dashboard combining vitals, logs, and alerts (`--once` supported) |
| `tazx watch <cpu\|logs\|docker>` | Continuous single-resource watcher |
| `tazx docker [ps\|stats]` | List Docker containers or view CPU/Mem/Net/Block I/O stats (`--watch` supported) |
| `tazx init` | Optionally create `~/.tazx.yaml` with `0600` permissions |
| `tazx config` | Print active configuration and source |
| `tazx update` | Check for updates (`--check`) or self-update via verified SHA-256 release |
| `tazx version` | Print Tazx version, Go runtime, and `OS/Arch` |

---

## 8. Command Examples

```bash
# Inspect listening ports, or filter a specific port/protocol
tazx ports
tazx ports --port 8080 --protocol tcp

# Inspect systemd services on Linux
tazx services --failed
tazx services nginx

# Check network interfaces, gateway, DNS, and verify outbound DNS + HTTPS reachability
tazx network
tazx network --check --host example.com

# Check partitions or locate large directories under /var
tazx storage
tazx storage --large --dir /var --depth 2 --limit 10

# Analyze Nginx, Apache, or custom access logs (read-only; never creates missing files)
tazx logs nginx --errors --lines 50
tazx logs apache --routes
tazx logs -p /var/log/nginx/access.log --follow

# Inspect Docker containers and live resource stats
tazx docker ps -a
tazx docker stats
tazx docker stats --watch
tazx docker stats api-gateway

# Check for updates or update an existing installation
tazx update --check
tazx update
bash scripts/install.sh --update --from-source

# Machine-readable JSON output for automation
tazx status --json
tazx doctor --json
tazx docker stats --json
```

---

## 9. Configuration

Configuration is **optional**. If you want custom alert thresholds or a custom default log path, run:

```bash
tazx init
```

This creates `$HOME/.tazx.yaml` with `0600` (owner-only) permissions and refuses to overwrite an existing file unless `--force` is specified:

```yaml
log_path: /var/log/nginx/access.log
refresh_rate: 2
cpu_threshold: 80
memory_threshold: 85
disk_threshold: 85
```

Validation bounds enforced on load:
- `refresh_rate`: `1` to `3600` seconds
- `cpu_threshold`, `memory_threshold`, `disk_threshold`: `> 0` and `<= 100`
- `log_path`: non-empty path without null bytes

---

## 10. Permissions & Security Considerations

Tazx is designed to be **read-only** and work without `root` privileges for most operations:

- **No root required by default**: CPU, memory, swap, disk partitions, network overview, `doctor`, and user-owned logs run as a regular user.
- **Process names on `tazx ports`**: Linux restricts socket-to-PID mapping for sockets owned by other users. When unprivileged, Tazx still reports the listening port, protocol, and bind address and notes that some PIDs are hidden.
- **Log file access**: If `/var/log/nginx/access.log` denies read access, grant read-only access via group membership (e.g., `adm`) or POSIX ACLs (`setfacl -m u:<user>:r`) instead of running monitoring tools as root.
- **Docker socket**: Reading container lists/stats requires access to the Docker daemon (`DOCKER_HOST` or `/var/run/docker.sock`). Only add trusted users to the `docker` group because Docker socket access grants root-equivalent host privileges. Tazx only executes read-only Docker API calls (`ContainerList`, `ContainerStats`) and never creates, execs, restarts, or removes containers.
- **Terminal & Log Safety**: Log routes and raw lines are stripped of ANSI escape/control characters before display, and sensitive query parameters (`token`, `api_key`, `password`, `secret`, `auth`, etc.) and inline bearer tokens are redacted.
- **Path Safety**: Log and config readers validate regular files and refuse directories, device nodes, or FIFOs; `tazx init` refuses to overwrite symlinks.

---

## 11. Supported Operating Systems

| OS / Arch | Build Status | Notes |
| :--- | :--- | :--- |
| Linux `amd64` | Built & Tested | Full feature support including `services` (systemd) |
| Linux `arm64` | Built | Full feature support including `services` (systemd) |
| macOS `amd64` | Cross-compiled cleanly | `services` reports that systemd is unavailable (Linux-only) |
| macOS `arm64` | Cross-compiled cleanly | `services` reports that systemd is unavailable (Linux-only) |

*Note: Runtime integration tests in this repository were executed on Linux `amd64`. Cross-compiled macOS binaries were verified to build cleanly with `CGO_ENABLED=0`, while runtime verification on macOS hardware requires testing on a macOS host.*

---

## 12. Troubleshooting

- **"Log file not found"**
  Tazx does not generate fake sample logs. Verify where your web server writes logs and pass `--path /path/to/access.log` or set `log_path` in `~/.tazx.yaml`.
- **"Permission denied accessing log file"**
  Grant your user read access via the `adm` group or `setfacl` rather than running Tazx permanently as `root`.
- **"systemd is not available on this host"**
  `tazx services` requires a Linux host booted with systemd (`/run/systemd/system` and `systemctl`). Containers without systemd and macOS return this message cleanly.
- **"Docker daemon not reachable" or "permission denied"**
  Confirm the Docker service is running (`systemctl status docker`) and that your user is permitted to access the Docker socket or `DOCKER_HOST`.
- **"Some PIDs or process names are hidden" in `tazx ports`**
  The kernel hides PID/process ownership for sockets owned by other users unless inspected with elevated privileges.

---

## 13. Uninstallation

If installed via `scripts/install.sh` (or into `~/.local/bin`):

```bash
rm -f ~/.local/bin/tazx
```

Or run:

```bash
bash scripts/install.sh --uninstall
```

If you installed to `/usr/local/bin`:

```bash
sudo rm -f /usr/local/bin/tazx
```

To also remove the optional configuration file:

```bash
rm -f ~/.tazx.yaml
```

---

## 14. Contributing

Contributions, bug reports, and pull requests are welcome.

Before submitting a PR:
1. Run `gofmt -l .`
2. Run `go vet ./...`
3. Run `go test -race ./...`
4. Keep changes aligned with Tazx's philosophy: fast, read-only, minimal dependencies, clear output.

---

## 15. License

MIT License — see [LICENSE](LICENSE).
