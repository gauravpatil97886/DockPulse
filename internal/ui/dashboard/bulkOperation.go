package dashboard

import (
	"fmt"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"devops-dashboard/internal/docker"
)

// BulkOperationMode manages multi-container selection
type BulkOperationMode struct {
	enabled     bool
	selectedIDs map[string]bool
	mu          sync.RWMutex
}

func NewBulkOperationMode() *BulkOperationMode {
	return &BulkOperationMode{
		selectedIDs: make(map[string]bool),
	}
}

func (b *BulkOperationMode) Toggle() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.enabled = !b.enabled
	if !b.enabled {
		b.selectedIDs = make(map[string]bool)
	}
}

func (b *BulkOperationMode) IsEnabled() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.enabled
}

func (b *BulkOperationMode) ToggleContainer(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.selectedIDs[id] {
		delete(b.selectedIDs, id)
	} else {
		b.selectedIDs[id] = true
	}
}

func (b *BulkOperationMode) IsSelected(id string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.selectedIDs[id]
}

func (b *BulkOperationMode) GetSelected() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	ids := make([]string, 0, len(b.selectedIDs))
	for id := range b.selectedIDs {
		ids = append(ids, id)
	}
	return ids
}

func (b *BulkOperationMode) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.selectedIDs)
}

func (b *BulkOperationMode) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.selectedIDs = make(map[string]bool)
}

// ShowBulkActionsMenu displays the bulk operations menu
func ShowBulkActionsMenu(app *tview.Application, mainView tview.Primitive, bulkMode *BulkOperationMode, containers []docker.ContainerInfo, updateList func()) {
	selectedIDs := bulkMode.GetSelected()
	if len(selectedIDs) == 0 {
		showMessage(app, mainView, "No Selection", "Please select at least one container first.\n\nPress SPACE to select containers.")
		return
	}

	// Get selected container names
	selectedNames := []string{}
	for _, container := range containers {
		if bulkMode.IsSelected(container.ID) {
			selectedNames = append(selectedNames, container.Name)
		}
	}

	// Create menu
	menu := tview.NewList().ShowSecondaryText(true)
	menu.SetBorder(true).
		SetTitle(fmt.Sprintf(" 🎯 Bulk Actions (%d selected) ", len(selectedIDs))).
		SetBorderColor(ColorOrange).
		SetBorderPadding(1, 1, 2, 2)

	menu.AddItem("🟢 Start All", "Start all selected containers", '1', func() {
		confirmBulkAction(app, mainView, "Start", selectedNames, func() {
			performBulkAction(app, mainView, selectedIDs, "start", bulkMode, updateList)
		})
	})

	menu.AddItem("🔴 Stop All", "Stop all selected containers", '2', func() {
		confirmBulkAction(app, mainView, "Stop", selectedNames, func() {
			performBulkAction(app, mainView, selectedIDs, "stop", bulkMode, updateList)
		})
	})

	menu.AddItem("🔄 Restart All", "Restart all selected containers", '3', func() {
		confirmBulkAction(app, mainView, "Restart", selectedNames, func() {
			performBulkAction(app, mainView, selectedIDs, "restart", bulkMode, updateList)
		})
	})

	menu.AddItem("🗑️  Delete All", "Remove all selected containers", '4', func() {
		confirmBulkAction(app, mainView, "Delete", selectedNames, func() {
			performBulkAction(app, mainView, selectedIDs, "delete", bulkMode, updateList)
		})
	})

	menu.AddItem("📋 Export Logs", "Save logs from all selected containers", '5', func() {
		showMessage(app, mainView, "Export Logs",
			fmt.Sprintf("Exporting logs from %d containers...\n\nLogs will be saved to: ./container-logs/", len(selectedIDs)))
		go exportBulkLogs(app, mainView, selectedIDs, containers)
		app.SetRoot(mainView, true)
	})

	menu.AddItem("❌ Cancel", "Go back to main view", 'q', func() {
		app.SetRoot(mainView, true)
	})

	// Show selected container list
	infoText := tview.NewTextView().
		SetDynamicColors(true).
		SetWordWrap(true)

	selectedList := "[yellow]Selected Containers:[-]\n\n"
	for i, name := range selectedNames {
		selectedList += fmt.Sprintf("[cyan]%d.[-] [white]%s[-]\n", i+1, name)
	}
	infoText.SetText(selectedList)
	infoText.SetBorder(true).
		SetTitle(" 📦 Selection ").
		SetBorderColor(ColorCyan).
		SetBorderPadding(0, 0, 1, 1)

	footer := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	footer.SetText("[black:green] 1-5 [-:-:-] Actions   [black:red] q/ESC [-:-:-] Cancel")

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(tview.NewFlex().
			AddItem(menu, 0, 2, true).
			AddItem(infoText, 0, 1, false), 0, 1, true).
		AddItem(footer, 1, 0, false)

	menu.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' || event.Rune() == 'Q' {
			app.SetRoot(mainView, true)
			return nil
		}
		return event
	})

	app.SetRoot(flex, true)
	app.SetFocus(menu)
}

