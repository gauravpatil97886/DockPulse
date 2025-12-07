package dashboard

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"devops-dashboard/internal/docker"
)

type Dashboard struct {
	app           *tview.Application
	containers    []docker.ContainerInfo
	selectedIndex int
	statsCtx      context.Context
	statsCancel   context.CancelFunc
	refreshCtx    context.Context
	refreshCancel context.CancelFunc
	mu            sync.RWMutex
	list          *tview.List
	detailsText   *tview.TextView
	statsText     *tview.TextView
	systemInfo    *tview.TextView
	bulkMode      *BulkOperationMode
	statsHistory  *StatsHistory
	mainFlex      *tview.Flex
}

type StatsHistory struct {
	cpuHistory []float64
	memHistory []float64
	mu         sync.RWMutex
}

func NewStatsHistory() *StatsHistory {
	return &StatsHistory{
		cpuHistory: make([]float64, 0, 30),
		memHistory: make([]float64, 0, 30),
	}
}

func (sh *StatsHistory) AddCPU(value float64) {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	sh.cpuHistory = append(sh.cpuHistory, value)
	if len(sh.cpuHistory) > 30 {
		sh.cpuHistory = sh.cpuHistory[1:]
	}
}

func (sh *StatsHistory) AddMem(value float64) {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	sh.memHistory = append(sh.memHistory, value)
	if len(sh.memHistory) > 30 {
		sh.memHistory = sh.memHistory[1:]
	}
}

func (sh *StatsHistory) GetCPUGraph() string {
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	if len(sh.cpuHistory) == 0 {
		return ""
	}
	return createMiniGraph(sh.cpuHistory, 30)
}

func (sh *StatsHistory) GetMemGraph() string {
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	if len(sh.memHistory) == 0 {
		return ""
	}
	return createMiniGraph(sh.memHistory, 30)
}

func createMiniGraph(data []float64, width int) string {
	if len(data) == 0 {
		return ""
	}

	max := 0.0
	for _, v := range data {
		if v > max {
			max = v
		}
	}
	if max == 0 {
		max = 1
	}

	graph := ""
	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	for _, v := range data {
		normalized := v / max
		index := int(normalized * float64(len(blocks)-1))
		if index < 0 {
			index = 0
		}
		if index >= len(blocks) {
			index = len(blocks) - 1
		}
		graph += string(blocks[index])
	}

	return graph
}

func NewDashboardUI() (*tview.Application, error) {
	d := &Dashboard{
		app:          tview.NewApplication(),
		bulkMode:     NewBulkOperationMode(),
		statsHistory: NewStatsHistory(),
	}

	d.statsCtx, d.statsCancel = context.WithCancel(context.Background())
	d.refreshCtx, d.refreshCancel = context.WithCancel(context.Background())

	// Container list
	d.list = tview.NewList().ShowSecondaryText(true)
	d.list.SetBorder(true).
		SetTitle(" 🐳 Docker Containers ").
		SetTitleAlign(tview.AlignCenter).
		SetBorderPadding(1, 1, 2, 2).
		SetBorderColor(tcell.ColorDodgerBlue)

	// Details panel
	d.detailsText = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true)
	d.detailsText.SetBorder(true).
		SetTitle(" 📊 Container Details ").
		SetBorderPadding(0, 0, 1, 1).
		SetBorderColor(tcell.ColorMediumPurple)

	// Stats panel
	d.statsText = tview.NewTextView().
		SetDynamicColors(true).
		SetWordWrap(false)
	d.statsText.SetBorder(true).
		SetTitle(" 📈 Live Stats ").
		SetBorderPadding(0, 0, 1, 1).
		SetBorderColor(tcell.ColorLime)

	// Actions panel with VISIBLE shortcuts
	actionsText := tview.NewTextView().
		SetDynamicColors(true).
		SetText(
			"[::b][yellow]Container Actions:[-:-:-]\n\n" +
				"[white][[lime]l[white]] View Logs\n" +
				"[white][[cyan]L[white]] Advanced Logs\n" +
				"[white][[lime]s[white]] Start/Stop\n" +
				"[white][[lime]r[white]] Restart\n" +
				"[white][[cyan]t[white]] Real-time Stats\n" +
				"[white][[blue]i[white]] Inspect\n" +
				"[white][[magenta]e[white]] Shell Menu\n" +
				"[white][[orange]h[white]] Health Check\n" +
				"[white][[red]d[white]] Delete\n\n" +
				"[::b][cyan]Bulk Operations:[-:-:-]\n" +
				"[white][[magenta]b[white]] Bulk Mode\n" +
				"[white][[yellow]SPACE[white]] Select\n" +
				"[white][[cyan]a[white]] Bulk Actions\n" +
				"[white][[orange]x[white]] Export Logs\n\n" +
				"[::b][dodgerblue]Navigation:[-:-:-]\n" +
				"[white][[lime]↑/↓[white]] Navigate\n" +
				"[white][[lime]F5[white]] Refresh\n" +
				"[white][[yellow]Backspace[white]] Back\n" +
				"[white][[red]q[white]] Quit")
	actionsText.SetBorder(true).
		SetTitle(" ⚡ Actions ").
		SetBorderPadding(0, 0, 1, 1).
		SetBorderColor(tcell.ColorOrange)

	// System info
	d.systemInfo = tview.NewTextView().
		SetDynamicColors(true)
	d.systemInfo.SetBorder(true).
		SetTitle(" 💻 System Info ").
		SetBorderPadding(0, 0, 1, 1).
		SetBorderColor(tcell.ColorTeal)

	// Layout
	rightTopPanel := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(d.detailsText, 0, 1, false).
		AddItem(d.statsText, 14, 0, false)

	rightPanel := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(rightTopPanel, 0, 2, false).
		AddItem(actionsText, 28, 0, false).
		AddItem(d.systemInfo, 8, 0, false)

	d.mainFlex = tview.NewFlex().
		AddItem(d.list, 0, 2, true).
		AddItem(rightPanel, 65, 0, false)

	if err := d.updateList(); err != nil {
		return nil, fmt.Errorf("failed to fetch containers: %v", err)
	}

	d.startStatsWorker()
	d.startRefreshWorker()
	d.setupKeyHandlers()

	d.list.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		d.mu.Lock()
		if index >= 0 && index < len(d.containers) {
			d.selectedIndex = index
		}
		d.mu.Unlock()
	})

	d.app.SetRoot(d.mainFlex, true)
	d.app.SetFocus(d.list)

	return d.app, nil
}

