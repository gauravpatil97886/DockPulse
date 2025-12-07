package dashboard

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"devops-dashboard/internal/docker"
)

type StatsViewer struct {
	cpuHistory    []float64
	memHistory    []float64
	netRxHistory  []float64
	netTxHistory  []float64
	maxDataPoints int
}

func NewStatsViewer() *StatsViewer {
	return &StatsViewer{
		cpuHistory:    make([]float64, 0, 60),
		memHistory:    make([]float64, 0, 60),
		netRxHistory:  make([]float64, 0, 60),
		netTxHistory:  make([]float64, 0, 60),
		maxDataPoints: 60,
	}
}

func (sv *StatsViewer) AddCPU(value float64) {
	sv.cpuHistory = append(sv.cpuHistory, value)
	if len(sv.cpuHistory) > sv.maxDataPoints {
		sv.cpuHistory = sv.cpuHistory[1:]
	}
}

func (sv *StatsViewer) AddMem(value float64) {
	sv.memHistory = append(sv.memHistory, value)
	if len(sv.memHistory) > sv.maxDataPoints {
		sv.memHistory = sv.memHistory[1:]
	}
}

func (sv *StatsViewer) GetCPUBar() string {
	if len(sv.cpuHistory) == 0 {
		return DrawGraph(0, 50)
	}
	latest := sv.cpuHistory[len(sv.cpuHistory)-1]
	return DrawGraph(latest, 50)
}

func (sv *StatsViewer) GetMemBar() string {
	if len(sv.memHistory) == 0 {
		return DrawGraph(0, 50)
	}
	latest := sv.memHistory[len(sv.memHistory)-1]
	return DrawGraph(latest, 50)
}

func (sv *StatsViewer) GetCPUGraph() string {
	return sv.createSparkline(sv.cpuHistory, 60)
}

func (sv *StatsViewer) GetMemGraph() string {
	return sv.createSparkline(sv.memHistory, 60)
}

