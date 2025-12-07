package dashboard

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"devops-dashboard/internal/docker"
)

type CommandHistory struct {
	commands []string
	index    int
}

func (h *CommandHistory) Add(cmd string) {
	if cmd == "" {
		return
	}
	// Don't add duplicates of last command
	if len(h.commands) > 0 && h.commands[len(h.commands)-1] == cmd {
		h.index = len(h.commands)
		return
	}
	h.commands = append(h.commands, cmd)
	h.index = len(h.commands)
}

func (h *CommandHistory) Previous() string {
	if len(h.commands) == 0 {
		return ""
	}
	if h.index > 0 {
		h.index--
	}
	return h.commands[h.index]
}

func (h *CommandHistory) Next() string {
	if len(h.commands) == 0 {
		return ""
	}
	if h.index < len(h.commands)-1 {
		h.index++
		return h.commands[h.index]
	}
	h.index = len(h.commands)
	return ""
}

func ShowInteractiveShell(app *tview.Application, mainView tview.Primitive, containerID string, containers []docker.ContainerInfo) {
	// Get container name
	containerName := containerID[:12]
	for _, c := range containers {
		if c.ID == containerID {
			containerName = c.Name
			break
		}
	}

	history := &CommandHistory{
		commands: []string{},
		index:    0,
	}

	// Output view (terminal-like display)
	outputView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true).
		SetChangedFunc(func() {
			app.Draw()
		})

	outputView.SetBorder(true).
		SetTitle(fmt.Sprintf(" 🖥️  Shell: %s ", containerName)).
		SetBorderPadding(1, 1, 2, 2).
		SetBorderColor(tcell.ColorGreen)

	// Command input
	commandInput := tview.NewInputField().
		SetLabel("$ ").
		SetFieldWidth(0).
		SetFieldBackgroundColor(tcell.ColorDarkSlateGray)

	commandInput.SetBorder(true).
		SetBorderColor(ColorCyan).
		SetBorderPadding(0, 0, 1, 1)

	// Quick commands panel
	quickCommands := tview.NewTextView().
		SetDynamicColors(true)

	quickCommands.SetText(
		"[::b][yellow]Quick Commands:[-:-:-]\n\n" +
			"[cyan]1[-] ls -la\n" +
			"[cyan]2[-] ps aux\n" +
			"[cyan]3[-] df -h\n" +
			"[cyan]4[-] top -bn1\n" +
			"[cyan]5[-] env\n" +
			"[cyan]6[-] cat /etc/os-release\n" +
			"[cyan]7[-] netstat -tulpn\n" +
			"[cyan]8[-] pwd\n" +
			"[cyan]9[-] whoami")

	quickCommands.SetBorder(true).
		SetTitle(" ⚡ Quick ").
		SetBorderColor(tcell.ColorYellow).
		SetBorderPadding(0, 0, 1, 1)

	// Status bar
	statusBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	updateStatus := func(status, color string) {
		statusBar.SetText(fmt.Sprintf("[black:%s] %s [-:-:-]", color, status))
	}
	updateStatus("Ready", "green")

	// Control bar
	controlBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	controlBar.SetText(
		"[black:green] Enter [-:-:-] Execute   " +
			"[black:cyan] ↑/↓ [-:-:-] History   " +
			"[black:yellow] 1-9 [-:-:-] Quick Cmd   " +
			"[black:magenta] Ctrl+C [-:-:-] Clear   " +
			"[black:red] ESC [-:-:-] Back")

	// Layout
	mainContent := tview.NewFlex().
		AddItem(outputView, 0, 3, false).
		AddItem(quickCommands, 25, 0, false)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(statusBar, 1, 0, false).
		AddItem(mainContent, 0, 1, false).
		AddItem(commandInput, 3, 0, true).
		AddItem(controlBar, 1, 0, false)

	// Command execution counter
	commandCount := 0

	// Welcome message
	welcomeMsg := fmt.Sprintf(
		"[::b][green]Interactive Shell Session Started[-:-:-]\n"+
			"[cyan]Container:[-] [white]%s[-]\n"+
			"[cyan]ID:[-] [white]%s[-]\n"+
			"[cyan]Time:[-] [white]%s[-]\n\n"+
			"[yellow]Type commands and press Enter to execute[-]\n"+
			"[gray]Use ↑/↓ for command history[-]\n\n"+
			"────────────────────────────────────\n\n",
		containerName, containerID[:12], time.Now().Format("2006-01-02 15:04:05"))

	outputView.SetText(welcomeMsg)

	// Quick command map
	quickCmdMap := map[rune]string{
		'1': "ls -la",
		'2': "ps aux",
		'3': "df -h",
		'4': "top -bn1",
		'5': "env",
		'6': "cat /etc/os-release",
		'7': "netstat -tulpn",
		'8': "pwd",
		'9': "whoami",
	}

	// Execute command function
	executeCommand := func(cmd string) {
		if cmd == "" {
			return
		}

		cmd = strings.TrimSpace(cmd)
		history.Add(cmd)
		commandCount++

		// Add command to output
		currentText := outputView.GetText(false)
		currentText += fmt.Sprintf("[green]$ %s[-]\n", cmd)
		outputView.SetText(currentText)
		outputView.ScrollToEnd()

		updateStatus("Executing...", "yellow")

		// Execute in background
		go func() {
			output, err := docker.ExecCommand(containerID, cmd)

			app.QueueUpdateDraw(func() {
				currentText := outputView.GetText(false)

				if err != nil {
					currentText += fmt.Sprintf("[red]Error: %s[-]\n\n", err.Error())
					updateStatus("Error", "red")
				} else {
					// Color code output
					if output == "" {
						output = "[gray](no output)[-]"
					}
					currentText += fmt.Sprintf("[white]%s[-]\n", output)
					updateStatus(fmt.Sprintf("✓ Command #%d completed", commandCount), "green")
				}

				currentText += "────────────────────────────────────\n\n"
				outputView.SetText(currentText)
				outputView.ScrollToEnd()
			})
		}()

		commandInput.SetText("")
	}

	// Command input handler
	commandInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp:
			// Previous command in history
			if cmd := history.Previous(); cmd != "" {
				commandInput.SetText(cmd)
			}
			return nil
		case tcell.KeyDown:
			// Next command in history
			commandInput.SetText(history.Next())
			return nil
		case tcell.KeyEscape:
			app.SetRoot(mainView, true)
			return nil
		case tcell.KeyCtrlC:
			// Clear output
			outputView.SetText(welcomeMsg)
			commandCount = 0
			updateStatus("Cleared", "green")
			return nil
		}

		// Quick commands (1-9)
		if cmd, ok := quickCmdMap[event.Rune()]; ok {
			commandInput.SetText(cmd)
			return nil
		}

		return event
	})

	commandInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			executeCommand(commandInput.GetText())
		}
	})

	// Output view key handling
	outputView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			app.SetRoot(mainView, true)
			return nil
		}
		// Focus back to input for typing
		app.SetFocus(commandInput)
		return event
	})

	app.SetRoot(flex, true)
	app.SetFocus(commandInput)
}

