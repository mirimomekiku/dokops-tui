package commandpalette_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"dok-ops/internal/commandpalette"
)

func sampleItems() []commandpalette.CommandItem {
	return []commandpalette.CommandItem{
		{
			TabID:       0,
			Title:       "System Monitor",
			Category:    "System",
			Description: "CPU, memory, load, processes",
			Keywords:    []string{"top", "htop", "cpu", "ram", "processes", "ps"},
		},
		{
			TabID:       1,
			Title:       "Docker Containers",
			Category:    "Net & DB",
			Description: "Container statuses, logs, restart, stop",
			Keywords:    []string{"docker", "containers", "ps", "logs", "compose"},
		},
		{
			TabID:       4,
			Title:       "Nginx VHosts",
			Category:    "WebOps",
			Description: "Sites-available, syntax test, reload",
			Keywords:    []string{"nginx", "vhosts", "sites", "web", "conf"},
		},
		{
			TabID:       12,
			Title:       "Database Runner",
			Category:    "Net & DB",
			Description: "PostgreSQL & MySQL queries",
			Keywords:    []string{"db", "database", "sql", "postgres", "mysql"},
		},
	}
}

func TestCommandPaletteOpenClose(t *testing.T) {
	m := commandpalette.New(sampleItems())

	if m.IsOpen() {
		t.Errorf("Expected palette to be closed initially")
	}

	m.Open()
	if !m.IsOpen() {
		t.Errorf("Expected palette to be open after Open()")
	}

	// Esc should close palette
	_, _, closed := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !closed {
		t.Errorf("Expected closed=true on Esc")
	}
	if m.IsOpen() {
		t.Errorf("Expected palette to be closed after Esc")
	}
}

func TestCommandPaletteFuzzySearch(t *testing.T) {
	m := commandpalette.New(sampleItems())
	m.Open()

	// Type 'doc'
	for _, r := range "doc" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Press Enter to select top match (should be Docker Containers)
	tabID, selected, closed := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !selected || !closed {
		t.Errorf("Expected selected=true and closed=true, got selected=%v, closed=%v", selected, closed)
	}

	if tabID != 1 {
		t.Errorf("Expected selected TabID=1 (Docker Containers), got %d", tabID)
	}
}

func TestCommandPaletteKeywordSearch(t *testing.T) {
	m := commandpalette.New(sampleItems())
	m.Open()

	// Type 'sql' which matches Database Runner
	for _, r := range "sql" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	tabID, selected, closed := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !selected || !closed {
		t.Errorf("Expected selected=true and closed=true, got selected=%v, closed=%v", selected, closed)
	}

	if tabID != 12 {
		t.Errorf("Expected selected TabID=12 (Database Runner), got %d", tabID)
	}
}

func TestCommandPaletteNavigation(t *testing.T) {
	m := commandpalette.New(sampleItems())
	m.Open()

	// Move cursor down twice
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m.Update(tea.KeyMsg{Type: tea.KeyDown})

	tabID, selected, closed := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !selected || !closed {
		t.Errorf("Expected selected=true and closed=true")
	}

	// Index 2 is Nginx VHosts (TabID 4)
	if tabID != 4 {
		t.Errorf("Expected TabID 4, got %d", tabID)
	}
}