func (sv *StatsViewer) createSparkline(data []float64, width int) string {
	if len(data) == 0 {
		return strings.Repeat("▁", width)
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

	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	result := ""

	padding := width - len(data)
	if padding > 0 {
		result += strings.Repeat("▁", padding)
	}

	for _, v := range data {
		normalized := v / max
		index := int(normalized * float64(len(blocks)-1))
		if index < 0 {
			index = 0
		}
		if index >= len(blocks) {
			index = len(blocks) - 1
		}
		result += string(blocks[index])
	}

	if len(result) > width {
		result = result[len(result)-width:]
	}

	return result
}

func (sv *StatsViewer) createLineGraph(data []float64, height, width int) string {
	if len(data) == 0 || height == 0 || width == 0 {
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

	graph := make([][]rune, height)
	for i := range graph {
		graph[i] = make([]rune, width)
		for j := range graph[i] {
			graph[i][j] = ' '
		}
	}

	dataPerCol := float64(len(data)) / float64(width)

	for col := 0; col < width; col++ {
		dataIndex := int(float64(col) * dataPerCol)
		if dataIndex >= len(data) {
			dataIndex = len(data) - 1
		}

		value := data[dataIndex]
		normalizedValue := value / max
		row := height - 1 - int(normalizedValue*float64(height-1))

		if row < 0 {
			row = 0
		}
		if row >= height {
			row = height - 1
		}

		graph[row][col] = '█'

		for r := row + 1; r < height; r++ {
			graph[r][col] = '│'
		}
	}

	var result strings.Builder
	for i, row := range graph {
		if i > 0 {
			result.WriteRune('\n')
		}
		result.WriteString(string(row))
	}

	return result.String()
}

func showEnhancedStats(app *tview.Application, mainView tview.Primitive, containerID, containerName string) {
	statsViewer := NewStatsViewer()

	statsView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false)

	statsView.SetBorder(true).
		SetTitle(fmt.Sprintf(" 📊 Real-time Statistics: %s ", containerName)).
		SetBorderPadding(1, 1, 2, 2).
		SetBorderColor(tcell.ColorLime)

	summaryView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false)
	summaryView.SetBorder(true).
		SetTitle(" 📈 Summary ").
		SetBorderColor(tcell.ColorDarkCyan).
		SetBorderPadding(0, 0, 1, 1)

	graphView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false)
	graphView.SetBorder(true).
		SetTitle(" 📉 Historical Graph ").
		SetBorderColor(tcell.ColorLightCyan).
		SetBorderPadding(0, 0, 1, 1)

	controlBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("[white][[yellow]Backspace/ESC[white]] Back   [[cyan]r[white]] Reset   [[yellow]p[white]] Pause   [[lime]q[white]] Quit")

	rightPanel := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(summaryView, 0, 1, false).
		AddItem(graphView, 12, 0, false)

	mainPanel := tview.NewFlex().
		AddItem(statsView, 0, 2, true).
		AddItem(rightPanel, 40, 0, false)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(mainPanel, 0, 1, true).
		AddItem(controlBar, 1, 0, false)

	ctx, cancel := context.WithCancel(context.Background())
	paused := false
	startTime := time.Now()

	var avgCPU, avgMem, maxCPU, maxMem float64
	sampleCount := 0

	updateStats := func() {
		stats, err := docker.GetStats(containerID)
		if err != nil {
			app.QueueUpdateDraw(func() {
				statsView.SetText(fmt.Sprintf("[red]Error: %s[-]", err.Error()))
			})
			return
		}

		var cpuVal, memVal float64
		fmt.Sscanf(stats.CPUPerc, "%f%%", &cpuVal)
		fmt.Sscanf(stats.MemPerc, "%f%%", &memVal)

		statsViewer.AddCPU(cpuVal)
		statsViewer.AddMem(memVal)

		sampleCount++
		avgCPU = ((avgCPU * float64(sampleCount-1)) + cpuVal) / float64(sampleCount)
		avgMem = ((avgMem * float64(sampleCount-1)) + memVal) / float64(sampleCount)

		if cpuVal > maxCPU {
			maxCPU = cpuVal
		}
		if memVal > maxMem {
			maxMem = memVal
		}

		cpuBar := statsViewer.GetCPUBar()
		memBar := statsViewer.GetMemBar()
		cpuGraph := statsViewer.GetCPUGraph()
		memGraph := statsViewer.GetMemGraph()

		cpuColor := "lime"
		if cpuVal > 80 {
			cpuColor = "red"
		} else if cpuVal > 50 {
			cpuColor = "yellow"
		}

		memColor := "lime"
		if memVal > 80 {
			memColor = "red"
		} else if memVal > 50 {
			memColor = "yellow"
		}

		mainDisplay := fmt.Sprintf(
			"[::b][cyan]CPU Usage:[-:-:-]\n"+
				"[white]Current: [%s]%.2f%%[-][-]\n"+
				"[%s]%s[-]\n"+
				"[cyan]%s[-]\n\n"+
				"[::b][magenta]Memory Usage:[-:-:-]\n"+
				"[white]Current: [%s]%.2f%%[-] (%s)[-]\n"+
				"[%s]%s[-]\n"+
				"[magenta]%s[-]\n\n"+
				"[::b][lime]Network I/O:[-:-:-]\n[white]%s[-]\n\n"+
				"[::b][yellow]Block I/O:[-:-:-]\n[white]%s[-]\n\n"+
				"[::b][dodgerblue]Process Info:[-:-:-]\n[white]PIDs: %s[-]",
			cpuColor, cpuVal, cpuColor, cpuBar, cpuGraph,
			memColor, memVal, stats.MemUsage, memColor, memBar, memGraph,
			stats.NetIO,
			stats.BlockIO,
			stats.PIDs)

		summaryDisplay := fmt.Sprintf(
			"[::b][yellow]Statistics Summary[-:-:-]\n\n"+
				"[cyan]Runtime:[-]\n[white]%s[-]\n\n"+
				"[cyan]Samples:[-]\n[white]%d[-]\n\n"+
				"[cyan]CPU Avg:[-]\n[white]%.2f%%[-]\n\n"+
				"[cyan]CPU Max:[-]\n[%s]%.2f%%[-]\n\n"+
				"[cyan]Mem Avg:[-]\n[white]%.2f%%[-]\n\n"+
				"[cyan]Mem Max:[-]\n[%s]%.2f%%[-]\n\n"+
				"[gray]Updated:\n%s[-]",
			time.Since(startTime).Round(time.Second),
			sampleCount,
			avgCPU,
			func() string {
				if maxCPU > 80 {
					return "red"
				} else if maxCPU > 50 {
					return "yellow"
				} else {
					return "lime"
				}
			}(), maxCPU,
			avgMem,
			func() string {
				if maxMem > 80 {
					return "red"
				} else if maxMem > 50 {
					return "yellow"
				} else {
					return "lime"
				}
			}(), maxMem,
			time.Now().Format("15:04:05"))

		lineGraph := statsViewer.createLineGraph(statsViewer.cpuHistory, 10, 38)
		graphDisplay := fmt.Sprintf(
			"[cyan]CPU Trend (60s):[-]\n"+
				"[lime]%s[-]",
			lineGraph)

		app.QueueUpdateDraw(func() {
			statsView.SetText(mainDisplay)
			summaryView.SetText(summaryDisplay)
			graphView.SetText(graphDisplay)
		})
	}

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		updateStats()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !paused {
					updateStats()
				}
			}
		}
	}()

	statsView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'q', 'Q':
			cancel()
			app.SetRoot(mainView, true)
			return nil
		case 'r', 'R':
			avgCPU, avgMem, maxCPU, maxMem = 0, 0, 0, 0
			sampleCount = 0
			startTime = time.Now()
			statsViewer = NewStatsViewer()
			return nil
		case 'p', 'P':
			paused = !paused
			if paused {
				controlBar.SetText("[white][[red]⏸ PAUSED[white]]   [[yellow]p[white]] Resume   [[yellow]Backspace[white]] Back")
			} else {
				controlBar.SetText("[white][[yellow]Backspace/ESC[white]] Back   [[cyan]r[white]] Reset   [[yellow]p[white]] Pause   [[lime]q[white]] Quit")
			}
			return nil
		}

		if event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2 {
			cancel()
			app.SetRoot(mainView, true)
			return nil
		}

		return event
	})

	app.SetRoot(flex, true)
	app.SetFocus(statsView)
}

func showEnhancedInspect(app *tview.Application, mainView tview.Primitive, containerID, containerName string) {
	inspectView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true)

	inspectView.SetBorder(true).
		SetTitle(fmt.Sprintf(" 🔍 Inspect: %s ", containerName)).
		SetBorderPadding(1, 1, 2, 2).
		SetBorderColor(tcell.ColorDarkMagenta)

	buttonBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("[white][[yellow]Backspace/ESC[white]] Back   [[cyan]↑/↓[white]] Scroll   [[lime]q[white]] Quit")

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(inspectView, 0, 1, true).
		AddItem(buttonBar, 1, 0, false)

	inspectView.SetText("[yellow]⏳ Loading container details...[-]")

	go func() {
		details, err := docker.InspectContainer(containerID)
		app.QueueUpdateDraw(func() {
			if err != nil {
				inspectView.SetText(fmt.Sprintf("[red]Error:[-] %s", err.Error()))
			} else {
				inspectView.SetText(details)
			}
		})
	}()

	inspectView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' || event.Rune() == 'Q' || event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2 {
			app.SetRoot(mainView, true)
			return nil
		}
		return event
	})

	app.SetRoot(flex, true)
	app.SetFocus(inspectView)
}
