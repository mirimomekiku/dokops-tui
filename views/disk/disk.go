package disk

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dok-ops/internal/actionmenu"
	"dok-ops/internal/theme"
)

type FileNode struct {
	Path      string
	Name      string
	Size      int64
	IsDir     bool
	ItemCount int
	Children  []*FileNode
	Parent    *FileNode
}

type ScanProgressMsg struct {
	ScannedCount int64
	CurrentPath  string
}

type ScanCompleteMsg struct {
	Root *FileNode
	Err  error
}

type DeleteCompleteMsg struct {
	Path string
	Err  error
}

type Model struct {
	rootPath       string
	rootDir        *FileNode
	currentNode    *FileNode
	cursor         int
	scrollOffset   int
	isScanning     bool
	scannedNodes   int64
	currentScan    string
	spinner        spinner.Model
	actionMenu     actionmenu.Model
	width          int
	height         int
	sortBySize     bool
	confirmDelete  bool
	deleteTarget   *FileNode
	statusMessage  string
	err            error
}

func New(initialPath string) Model {
	if initialPath == "" {
		var err error
		initialPath, err = os.Getwd()
		if err != nil {
			initialPath = "."
		}
	}
	initialPath, _ = filepath.Abs(initialPath)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(theme.ColorHighlight)

	return Model{
		rootPath:   initialPath,
		spinner:    s,
		sortBySize: true,
		isScanning: true,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.startScan(m.rootPath),
	)
}

func (m Model) startScan(rootPath string) tea.Cmd {
	return func() tea.Msg {
		var scanned int64
		root := &FileNode{
			Path:  rootPath,
			Name:  filepath.Base(rootPath),
			IsDir: true,
		}

		type nodeMapEntry struct {
			node *FileNode
		}
		nodeMap := make(map[string]*FileNode)
		nodeMap[rootPath] = root
		var mu sync.Mutex

		err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // Skip unreadable paths without crashing
			}
			atomic.AddInt64(&scanned, 1)

			if path == rootPath {
				return nil
			}

			info, err := d.Info()
			var size int64
			if err == nil {
				size = info.Size()
			}

			node := &FileNode{
				Path:  path,
				Name:  d.Name(),
				Size:  size,
				IsDir: d.IsDir(),
			}

			mu.Lock()
			nodeMap[path] = node
			parentPath := filepath.Dir(path)
			if parent, exists := nodeMap[parentPath]; exists {
				node.Parent = parent
				parent.Children = append(parent.Children, node)
			}
			mu.Unlock()

			return nil
		})

		if err != nil {
			return ScanCompleteMsg{Err: err}
		}

		// Calculate recursive directory sizes
		calculateDirectorySizes(root)

		return ScanCompleteMsg{Root: root}
	}
}

func calculateDirectorySizes(node *FileNode) int64 {
	if !node.IsDir {
		return node.Size
	}
	var totalSize int64
	var totalItems int
	for _, child := range node.Children {
		childSize := calculateDirectorySizes(child)
		totalSize += childSize
		totalItems++
		if child.IsDir {
			totalItems += child.ItemCount
		}
	}
	node.Size = totalSize
	node.ItemCount = totalItems
	return totalSize
}

