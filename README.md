# 🚢 DockPulse

> **Real-Time Docker Monitoring TUI**

---

## 🖥️ Terminal UI Preview

Here is how **DockPulse looks inside the terminal** when running:

<img width="1905" height="998" alt="DockPulse Terminal UI" src="https://github.com/user-attachments/assets/eea1de67-0c63-4ad0-9e36-bed19f897e75" />

---

**DockPulse** is a fast, terminal-based dashboard for monitoring and managing Docker containers in real time.

Think of DockPulse as:

> **htop + docker stats + logs + container management — all inside one powerful TUI.**

Built with **Golang** and the **TView TUI library**, DockPulse runs directly in your terminal and provides a smooth, keyboard-driven user experience — no browser, no heavy UI tools.

---

## ✨ Features

### 📊 Live Container Monitoring
- Real-time CPU usage
- Memory usage tracking
- Network I/O statistics
- Block I/O metrics
- Health status indicators

---

### 🐳 Container Management
- Start containers
- Stop containers
- Restart containers
- Delete stopped containers safely
- Inspect container configuration
- Open shell inside containers

---

### 📜 Logs Viewer
- Live streaming of container logs
- ANSI color support
- Auto-scroll logs
- Scroll and pause historical logs

---

### 🔄 Bulk Operations
- Select multiple containers
- Start / Stop / Restart containers in bulk
- Bulk delete stopped containers

---

### 🧠 Compare Mode
- Compare up to **4 containers side-by-side**
- CPU and memory usage bars
- Performance summaries
- Network & I/O metrics comparison

---

### 🌐 Network Monitoring
- Real-time network traffic view
- Container port detection
- Gateway and routing insights
- Ping test & traceroute utilities

---

### 💾 Storage & Volume Viewer
- Detect mounted volumes
- Disk usage reports
- Volume type detection (bind / volume)

---

### 🏥 Health Monitoring
- CPU & memory threshold warnings
- Restart tracking
- OOM event detection
- Simple health scoring

---

### 📤 Data Export
- Export logs
- Export stats
- Export network info
- Volume snapshots
- CSV export for container comparisons

---

---

## ⌨️ Keyboard Controls

| Key | Action |
|------|----------|
| `↑ ↓` | Navigate containers |
| `F5` | Refresh values |
| `l` | View logs |
| `s` | Start / Stop container |
| `r` | Restart container |
| `t` | Open real-time stats |
| `i` | Inspect container |
| `e` | Open shell menu |
| `h` | Health check |
| `SPACE` | Select container |
| `b` | Enable bulk mode |
| `a` | Perform bulk action |
| `x` | Export logs |
| `Backspace` | Go back |
| `q` | Quit application |

---

---

## 🐳 Run DockPulse using Docker (Recommended)

DockPulse is available as a **public Docker image** and is **FREE to use**.  
You do **not** need to install Go — Docker is enough.

### ▶️ Run with a single command

```bash
docker run -it --rm \
  -v /var/run/docker.sock:/var/run/docker.sock \
  gauravsde/dockpulse