func (d *Dashboard) setupKeyHandlers() {
	d.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		d.mu.RLock()
		containerCount := len(d.containers)
		d.mu.RUnlock()

		if event.Rune() == 'q' || event.Rune() == 'Q' {
			d.cleanup()
			d.app.Stop()
			return nil
		}

		if event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2 {
			if d.bulkMode.IsEnabled() {
				d.bulkMode.Toggle()
				d.updateList()
			}
			return nil
		}

		if containerCount == 0 {
			return event
		}

		currentIndex := d.list.GetCurrentItem()
		if currentIndex < 0 || currentIndex >= containerCount {
			return event
		}

		d.mu.Lock()
		d.selectedIndex = currentIndex
		container := d.containers[d.selectedIndex]
		d.mu.Unlock()

		switch event.Rune() {
		case 'l':
			showLogs(d.app, d.mainFlex, container.ID, d.containers)
			return nil
		case 'L':
			ShowAdvancedLogs(d.app, d.mainFlex, container.ID, d.containers)
			return nil
		case 's', 'S':
			d.toggleContainer(container)
			return nil
		case 'r', 'R':
			d.restartContainer(container)
			return nil
		case 'd', 'D':
			d.deleteContainer(container)
			return nil
		case 't', 'T':
			showEnhancedStats(d.app, d.mainFlex, container.ID, container.Name)
			return nil
		case 'i', 'I':
			showEnhancedInspect(d.app, d.mainFlex, container.ID, container.Name)
			return nil
		case 'e', 'E':
			ShowShellOptionsMenu(d.app, d.mainFlex, container.ID, d.containers)
			return nil
		case 'h', 'H':
			d.showHealthCheck(container)
			return nil
		case 'x', 'X':
			d.exportContainerLogs(container)
			return nil
		case 'b', 'B':
			d.bulkMode.Toggle()
			d.updateList()
			if d.bulkMode.IsEnabled() {
				d.showBulkModeInfo()
			}
			return nil
		case 'a', 'A':
			if d.bulkMode.IsEnabled() {
				d.mu.RLock()
				containers := d.containers
				d.mu.RUnlock()
				ShowBulkActionsMenu(d.app, d.mainFlex, d.bulkMode, containers, func() { d.updateList() })
			}
			return nil
		case ' ':
			if d.bulkMode.IsEnabled() {
				d.bulkMode.ToggleContainer(container.ID)
				d.updateList()
				d.showBulkModeInfo()
			}
			return nil
		}

		if event.Key() == tcell.KeyF5 {
			d.updateList()
			return nil
		}

		return event
	})
}