func sortNodeChildren(node *FileNode, bySize bool) {
	if node == nil {
		return
	}
	if bySize {
		sort.Slice(node.Children, func(i, j int) bool {
			if node.Children[i].Size != node.Children[j].Size {
				return node.Children[i].Size > node.Children[j].Size
			}
			return node.Children[i].Name < node.Children[j].Name
		})
	} else {
		sort.Slice(node.Children, func(i, j int) bool {
			if node.Children[i].IsDir != node.Children[j].IsDir {
				return node.Children[i].IsDir // Directories first
			}
			return node.Children[i].Name < node.Children[j].Name
		})
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case spinner.TickMsg:
		if m.isScanning {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case ScanProgressMsg:
		m.scannedNodes = msg.ScannedCount
		m.currentScan = msg.CurrentPath

	case ScanCompleteMsg:
		m.isScanning = false
		if msg.Err != nil {
			m.err = msg.Err
			m.statusMessage = fmt.Sprintf("Scan error: %v", msg.Err)
		} else {
			m.rootDir = msg.Root
			m.currentNode = msg.Root
			sortNodeChildren(m.currentNode, m.sortBySize)
			m.cursor = 0
			m.scrollOffset = 0
			m.statusMessage = fmt.Sprintf("Scanned %d items successfully.", m.rootDir.ItemCount+1)
		}

	case DeleteCompleteMsg:
		if msg.Err != nil {
			m.statusMessage = fmt.Sprintf("Failed to delete %s: %v", filepath.Base(msg.Path), msg.Err)
		} else {
			m.statusMessage = fmt.Sprintf("Deleted %s", filepath.Base(msg.Path))
			// Rescan current directory
			m.isScanning = true
			cmds = append(cmds, m.spinner.Tick, m.startScan(m.rootPath))
		}

	case tea.KeyMsg:
		if m.actionMenu.IsOpen() {
			action, closed := m.actionMenu.Update(msg)
			if closed && action != "" {
				switch action {
				case "open":
					if m.currentNode != nil && m.cursor < len(m.currentNode.Children) {
						selected := m.currentNode.Children[m.cursor]
						if selected.IsDir && len(selected.Children) > 0 {
							m.currentNode = selected
							sortNodeChildren(m.currentNode, m.sortBySize)
							m.cursor = 0
							m.scrollOffset = 0
						}
					}
				case "parent":
					if m.currentNode != nil && m.currentNode.Parent != nil {
						prevNode := m.currentNode
						m.currentNode = m.currentNode.Parent
						sortNodeChildren(m.currentNode, m.sortBySize)
						m.cursor = 0
						for i, child := range m.currentNode.Children {
							if child == prevNode {
								m.cursor = i
								break
							}
						}
						m.adjustScroll()
					}
				case "sort_size":
					m.sortBySize = true
					if m.currentNode != nil {
						sortNodeChildren(m.currentNode, m.sortBySize)
						m.cursor = 0
						m.scrollOffset = 0
					}
				case "sort_name":
					m.sortBySize = false
					if m.currentNode != nil {
						sortNodeChildren(m.currentNode, m.sortBySize)
						m.cursor = 0
						m.scrollOffset = 0
					}
				case "delete":
					if m.currentNode != nil && m.cursor < len(m.currentNode.Children) {
						m.deleteTarget = m.currentNode.Children[m.cursor]
						m.confirmDelete = true
					}
				case "rescan":
					m.isScanning = true
					if m.currentNode != nil {
						m.rootPath = m.currentNode.Path
					}
					cmds = append(cmds, m.spinner.Tick, m.startScan(m.rootPath))
				}
			}
			return m, tea.Batch(cmds...)
		}

		if m.confirmDelete {
			switch msg.String() {
			case "y", "Y", "enter":
				if m.deleteTarget != nil {
					targetPath := m.deleteTarget.Path
					cmds = append(cmds, func() tea.Msg {
						err := os.RemoveAll(targetPath)
						return DeleteCompleteMsg{Path: targetPath, Err: err}
					})
				}
				m.confirmDelete = false
			case "n", "N", "esc", "space":
				m.confirmDelete = false
				m.statusMessage = "Delete cancelled."
			}
			return m, tea.Batch(cmds...)
		}

		numChildren := 0
		if m.currentNode != nil {
			numChildren = len(m.currentNode.Children)
		}

		switch msg.String() {
		case "j", "down":
			if m.cursor < numChildren-1 {
				m.cursor++
				m.adjustScroll()
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.adjustScroll()
			}
		case "space":
			itemName := "Directory"
			if m.currentNode != nil && m.cursor < len(m.currentNode.Children) {
				itemName = m.currentNode.Children[m.cursor].Name
			}
			title := "Actions: " + itemName
			subtitle := "Select storage operation"
			items := []actionmenu.Item{
				{Key: "open", Title: "Open Directory", Description: "Descend into selected folder"},
				{Key: "parent", Title: "Parent Directory", Description: "Navigate to parent folder"},
				{Key: "sort_size", Title: "Sort by Size (Descending)", Description: "Largest items first"},
				{Key: "sort_name", Title: "Sort by Name (Ascending)", Description: "Alphabetical ordering"},
				{Key: "delete", Title: "Delete File / Folder", Description: "Permanently remove selected item", Danger: true},
				{Key: "rescan", Title: "Rescan Directory", Description: "Recalculate storage metrics"},
			}
			m.actionMenu.Open(title, subtitle, items)
			return m, nil

		case "enter":
			if m.currentNode != nil && m.cursor < len(m.currentNode.Children) {
				selected := m.currentNode.Children[m.cursor]
				if selected.IsDir && len(selected.Children) > 0 {
					m.currentNode = selected
					sortNodeChildren(m.currentNode, m.sortBySize)
					m.cursor = 0
					m.scrollOffset = 0
				}
			}
		}
	}

	return m, nil
}

func (m *Model) adjustScroll() {
	visibleRows := m.height - 6
	if visibleRows < 4 {
		visibleRows = 4
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	} else if m.cursor >= m.scrollOffset+visibleRows {
		m.scrollOffset = m.cursor - visibleRows + 1
	}
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	if m.isScanning {
		return lipgloss.NewStyle().
			Padding(2, 3).
			Width(contentWidth).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("Disk Space Analyzer (ncdu)"),
					"",
					fmt.Sprintf("%s Scanning filesystem tree...", m.spinner.View()),
					lipgloss.NewStyle().Foreground(theme.ColorInfo).Render(fmt.Sprintf("Target Path: %s", m.rootPath)),
					"",
					lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("Indexing directories and aggregating sizes hierarchically..."),
				),
			)
	}

	if m.currentNode == nil {
		return "No disk data available."
	}

	pathBar := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("Disk Space"),
		"   ",
		lipgloss.NewStyle().Bold(true).Foreground(theme.ColorHighlight).Render(m.currentNode.Path),
		"   ",
		lipgloss.NewStyle().Foreground(theme.ColorSuccess).Render(theme.FormatIntBytes(m.currentNode.Size)),
		"  ",
		lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf("(%d items)", len(m.currentNode.Children))),
	)

	statusLine := ""
	if m.confirmDelete && m.deleteTarget != nil {
		statusLine = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(theme.ColorDanger).
			Bold(true).
			Padding(0, 1).
			Render(fmt.Sprintf("[!] Permanently delete '%s'? (y/N)", m.deleteTarget.Name))
	} else if m.statusMessage != "" {
		statusLine = lipgloss.NewStyle().Foreground(theme.ColorHighlight).Render("  " + m.statusMessage)
	}

	// Render Directory Items Table
	visibleRows := m.height - 6
	if visibleRows < 4 {
		visibleRows = 4
	}

	start := m.scrollOffset
	end := start + visibleRows
	if end > len(m.currentNode.Children) {
		end = len(m.currentNode.Children)
	}

	barWidth := 20

	var rows []string
	if len(m.currentNode.Children) == 0 {
		rows = append(rows, lipgloss.NewStyle().Foreground(theme.ColorMuted).Italic(true).Render("  (empty directory)"))
	} else {
		for i := start; i < end; i++ {
			node := m.currentNode.Children[i]
			isSelected := (i == m.cursor)

			nameStr := node.Name
			if node.IsDir {
				nameStr = "/" + node.Name
			}

			sizeStr := fmt.Sprintf("%10s", theme.FormatIntBytes(node.Size))

			pct := 0.0
			if m.currentNode.Size > 0 {
				pct = (float64(node.Size) / float64(m.currentNode.Size)) * 100.0
			}
			bar := theme.RenderProgressBar(barWidth, pct, theme.ColorPrimary, theme.ColorSurfaceAlt)
			pctStr := fmt.Sprintf("%5.1f%%", pct)

			gutter := "  "
			if isSelected {
				gutter = "▶ "
			}

			nameWidth := contentWidth - 12 - barWidth - 10 - 8
			if nameWidth < 15 {
				nameWidth = 15
			}
			if len(nameStr) > nameWidth {
				nameStr = nameStr[:nameWidth-1] + "…"
			}

			nameStyled := lipgloss.NewStyle().Width(nameWidth).Render(nameStr)
			if node.IsDir {
				nameStyled = lipgloss.NewStyle().Width(nameWidth).Bold(true).Foreground(theme.ColorHighlight).Render(nameStr)
			}

			line := fmt.Sprintf("%s%s  [ %s ] %s  %s", gutter, sizeStr, bar, pctStr, nameStyled)
			if isSelected {
				line = lipgloss.NewStyle().
					Background(lipgloss.Color("#283457")).
					Foreground(lipgloss.Color("#ffffff")).
					Bold(true).
					Width(contentWidth - 2).
					Render(line)
			}
			rows = append(rows, line)
		}
	}

	elements := []string{
		pathBar,
	}
	if statusLine != "" {
		elements = append(elements, statusLine)
	}
	elements = append(elements, "", lipgloss.JoinVertical(lipgloss.Left, rows...))

	rendered := lipgloss.NewStyle().
		Padding(0, 1).
		Width(contentWidth).
		Render(lipgloss.JoinVertical(lipgloss.Left, elements...))

	return m.actionMenu.RenderModal(rendered, m.width, m.height)
}
