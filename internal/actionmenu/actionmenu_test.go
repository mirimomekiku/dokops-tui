package actionmenu_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"dok-ops/internal/actionmenu"
)

func TestActionMenuOpenClose(t *testing.T) {
	var m actionmenu.Model

	if m.IsOpen() {
		t.Errorf("Expected actionmenu to be closed initially")
	}

	items := []actionmenu.Item{
		{Key: "restart", Title: "Restart Service"},
		{Key: "stop", Title: "Stop Service"},
	}

	m.Open("Service Actions", "nginx.service", items)

	if !m.IsOpen() {
		t.Errorf("Expected actionmenu to be open after Open()")
	}

	// Dismiss with Esc
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	action, closed := m.Update(escMsg)

	if !closed {
		t.Errorf("Expected menu to close on Esc")
	}
	if action != "" {
		t.Errorf("Expected no action returned on Esc, got %s", action)
	}
	if m.IsOpen() {
		t.Errorf("Expected actionmenu to be closed after Esc")
	}
}

func TestActionMenuSelection(t *testing.T) {
	var m actionmenu.Model

	items := []actionmenu.Item{
		{Key: "start", Title: "Start"},
		{Key: "stop", Title: "Stop"},
		{Key: "restart", Title: "Restart"},
	}

	m.Open("Actions", "Sub", items)

	// Move cursor down with 'j'
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	// Press Enter to select second item
	action, closed := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !closed {
		t.Errorf("Expected menu to close on Enter")
	}
	if action != "stop" {
		t.Errorf("Expected action 'stop', got '%s'", action)
	}
}

func TestActionMenuNumberSelection(t *testing.T) {
	var m actionmenu.Model

	items := []actionmenu.Item{
		{Key: "start", Title: "Start"},
		{Key: "stop", Title: "Stop"},
		{Key: "restart", Title: "Restart"},
	}

	m.Open("Actions", "Sub", items)

	// Press '3' to select third item directly
	action, closed := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})

	if !closed {
		t.Errorf("Expected menu to close on numeric key")
	}
	if action != "restart" {
		t.Errorf("Expected action 'restart', got '%s'", action)
	}
}