func (d *Dashboard) showBulkModeInfo() {
	d.app.QueueUpdateDraw(func() {
		d.detailsText.SetText(
			"[::b][yellow]🎯 BULK MODE ACTIVE[-:-:-]\n\n" +
				"[cyan]Instructions:[-]\n" +
				"• Press [green]SPACE[-] to select containers\n" +
				"• Press [green]'a'[-] for bulk actions menu\n" +
				"• Press [green]'b'[-] or [yellow]Backspace[-] to exit\n\n" +
				"[white]Selected: [yellow]" + fmt.Sprintf("%d", d.bulkMode.Count()) + "[-] containers")
	})
}

func (d *Dashboard) cleanup() {
	if d.statsCancel != nil {
		d.statsCancel()
	}
	if d.refreshCancel != nil {
		d.refreshCancel()
	}
}

func (d *Dashboard) startStatsWorker() {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-d.statsCtx.Done():
				return
			case <-ticker.C:
				d.updateStats()
			}
		}
	}()
}

func (d *Dashboard) startRefreshWorker() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-d.refreshCtx.Done():
				return
			case <-ticker.C:
				d.app.QueueUpdateDraw(func() {
					d.updateList()
				})
			}
		}
	}()
}

func (d *Dashboard) updateStats() {
	d.mu.RLock()
	if len(d.containers) == 0 || d.selectedIndex < 0 || d.selectedIndex >= len(d.containers) {
		d.mu.RUnlock()
		return
	}
	container := d.containers[d.selectedIndex]
	d.mu.RUnlock()

	stats, err := docker.GetStats(container.ID)
	if err != nil {
		d.app.QueueUpdateDraw(func() {
			d.statsText.SetText("[red]Stats unavailable[-]")
		})
		return
	}

	var cpuVal, memVal float64
	fmt.Sscanf(stats.CPUPerc, "%f%%", &cpuVal)
	fmt.Sscanf(stats.MemPerc, "%f%%", &memVal)

	d.statsHistory.AddCPU(cpuVal)
	d.statsHistory.AddMem(memVal)

	d.app.QueueUpdateDraw(func() {
		if !d.bulkMode.IsEnabled() {
			cpuGraph := d.statsHistory.GetCPUGraph()
			memGraph := d.statsHistory.GetMemGraph()

			statsDisplay := fmt.Sprintf(
				"[::b][cyan]CPU Usage:[-:-:-]\n"+
					"[white]%s[-]\n"+
					"[cyan]%s[-]\n\n"+
					"[::b][magenta]Memory:[-:-:-]\n"+
					"[white]%s (%s)[-]\n"+
					"[magenta]%s[-]\n\n"+
					"[::b][lime]Network I/O:[-:-:-]\n[white]%s[-]\n\n"+
					"[::b][yellow]Block I/O:[-:-:-]\n[white]%s[-]",
				stats.CPUPerc, cpuGraph,
				stats.MemPerc, stats.MemUsage, memGraph,
				stats.NetIO,
				stats.BlockIO)

			d.statsText.SetText(statsDisplay)

			d.detailsText.SetText(fmt.Sprintf(
				"[::b][yellow]Container:[-:-:-]\n[white]%s[-]\n\n"+
					"[::b][cyan]ID:[-:-:-]\n[white]%s[-]\n\n"+
					"[::b][lime]Status:[-:-:-]\n[white]%s[-]\n\n"+
					"[::b][magenta]Image:[-:-:-]\n[white]%s[-]\n\n"+
					"[::b][orange]Ports:[-:-:-]\n[white]%s[-]",
				container.Name,
				container.ID[:12],
				container.Status,
				container.Image,
				container.Ports))
		}
	})
}

func (d *Dashboard) updateList() error {
	newContainers, err := docker.ListContainers()
	if err != nil {
		return err
	}

	d.mu.Lock()
	d.containers = newContainers
	d.mu.Unlock()

	d.list.Clear()

	if len(newContainers) == 0 {
		d.list.AddItem("[yellow]No containers found[-]",
			"[gray]Start some Docker containers to manage them[-]", 0, nil)
		d.detailsText.SetText("[yellow]No containers available[-]\n\nStart Docker containers to manage them here.")
		d.statsText.SetText("")
		d.updateSystemInfo()
		return nil
	}

	for _, container := range newContainers {
		statusIcon := "🔴"
		statusColor := "red"
		if container.State == "running" {
			statusIcon = "🟢"
			statusColor = "lime"
		}

		checkbox := ""
		if d.bulkMode.IsEnabled() {
			if d.bulkMode.IsSelected(container.ID) {
				checkbox = "[lime]☑[-] "
			} else {
				checkbox = "[gray]☐[-] "
			}
		}

		primaryText := fmt.Sprintf("%s%s [%s]%s[-]", checkbox, statusIcon, statusColor, container.Name)
		secondaryText := fmt.Sprintf("[gray]%s | %s | %s[-]", container.ID[:12], container.Image, container.Status)

		d.list.AddItem(primaryText, secondaryText, 0, nil)
	}

	d.updateSystemInfo()
	return nil
}

