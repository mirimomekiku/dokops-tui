package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWorkspacesCoversAllTabs(t *testing.T) {
	tabCount := 20 // 20 tabs across 4 active workspaces (Ports removed from System; Tools disabled)
	seen := make(map[Tab]bool)

	for _, ws := range Workspaces {
		for _, tab := range ws.Tabs {
			if seen[tab] {
				t.Errorf("Duplicate tab %v in workspace %s", tab, ws.Name)
			}
			seen[tab] = true
		}
	}

	if len(seen) != tabCount {
		t.Errorf("Expected %d unique tabs in workspaces, got %d", tabCount, len(seen))
	}
}

func TestWorkspaceNavigation(t *testing.T) {
	m := NewModel()

	if m.ActiveWorkspace != WorkspaceSystem {
		t.Errorf("Expected initial workspace to be WorkspaceSystem, got %v", m.ActiveWorkspace)
	}
	if m.ActiveTab != TabDashboard {
		t.Errorf("Expected initial tab to be TabDashboard, got %v", m.ActiveTab)
	}

	// Switch to Workspace WebOps
	m.SetWorkspace(WorkspaceWebOps)
	if m.ActiveWorkspace != WorkspaceWebOps {
		t.Errorf("Expected active workspace to be WorkspaceWebOps, got %v", m.ActiveWorkspace)
	}
	if m.ActiveTab != TabNginx {
		t.Errorf("Expected active tab in WebOps to be TabNginx, got %v", m.ActiveTab)
	}

	// Cycle sub-tabs
	m.NextSubTab()
	if m.ActiveTab != TabAutoNginx {
		t.Errorf("Expected active tab after NextSubTab to be TabAutoNginx, got %v", m.ActiveTab)
	}

	// Switch away and back: should remember last active tab
	m.SetWorkspace(WorkspaceSystem)
	if m.ActiveTab != TabMonitor {
		t.Errorf("Expected TabMonitor when switching back to System, got %v", m.ActiveTab)
	}
	m.SetWorkspace(WorkspaceWebOps)
	if m.ActiveTab != TabAutoNginx {
		t.Errorf("Expected remembered tab TabAutoNginx when switching back to WebOps, got %v", m.ActiveTab)
	}

	// Direct tab set
	m.SetTab(TabDisk)
	if m.ActiveWorkspace != WorkspaceSystem {
		t.Errorf("Expected TabDisk to activate WorkspaceSystem, got %v", m.ActiveWorkspace)
	}
	if m.ActiveTab != TabDisk {
		t.Errorf("Expected active tab to be TabDisk, got %v", m.ActiveTab)
	}
}

func TestCommandPaletteIntegration(t *testing.T) {
	m := NewModel()

	// Initially palette closed
	if m.CommandPalette.IsOpen() {
		t.Errorf("Expected CommandPalette to be closed initially")
	}

	// Trigger Ctrl+K keypress
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = updatedModel.(Model)

	if !m.CommandPalette.IsOpen() {
		t.Errorf("Expected CommandPalette to open on Ctrl+K")
	}

	// Type "nginx" into the palette
	for _, r := range "nginx" {
		updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updatedModel.(Model)
	}

	// Press Enter to select
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updatedModel.(Model)

	if m.CommandPalette.IsOpen() {
		t.Errorf("Expected CommandPalette to close after selection")
	}

	if m.ActiveTab != TabNginx {
		t.Errorf("Expected ActiveTab to be TabNginx, got %v", m.ActiveTab)
	}
	if m.ActiveWorkspace != WorkspaceWebOps {
		t.Errorf("Expected ActiveWorkspace to be WorkspaceWebOps for TabNginx, got %v", m.ActiveWorkspace)
	}
}

