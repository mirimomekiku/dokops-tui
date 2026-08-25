package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"dok-ops/app"
)

func main() {
	model := app.NewModel()
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Attach PTY subshell stream reader to program
	model.TerminalView.ReadPTY(p)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error starting dok-ops: %v\n", err)
		os.Exit(1)
	}
}