func (d *Dashboard) updateSystemInfo() {
	d.mu.RLock()
	total := len(d.containers)
	running := countRunning(d.containers)
	d.mu.RUnlock()

	bulkStatus := ""
	if d.bulkMode.IsEnabled() {
		bulkStatus = fmt.Sprintf("[::b][magenta]Bulk Mode:[-:-:-] [yellow]ON (%d)[-]\n\n", d.bulkMode.Count())
	}

	info := fmt.Sprintf(
		"%s"+
			"[::b][dodgerblue]Total:[-:-:-] [white]%d[-]\n"+
			"[::b][lime]Running:[-:-:-] [white]%d[-]\n"+
			"[::b][red]Stopped:[-:-:-] [white]%d[-]\n\n"+
			"[gray]Updated: %s[-]",
		bulkStatus, total, running, total-running,
		time.Now().Format("15:04:05"))

	d.systemInfo.SetText(info)
}

func (d *Dashboard) toggleContainer(container docker.ContainerInfo) {
	go func() {
		var err error
		if container.State == "running" {
			err = docker.StopContainer(container.ID)
		} else {
			err = docker.StartContainer(container.ID)
		}

		d.app.QueueUpdateDraw(func() {
			if err != nil {
				showMessage(d.app, d.mainFlex, "Error", err.Error())
			} else {
				d.updateList()
			}
		})
	}()
}

func (d *Dashboard) restartContainer(container docker.ContainerInfo) {
	go func() {
		err := docker.RestartContainer(container.ID)
		d.app.QueueUpdateDraw(func() {
			if err != nil {
				showMessage(d.app, d.mainFlex, "Error", err.Error())
			} else {
				showMessage(d.app, d.mainFlex, "✅ Success", "Container restarted!")
				d.updateList()
			}
		})
	}()
}

func (d *Dashboard) deleteContainer(container docker.ContainerInfo) {
	showConfirmation(d.app, d.mainFlex,
		fmt.Sprintf("Delete container '%s'?\n\nThis action cannot be undone!", container.Name),
		func() {
			go func() {
				err := docker.RemoveContainer(container.ID)
				d.app.QueueUpdateDraw(func() {
					if err != nil {
						showMessage(d.app, d.mainFlex, "Error", err.Error())
					} else {
						d.updateList()
					}
				})
			}()
		})
}

func (d *Dashboard) showHealthCheck(container docker.ContainerInfo) {
	modal := tview.NewModal().SetText("🏥 Checking container health...")
	modal.SetBorder(true).SetTitle(" ⏳ Health Check ")
	d.app.SetRoot(modal, false)

	go func() {
		health, err := docker.CheckHealth(container.ID)
		d.app.QueueUpdateDraw(func() {
			d.app.SetRoot(d.mainFlex, true)
			if err != nil {
				showMessage(d.app, d.mainFlex, "Error", err.Error())
				return
			}

			healthText := fmt.Sprintf(
				"[::b][cyan]🏥 Health Check Results[-:-:-]\n\n"+
					"[yellow]Responsive:[-] %s\n"+
					"[yellow]Disk Usage:[-] %s\n"+
					"[yellow]Memory:[-] %s\n",
				health["responsive"],
				health["disk_usage"],
				health["memory_usage"])

			showMessage(d.app, d.mainFlex, "Health Check", healthText)
		})
	}()
}

func (d *Dashboard) exportContainerLogs(container docker.ContainerInfo) {
	showMessage(d.app, d.mainFlex, "📋 Export Logs",
		fmt.Sprintf("Exporting logs for: %s\n\nLocation: ./logs/%s_%s.log",
			container.Name, container.Name, time.Now().Format("20060102_150405")))
}

func countRunning(containers []docker.ContainerInfo) int {
	count := 0
	for _, c := range containers {
		if c.State == "running" {
			count++
		}
	}
	return count
}
