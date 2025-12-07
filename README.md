# 🚢 DockPulse

> **Real-time Docker Monitoring TUI**
>
> <img width="1902" height="958" alt="image" src="https://github.com/user-attachments/assets/471b9908-62ac-4aec-8825-455ba06f82c4" />


DockPulse is a powerful **terminal-based dashboard** for monitoring Docker containers in real time.  
Think of it as:

> **htop + ctop + docker logs + metrics + network + health — all inside one blazing-fast TUI.**

Built with **Golang + TView**, DockPulse provides a smooth, interactive experience directly inside your terminal — no browser, no UI setup, no bloat.

---

## ✨ Features

### 📊 Live Metrics
- CPU & memory usage tracking
- Real-time ASCII graphs
- PIDs, Block I/O & Network I/O stats
- Color-coded health indicators

### 🐳 Container Management
- Start / Stop / Restart containers
- Bulk operations
- Inspect configs
- Shell access
- Delete safely

### 📜 Streaming Logs
- True live logs
- ANSI color support
- Toggle **auto-scroll**
- Scroll & pause historical logs

### 🔍 Comparison Mode
- Compare up to **4 containers** side-by-side
- CPU & memory bar charts
- Performance summaries
- Network & I/O metrics

### 🌐 Network Monitor
- Port mapping detection
- Gateway routing
- Active connections
- Ping tests & traceroute

### 💾 Volume & Storage Analysis
- Mounted volume discovery
- Disk utilization reports
- Storage type (bind / volume)

### 🏥 Health Monitoring
- CPU/memory threshold alerts
- Restart counters
- OOM detection
- Automatic health scoring

### 📤 Export Tools
- Logs
- Stats
- Network info
- Volume snapshots
- Full comparison CSV exports

---

---

## 🖥️ Preview (Terminal UI)
┌───────────────────────────────────────────────────────────────┐
│ 🐳 DockPulse │
├──────────────── Containers ────────────────┬────── Metrics ──┤
│ 🟢 api-service 0.12% │ CPU ▄▅▆▇█▆▃▁ │
│ 🟡 postgres-db 42.31% │ MEM ▂▃▄▅▆▇▆▅ │
│ 🔴 redis-cache Exited │ │
│ │ Network ↓ ↑ │
├──────────────── Logs ────────────────────────┴────────────────┤
│ 2025/12/07 21:02:39 API started │
│ 2025/12/07 21:02:40 Connected to database │
│ ... │
└───────────────────────────────────────────────────────────────┘
