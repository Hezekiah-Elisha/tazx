<img src="ascii-tazx.png" style="width: 50%"/>

---

# TAZX

> Tazama your server’s pulse instantly

- Detects spikes, errors, and suspicious activity

- 📜 **Log analysis**
  - Identify errors, unusual routes, and traffic patterns

- 🐳 **Docker support**
  - View running containers and resource usage

- 🔍 **Developer-friendly insights**
  - Not just metrics — _explanations_

for more understanding I used [this](https://leapcell.medium.com/gopsutil-powerful-system-stats-for-go-developers-2a1941c40822) article here

---

## 🖥️ Installation

### From source

```bash
git clone https://github.com/Hezekiah-Elisha/tazx.git
cd tazx
go build -o tazx
```

### Run

```bash
./tazx status
```

### Usage

#### Check system status

```bash
tazx status
```

Example output

```bash
=== SYSTEM ===
CPU: 22%   RAM: 1.9GB   Disk: OK

=== APP ===
Requests/min: 40
Errors: 1

=== ALERTS ===
⚠️ High traffic on /login
```

#### Live monitoring

```bash
tazx top
```

#### Analyze Logs

By default, Tazx analyzes Nginx logs (`/var/log/nginx/access.log`). You can also target other services or custom log files:

```bash
tazx logs                  # Views Nginx logs by default
tazx logs nginx            # Explicitly target Nginx logs
tazx logs apache           # Target Apache logs (/var/log/apache2/access.log)
tazx logs ./access.log     # Target custom log file
tazx logs --errors         # Filter error status codes (>= 400)
tazx logs --routes         # Route traffic and bot scan analysis
tazx logs --follow         # Stream live logs (like tail -f)
```

#### Diagnose issues

```bash
tazx doctor
```

Example:

```bash
❌ High CPU usage (91%)
→ Likely cause: spike in API traffic

⚠️ 200 requests to /wp-admin
→ Possible bot scanning attempt

⚠️ Multiple failed logins detected
→ Possible brute-force attack
```

#### Docker insights

```bash
tazx docker ps
tazx docker stats
```

### Configuration

Initialize config

```bash
tazx init
```

Example config:

```yaml
log_path: /var/log/nginx/access.log
refresh_rate: 2
```

## Why tazx

Most tools show you numbers.

Tazx tells you what those numbers mean.

Instead of:

```bash
cpu: 90%
```

You get:

```bash
CPU: 90% (High)
→ Likely caused by increased API traffic
```

## 🎯 Use Cases

- Debugging high server load
- Detecting bot traffic (e.g. /wp-admin scans)
- Monitoring Docker containers
- Understanding production issues quickly
- Lightweight alternative to complex monitoring stacks

## 🛠️ Built With

- Go 🐹
- Cobra (CLI framework)
- gopsutil (system metrics)
- Docker SDK

## 📌 Roadmap

- Advanced anomaly detection
- Request tracing
- Alert notifications
- Remote agent support
- Plugin system

## 🤝 Contributing

Contributions are welcome!

If you have ideas, suggestions, or bug fixes:

- Fork the repo
- Create a new branch
- Submit a pull request

> I am pretty new to open source so, Please, I accept contributions. Raise issues, I will respond as fast as I can.

## 📣 Inspiration

Tazx was built to solve a real problem:

> understanding strange server behavior quickly without complex tools.

## ⭐ Support

If you find this useful, give it a star ⭐
It helps others discover the project!

## 📄 License

MIT License