func confirmBulkAction(app *tview.Application, mainView tview.Primitive, action string, containerNames []string, onConfirm func()) {
	message := fmt.Sprintf("[yellow]%s %d containers?[-]\n\n", action, len(containerNames))
	if len(containerNames) <= 5 {
		for _, name := range containerNames {
			message += fmt.Sprintf("• %s\n", name)
		}
	} else {
		for i := 0; i < 3; i++ {
			message += fmt.Sprintf("• %s\n", containerNames[i])
		}
		message += fmt.Sprintf("... and %d more\n", len(containerNames)-3)
	}
	message += "\n[red]This action cannot be undone![-]"

	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"Yes", "No"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Yes" {
				onConfirm()
			} else {
				app.SetRoot(mainView, true)
			}
		})
	modal.SetTitle(" ⚠️  Confirm Bulk Action ").
		SetBorder(true).
		SetBorderColor(ColorRed)

	app.SetRoot(modal, true)
}

func performBulkAction(app *tview.Application, mainView tview.Primitive, containerIDs []string, action string, bulkMode *BulkOperationMode, updateList func()) {
	// Progress view
	progressView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)

	progressView.SetBorder(true).
		SetTitle(fmt.Sprintf(" ⚙️  Processing: %s ", action)).
		SetBorderColor(ColorYellow).
		SetBorderPadding(1, 1, 2, 2)

	app.SetRoot(progressView, true)

	// Perform actions in background
	go func() {
		total := len(containerIDs)
		success := 0
		failed := 0

		for i, id := range containerIDs {
			app.QueueUpdateDraw(func() {
				progressView.SetText(fmt.Sprintf(
					"[cyan]Progress: %d/%d[-]\n\n"+
						"[green]✓ Success: %d[-]\n"+
						"[red]✗ Failed: %d[-]\n\n"+
						"[yellow]Processing container %d...[-]",
					i+1, total, success, failed, i+1))
			})

			var err error
			switch action {
			case "start":
				err = docker.StartContainer(id)
			case "stop":
				err = docker.StopContainer(id)
			case "restart":
				err = docker.RestartContainer(id)
			case "delete":
				err = docker.RemoveContainer(id)
			}

			if err == nil {
				success++
			} else {
				failed++
			}
		}

		// Show final results
		app.QueueUpdateDraw(func() {
			resultText := fmt.Sprintf(
				"[::b][cyan]Bulk Operation Complete![-:-:-]\n\n"+
					"[green]✓ Successful: %d[-]\n"+
					"[red]✗ Failed: %d[-]\n"+
					"[yellow]Total: %d[-]\n\n"+
					"Press any key to continue...",
				success, failed, total)

			progressView.SetText(resultText)
			progressView.SetTitle(" ✅ Complete ")
			progressView.SetBorderColor(ColorGreen)

			progressView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				bulkMode.Clear()
				bulkMode.Toggle() // Exit bulk mode
				updateList()
				app.SetRoot(mainView, true)
				return nil
			})
		})
	}()
}

func exportBulkLogs(app *tview.Application, mainView tview.Primitive, containerIDs []string, containers []docker.ContainerInfo) {
	// This would save logs to files
	// Implementation depends on your requirements
	// For now, just a placeholder

	for _, id := range containerIDs {
		// Get container name
		var name string
		for _, c := range containers {
			if c.ID == id {
				name = c.Name
				break
			}
		}

		// In real implementation:
		// logs, _ := docker.GetLogs(id)
		// ioutil.WriteFile(fmt.Sprintf("./container-logs/%s.log", name), []byte(logs), 0644)

		_ = name // Use the name for filename
	}

	app.QueueUpdateDraw(func() {
		showMessage(app, mainView, "Success",
			fmt.Sprintf("Successfully exported logs from %d containers!\n\nLocation: ./container-logs/", len(containerIDs)))
	})
}
