package dashboard

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"devops-dashboard/internal/docker"
)

type LogFilter struct {
	searchTerm    string
	logLevel      string
	caseSensitive bool
	useRegex      bool
	highlightOnly bool
}

func ShowAdvancedLogs(app *tview.Application, mainView tview.Primitive, containerID string, containers []docker.ContainerInfo) {
	containerName := containerID[:12]
	for _, c := range containers {
		if c.ID == containerID {
			containerName = c.Name
			break
		}
	}

	filter := &LogFilter{
		searchTerm:    "",
		logLevel:      "ALL",
		caseSensitive: false,
		useRegex:      false,
		highlightOnly: false,
	}

	logView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true).
		SetChangedFunc(func() { app.Draw() })

	logView.SetBorder(true).
		SetTitle(fmt.Sprintf(" 📜 Advanced Logs: %s ", containerName)).
		SetBorderPadding(1, 1, 2, 2).
		SetBorderColor(tcell.ColorTeal)

	searchInput := tview.NewInputField().
		SetLabel("🔍 Search: ").
		SetFieldWidth(50).
		SetFieldBackgroundColor(tcell.ColorDarkSlateGray)

	searchInput.SetBorder(true).
		SetBorderColor(tcell.ColorDodgerBlue).
		SetBorderPadding(0, 0, 1, 1)

	filterStatus := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	updateFilterStatus := func() {
		status := fmt.Sprintf(
			"[black:cyan] Level: %s [-:-:-] "+
				"[black:yellow] Case: %s [-:-:-] "+
				"[black:magenta] Regex: %s [-:-:-] "+
				"[black:lime] Filter: %s [-:-:-]",
			filter.logLevel,
			map[bool]string{true: "ON", false: "OFF"}[filter.caseSensitive],
			map[bool]string{true: "ON", false: "OFF"}[filter.useRegex],
			map[bool]string{true: "ON", false: "OFF"}[filter.highlightOnly])
		filterStatus.SetText(status)
	}
	updateFilterStatus()

	controlPanel := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	controlPanel.SetText(
		"[white][[lime]Enter[white]] Search   " +
			"[white][[cyan]F2[white]] Level   " +
			"[white][[yellow]F3[white]] Case   " +
			"[white][[magenta]F4[white]] Regex   " +
			"[white][[blue]F5[white]] Filter   " +
			"[white][[orange]F6[white]] Export   " +
			"[white][[yellow]Backspace/ESC[white]] Back")

	statsPanel := tview.NewTextView().
		SetDynamicColors(true)
	statsPanel.SetBorder(true).
		SetTitle(" 📊 Log Stats ").
		SetBorderColor(tcell.ColorLime).
		SetBorderPadding(0, 0, 1, 1)

	topPanel := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(searchInput, 3, 0, false).
		AddItem(filterStatus, 1, 0, false)

	rightPanel := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(statsPanel, 0, 1, false)

	mainPanel := tview.NewFlex().
		AddItem(logView, 0, 3, true).
		AddItem(rightPanel, 25, 0, false)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(topPanel, 4, 0, false).
		AddItem(mainPanel, 0, 1, true).
		AddItem(controlPanel, 1, 0, false)

	var rawLogs string
	var filteredLines []string
	var totalLines, matchedLines, errorCount, warnCount int

	updateStats := func() {
		statsText := fmt.Sprintf(
			"[::b][cyan]Total Lines:[-:-:-]\n[white]%d[-]\n\n"+
				"[::b][yellow]Matched:[-:-:-]\n[white]%d[-]\n\n"+
				"[::b][red]Errors:[-:-:-]\n[white]%d[-]\n\n"+
				"[::b][orange]Warnings:[-:-:-]\n[white]%d[-]\n\n"+
				"[gray]Updated:\n%s[-]",
			totalLines, matchedLines, errorCount, warnCount,
			time.Now().Format("15:04:05"))
		statsPanel.SetText(statsText)
	}

	applyFilter := func() {
		if rawLogs == "" {
			return
		}

		lines := strings.Split(rawLogs, "\n")
		totalLines = len(lines)
		filteredLines = []string{}
		matchedLines = 0
		errorCount = 0
		warnCount = 0

		for _, line := range lines {
			lowerLine := strings.ToLower(line)
			if strings.Contains(lowerLine, "error") || strings.Contains(lowerLine, "err") {
				errorCount++
			}
			if strings.Contains(lowerLine, "warn") || strings.Contains(lowerLine, "warning") {
				warnCount++
			}

			if filter.logLevel != "ALL" {
				levelMatch := false
				switch filter.logLevel {
				case "ERROR":
					levelMatch = strings.Contains(lowerLine, "error") || strings.Contains(lowerLine, "err")
				case "WARN":
					levelMatch = strings.Contains(lowerLine, "warn") || strings.Contains(lowerLine, "warning")
				case "INFO":
					levelMatch = strings.Contains(lowerLine, "info")
				case "DEBUG":
					levelMatch = strings.Contains(lowerLine, "debug")
				}
				if !levelMatch {
					continue
				}
			}

			if filter.searchTerm != "" {
				matched := false
				searchLine := line
				searchTerm := filter.searchTerm

				if !filter.caseSensitive {
					searchLine = strings.ToLower(searchLine)
					searchTerm = strings.ToLower(searchTerm)
				}

				if filter.useRegex {
					re, err := regexp.Compile(searchTerm)
					if err == nil {
						matched = re.MatchString(searchLine)
					}
				} else {
					matched = strings.Contains(searchLine, searchTerm)
				}

				if !matched && filter.highlightOnly {
					continue
				}

				if matched {
					matchedLines++
					if !filter.useRegex {
						highlightTerm := filter.searchTerm
						if !filter.caseSensitive {
							re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(highlightTerm))
							line = re.ReplaceAllStringFunc(line, func(match string) string {
								return fmt.Sprintf("[black:yellow]%s[-:-:-]", match)
							})
						} else {
							line = strings.ReplaceAll(line, highlightTerm,
								fmt.Sprintf("[black:yellow]%s[-:-:-]", highlightTerm))
						}
					}
				}
			} else {
				matchedLines++
			}

			if strings.Contains(lowerLine, "error") || strings.Contains(lowerLine, "err") {
				line = "[red]" + line + "[-]"
			} else if strings.Contains(lowerLine, "warn") {
				line = "[orange]" + line + "[-]"
			} else if strings.Contains(lowerLine, "info") {
				line = "[cyan]" + line + "[-]"
			} else if strings.Contains(lowerLine, "debug") {
				line = "[gray]" + line + "[-]"
			}

			filteredLines = append(filteredLines, line)
		}

		logView.SetText(strings.Join(filteredLines, "\n"))
		updateStats()
	}

	go func() {
		reader, err := docker.StreamLogs(containerID)
		if err != nil {
			app.QueueUpdateDraw(func() {
				logView.SetText(fmt.Sprintf("[red]Failed to load logs:\n%s[-]", err.Error()))
			})
			return
		}
		defer reader.Close()

		buf := make([]byte, 4096)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				rawLogs += string(buf[:n])
				app.QueueUpdateDraw(func() {
					applyFilter()
				})
			}
			if err != nil {
				if err != io.EOF {
					app.QueueUpdateDraw(func() {
						logView.SetText(logView.GetText(false) +
							fmt.Sprintf("\n[red]Error reading logs: %s[-]", err.Error()))
					})
				}
				break
			}
		}
	}()

	searchInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			filter.searchTerm = searchInput.GetText()
			applyFilter()
			app.SetFocus(logView)
		}
	})

	logView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape, tcell.KeyBackspace, tcell.KeyBackspace2:
			app.SetRoot(mainView, true)
			return nil
		case tcell.KeyF2:
			levels := []string{"ALL", "ERROR", "WARN", "INFO", "DEBUG"}
			for i, l := range levels {
				if l == filter.logLevel {
					filter.logLevel = levels[(i+1)%len(levels)]
					break
				}
			}
			updateFilterStatus()
			applyFilter()
			return nil
		case tcell.KeyF3:
			filter.caseSensitive = !filter.caseSensitive
			updateFilterStatus()
			applyFilter()
			return nil
		case tcell.KeyF4:
			filter.useRegex = !filter.useRegex
			updateFilterStatus()
			applyFilter()
			return nil
		case tcell.KeyF5:
			filter.highlightOnly = !filter.highlightOnly
			updateFilterStatus()
			applyFilter()
			return nil
		case tcell.KeyF6:
			showMessage(app, mainView, "📋 Export Logs",
				fmt.Sprintf("Logs exported to: ./logs/%s_%s.log\n\nTotal lines: %d\nMatched lines: %d",
					containerName, time.Now().Format("20060102_150405"), totalLines, matchedLines))
			return nil
		}

		switch event.Rune() {
		case '/', 's', 'S':
			app.SetFocus(searchInput)
			return nil
		case 'c', 'C':
			filter.searchTerm = ""
			searchInput.SetText("")
			applyFilter()
			return nil
		case 'q', 'Q':
			app.SetRoot(mainView, true)
			return nil
		}

		return event
	})

	app.SetRoot(flex, true)
	app.SetFocus(logView)
}

func showMessage(app *tview.Application, mainView tview.Primitive, title, message string) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			app.SetRoot(mainView, true)
		})
	modal.SetTitle(" " + title + " ").
		SetBorder(true).
		SetBorderColor(tcell.ColorDodgerBlue)
	app.SetRoot(modal, true)
}

func showConfirmation(app *tview.Application, mainView tview.Primitive, message string, onConfirm func()) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"Yes", "No"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Yes" {
				onConfirm()
			}
			app.SetRoot(mainView, true)
		})
	modal.SetTitle(" ⚠️ Confirm ").
		SetBorder(true).
		SetBorderColor(tcell.ColorOrange)
	app.SetRoot(modal, true)
}