// Helper function to show shell options menu
func ShowShellOptionsMenu(app *tview.Application, mainView tview.Primitive, containerID string, containers []docker.ContainerInfo) {
	menu := tview.NewList().ShowSecondaryText(true)

	// Get container name
	containerName := containerID[:12]
	for _, c := range containers {
		if c.ID == containerID {
			containerName = c.Name
			break
		}
	}

	menu.SetBorder(true).
		SetTitle(fmt.Sprintf(" 🖥️  Shell Options: %s ", containerName)).
		SetBorderColor(tcell.ColorGreen).
		SetBorderPadding(1, 1, 2, 2)

	menu.AddItem("⚡ Interactive Shell", "Run commands interactively with history", '1', func() {
		ShowInteractiveShell(app, mainView, containerID, containers)
	})

	menu.AddItem("📝 Quick Command", "Execute a single command and return", '2', func() {
		showQuickCommand(app, mainView, containerID, containerName)
	})

	menu.AddItem("📂 File Browser", "Browse container filesystem", '3', func() {
		showFileBrowser(app, mainView, containerID, containerName)
	})

	menu.AddItem("🔧 System Info", "Get container system information", '4', func() {
		showSystemInfo(app, mainView, containerID, containerName)
	})

	menu.AddItem("❌ Cancel", "Go back", 'q', func() {
		app.SetRoot(mainView, true)
	})

	menu.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			app.SetRoot(mainView, true)
			return nil
		}
		return event
	})

	app.SetRoot(menu, true)
	app.SetFocus(menu)
}

func showQuickCommand(app *tview.Application, mainView tview.Primitive, containerID, containerName string) {
	cmdInput := tview.NewInputField().
		SetLabel("Command: ").
		SetFieldWidth(50)

	form := tview.NewForm().
		AddFormItem(cmdInput).
		AddButton("Execute", func() {
			cmd := cmdInput.GetText()
			if cmd == "" {
				return
			}

			// Show loading
			modal := tview.NewModal().
				SetText(fmt.Sprintf("Executing: %s\n\nPlease wait...", cmd))
			modal.SetBorder(true).SetTitle(" ⏳ Executing ")
			app.SetRoot(modal, false)

			go func() {
				output, err := docker.ExecCommand(containerID, cmd)
				app.QueueUpdateDraw(func() {
					result := output
					if err != nil {
						result = fmt.Sprintf("[red]Error:[-]\n%s", err.Error())
					}
					showMessage(app, mainView, "Command Output", result)
				})
			}()
		}).
		AddButton("Cancel", func() {
			app.SetRoot(mainView, true)
		})

	form.SetBorder(true).
		SetTitle(fmt.Sprintf(" 📝 Quick Command: %s ", containerName)).
		SetBorderColor(ColorCyan)

	app.SetRoot(form, true)
}

func showFileBrowser(app *tview.Application, mainView tview.Primitive, containerID, containerName string) {
	showMessage(app, mainView, "File Browser",
		"File browser coming soon!\n\nFor now, use the shell to browse:\nls -la /path/to/directory")
}

func showSystemInfo(app *tview.Application, mainView tview.Primitive, containerID, containerName string) {
	modal := tview.NewModal().
		SetText("Gathering system information...")
	modal.SetBorder(true).SetTitle(" ⏳ Loading ")
	app.SetRoot(modal, false)

	go func() {
		commands := []string{
			"uname -a",
			"cat /etc/os-release",
			"df -h",
			"free -h",
		}

		info := ""
		for _, cmd := range commands {
			output, _ := docker.ExecCommand(containerID, cmd)
			info += fmt.Sprintf("[yellow]$ %s[-]\n[white]%s[-]\n\n", cmd, output)
		}

		app.QueueUpdateDraw(func() {
			view := tview.NewTextView().
				SetDynamicColors(true).
				SetScrollable(true).
				SetText(info)

			view.SetBorder(true).
				SetTitle(fmt.Sprintf(" 💻 System Info: %s ", containerName)).
				SetBorderColor(ColorGreen)

			view.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
					app.SetRoot(mainView, true)
					return nil
				}
				return event
			})

			app.SetRoot(view, true)
		})
	}()
}
