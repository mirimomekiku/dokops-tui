package disk

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
		if m.confirmDelete {
			switch msg.String() {
			case "y", "Y":
				if m.deleteTarget != nil {
					targetPath := m.deleteTarget.Path
					cmds = append(cmds, func() tea.Msg {
						err := os.RemoveAll(targetPath)
						return DeleteCompleteMsg{Path: targetPath, Err: err}
					})
				}
				m.confirmDelete = false
			case "n", "N", "esc":
				m.confirmDelete = false
				m.statusMessage = "Deletion cancelled"
			}
			return m, tea.Batch(cmds...)
		}

		if m.isScanning {
			return m, nil
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
		case "g", "home":
			m.cursor = 0
			m.scrollOffset = 0
		case "G", "end":
			if numChildren > 0 {
				m.cursor = numChildren - 1
				m.adjustScroll()
			}
		case "l", "enter", "right":
			if m.currentNode != nil && m.cursor < len(m.currentNode.Children) {
				selected := m.currentNode.Children[m.cursor]
				if selected.IsDir && len(selected.Children) > 0 {
					m.currentNode = selected
					sortNodeChildren(m.currentNode, m.sortBySize)
					m.cursor = 0
					m.scrollOffset = 0
				}
			}
		case "h", "backspace", "left":
			if m.currentNode != nil && m.currentNode.Parent != nil {
				prevNode := m.currentNode
				m.currentNode = m.currentNode.Parent
				sortNodeChildren(m.currentNode, m.sortBySize)
				// Find index of previously active child
				m.cursor = 0
				for i, child := range m.currentNode.Children {
					if child == prevNode {
						m.cursor = i
						break
					}
				}
				m.adjustScroll()
			}
		case "s":
			m.sortBySize = !m.sortBySize
			if m.currentNode != nil {
				sortNodeChildren(m.currentNode, m.sortBySize)
				m.cursor = 0
				m.scrollOffset = 0
			}
		case "r":
			m.isScanning = true
			if m.currentNode != nil {
				m.rootPath = m.currentNode.Path
			}
			cmds = append(cmds, m.spinner.Tick, m.startScan(m.rootPath))
		case "d":
			if m.currentNode != nil && m.cursor < len(m.currentNode.Children) {
				m.deleteTarget = m.currentNode.Children[m.cursor]
				m.confirmDelete = true
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) adjustScroll() {
	visibleRows := m.height - 10
	if visibleRows < 5 {
		visibleRows = 5
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
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.ColorBorder).
			Padding(2, 3).
			Width(contentWidth).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					theme.CardTitleStyle.Render("📁 DISK SPACE ANALYZER (ncdu)"),
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

	// Path & Summary Header
	totalDirSize := m.currentNode.Size
	pathBar := lipgloss.JoinHorizontal(lipgloss.Center,
		theme.BadgeInfo.Render(" PATH "),
		" ",
		lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render(m.currentNode.Path),
		"  ",
		lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf("Total: %s (%d items)", theme.FormatIntBytes(totalDirSize), len(m.currentNode.Children))),
	)

	sortHint := "Size ▼"
	if !m.sortBySize {
		sortHint = "Name / Type"
	}
	helpBar := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(
		fmt.Sprintf("[j/k: Move]  [Enter/l: Open]  [Backspace/h: Up]  [s: Sort (%s)]  [r: Rescan]  [d: Delete]", sortHint),
	)

	statusLine := ""
	if m.confirmDelete && m.deleteTarget != nil {
		statusLine = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(theme.ColorDanger).
			Bold(true).
			Padding(0, 1).
			Render(fmt.Sprintf("⚠️ Permanently delete '%s'? (y/N)", m.deleteTarget.Name))
	} else if m.statusMessage != "" {
		statusLine = lipgloss.NewStyle().Foreground(theme.ColorHighlight).Render(m.statusMessage)
	}

	// Render Directory Items Table
	visibleRows := m.height - 10
	if visibleRows < 5 {
		visibleRows = 5
	}

	start := m.scrollOffset
	end := start + visibleRows
	if end > len(m.currentNode.Children) {
		end = len(m.currentNode.Children)
	}

	barWidth := contentWidth - 52
	if barWidth < 10 {
		barWidth = 10
	}

	var rows []string
	tableHeader := lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Width(12).Bold(true).Foreground(theme.ColorPrimary).Render("SIZE"),
		" ",
		lipgloss.NewStyle().Width(barWidth+10).Bold(true).Foreground(theme.ColorPrimary).Render("CAPACITY USAGE"),
		" ",
		lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("NAME / ITEM"),
	)
	rows = append(rows, tableHeader, strings.Repeat("─", contentWidth-4))

	if len(m.currentNode.Children) == 0 {
		rows = append(rows, lipgloss.NewStyle().Foreground(theme.ColorMuted).Italic(true).Render("  (Directory is empty)"))
	} else {
		for i := start; i < end; i++ {
			child := m.currentNode.Children[i]
			var pct float64
			if totalDirSize > 0 {
				pct = (float64(child.Size) / float64(totalDirSize)) * 100.0
			}

			sizeStr := lipgloss.NewStyle().Width(12).Foreground(theme.ColorInfo).Render(theme.FormatIntBytes(child.Size))
			bar := theme.RenderProgressBar(barWidth, pct, theme.ColorSuccess, theme.ColorSurfaceAlt)
			pctStr := lipgloss.NewStyle().Width(7).Foreground(theme.ColorMuted).Render(fmt.Sprintf("%5.1f%%", pct))
			capacityStr := lipgloss.JoinHorizontal(lipgloss.Left, "[", bar, "]", " ", pctStr)

			nameStr := child.Name
			if child.IsDir {
				nameStr = "📁 " + child.Name + "/"
				if child.ItemCount > 0 {
					nameStr += fmt.Sprintf(" (%d)", child.ItemCount)
				}
			} else {
				nameStr = "📄 " + child.Name
			}

			line := lipgloss.JoinHorizontal(lipgloss.Left, sizeStr, " ", capacityStr, " ", nameStr)

			if i == m.cursor {
				line = lipgloss.NewStyle().
					Background(lipgloss.Color("#3d59a1")).
					Foreground(lipgloss.Color("#ffffff")).
					Bold(true).
					Width(contentWidth - 4).
					Render(line)
			}
			rows = append(rows, line)
		}
	}

	// Scroll indicator
	if len(m.currentNode.Children) > visibleRows {
		rows = append(rows, lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf("  ▲▼ Showing %d-%d of %d items", start+1, end, len(m.currentNode.Children))))
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		pathBar,
		helpBar,
		statusLine,
		"",
		lipgloss.JoinVertical(lipgloss.Left, rows...),
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Padding(0, 1).
		Width(contentWidth).
		Render(body)
}
