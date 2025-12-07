package dashboard

import (
	"fmt"
	"io"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"devops-dashboard/internal/docker"
)

func showLogs(app *tview.Application, mainView tview.Primitive, containerID string, containers []docker.ContainerInfo) {
	containerName := containerID[:12]
	for _, c := range containers {
		if c.ID == containerID {
			containerName = c.Name
			break
		}
	}

	logView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(false).
		SetChangedFunc(func() { app.Draw() })

	logView.SetBorder(true).
		SetTitle(fmt.Sprintf(" 📜 Logs: %s ", containerName)).
		SetBorderPadding(1, 1, 2, 2).
		SetBorderColor(tcell.ColorTeal)

	statusBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	statusBar.SetText("[black:yellow] ⏳ Loading logs... [-:-:-]")

	bottomBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	bottomBar.SetText(
		"[white][[yellow]Backspace/ESC[white]] Back   " +
			"[white][[cyan]↑/↓[white]] Scroll   " +
			"[white][[blue]PgUp/PgDn[white]] Page   " +
			"[white][[magenta]Home/End[white]] Top/Bottom   " +
			"[white][[lime]q[white]] Quit")

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(statusBar, 1, 0, false).
		AddItem(logView, 0, 1, true).
		AddItem(bottomBar, 1, 0, false)

	go func() {
		reader, err := docker.StreamLogs(containerID)
		if err != nil {
			app.QueueUpdateDraw(func() {
				statusBar.SetText("[black:red] ❌ Error loading logs [-:-:-]")
				logView.SetText(fmt.Sprintf("[red]Failed to load logs:[-]\n[yellow]%s[-]", err.Error()))
			})
			return
		}
		defer reader.Close()

		app.QueueUpdateDraw(func() {
			statusBar.SetText("[black:lime] ● Live Logs Streaming... [-:-:-]")
		})

		io.Copy(logView, reader)
	}()

	logView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'b', 'B', 'q', 'Q':
			app.SetRoot(mainView, true)
			return nil
		case 'g', 'G':
			logView.ScrollToBeginning()
			return nil
		}

		switch event.Key() {
		case tcell.KeyEscape, tcell.KeyBackspace, tcell.KeyBackspace2:
			app.SetRoot(mainView, true)
			return nil
		case tcell.KeyHome:
			logView.ScrollToBeginning()
			return nil
		case tcell.KeyEnd:
			logView.ScrollToEnd()
			return nil
		}

		return event
	})

	app.SetRoot(flex, true)
	app.SetFocus(logView)
}
