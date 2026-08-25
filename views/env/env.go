package env

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joho/godotenv"

	"dok-ops/internal/theme"
)

type EnvStatus string

const (
	StatusValid   EnvStatus = "VALID"
	StatusMissing EnvStatus = "MISSING ⚠️"
	StatusEmpty   EnvStatus = "EMPTY"
	StatusExtra   EnvStatus = "EXTRA / UNDOC"
)

type EnvDriftItem struct {
	Key          string
	Status       EnvStatus
	ActualValue  string
	ExampleValue string
	TypeHint     string
}

type EnvValidationMsg struct {
	Items    []EnvDriftItem
	EnvPath  string
	ExamPath string
	Err      error
}

type Model struct {
	envPathInput  textinput.Model
	examPathInput textinput.Model
	items         []EnvDriftItem
	table         table.Model
	focusExample  bool
	isLoading     bool
	width         int
	height        int
	err           error
}

func New() Model {
	epi := textinput.New()
	epi.Placeholder = ".env"
	epi.SetValue(".env")
	epi.Focus()
	epi.CharLimit = 255
	epi.Width = 30

	exi := textinput.New()
	exi.Placeholder = ".env.example"
	exi.SetValue(".env.example")
	exi.CharLimit = 255
	exi.Width = 30

	cols := []table.Column{
		{Title: "VARIABLE KEY", Width: 28},
		{Title: "DRIFT STATUS", Width: 16},
		{Title: "ACTIVE .ENV VALUE", Width: 28},
		{Title: "EXAMPLE / DEFAULT", Width: 28},
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(theme.ColorBorder).
		BorderBottom(true).
		Bold(true).
		Foreground(theme.ColorPrimary)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#3d59a1")).
		Bold(true)
	t.SetStyles(s)

	return Model{
		envPathInput:  epi,
		examPathInput: exi,
		table:         t,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.ValidateEnv(),
	)
}

func (m Model) ValidateEnv() tea.Cmd {
	envPath := strings.TrimSpace(m.envPathInput.Value())
	examPath := strings.TrimSpace(m.examPathInput.Value())

	return func() tea.Msg {
		actualMap, _ := godotenv.Read(envPath)
		exampleMap, _ := godotenv.Read(examPath)

		if len(actualMap) == 0 && len(exampleMap) == 0 {
			// If neither exists, provide sample validation
			actualMap = map[string]string{
				"PORT":      "8080",
				"DB_HOST":   "localhost",
				"DB_PASS":   "",
				"DEBUG":     "true",
				"NEW_FEAT":  "enabled",
			}
			exampleMap = map[string]string{
				"PORT":      "3000",
				"DB_HOST":   "127.0.0.1",
				"DB_PASS":   "secret",
				"API_KEY":   "your_key_here",
				"DEBUG":     "false",
			}
		}

		allKeys := make(map[string]struct{})
		for k := range actualMap {
			allKeys[k] = struct{}{}
		}
		for k := range exampleMap {
			allKeys[k] = struct{}{}
		}

		var items []EnvDriftItem
		for k := range allKeys {
			actVal, actExists := actualMap[k]
			exVal, exExists := exampleMap[k]

			status := StatusValid
			if !actExists && exExists {
				status = StatusMissing
			} else if actExists && !exExists {
				status = StatusExtra
			} else if actExists && strings.TrimSpace(actVal) == "" {
				status = StatusEmpty
			}

			// Type checking hints
			typeHint := "string"
			if _, err := strconv.Atoi(exVal); err == nil {
				typeHint = "int"
			} else if strings.ToLower(exVal) == "true" || strings.ToLower(exVal) == "false" {
				typeHint = "bool"
			}

			maskedActVal := actVal
			if strings.Contains(strings.ToLower(k), "pass") || strings.Contains(strings.ToLower(k), "secret") || strings.Contains(strings.ToLower(k), "key") {
				if len(actVal) > 4 {
					maskedActVal = actVal[:2] + strings.Repeat("•", len(actVal)-4) + actVal[len(actVal)-2:]
				} else if len(actVal) > 0 {
					maskedActVal = "••••"
				}
			}

			items = append(items, EnvDriftItem{
				Key:          k,
				Status:       status,
				ActualValue:  maskedActVal,
				ExampleValue: exVal,
				TypeHint:     typeHint,
			})
		}

		// Sort missing first, then empty, then key
		sort.Slice(items, func(i, j int) bool {
			if items[i].Status != items[j].Status {
				if items[i].Status == StatusMissing {
					return true
				}
				if items[j].Status == StatusMissing {
					return false
				}
				if items[i].Status == StatusEmpty {
					return true
				}
				if items[j].Status == StatusEmpty {
					return false
				}
			}
			return items[i].Key < items[j].Key
		})

		return EnvValidationMsg{
			Items:    items,
			EnvPath:  envPath,
			ExamPath: examPath,
		}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case EnvValidationMsg:
		m.isLoading = false
		m.items = msg.Items
		m.updateTableRows()

	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m.focusExample = !m.focusExample
			if m.focusExample {
				m.examPathInput.Focus()
				m.envPathInput.Blur()
			} else {
				m.envPathInput.Focus()
				m.examPathInput.Blur()
			}
			return m, nil

		case "enter", "r":
			m.isLoading = true
			return m, m.ValidateEnv()
		}

		var cmd tea.Cmd
		if m.focusExample {
			m.examPathInput, cmd = m.examPathInput.Update(msg)
		} else {
			m.envPathInput, cmd = m.envPathInput.Update(msg)
		}
		cmds = append(cmds, cmd)
	}

	var tableCmd tea.Cmd
	m.table, tableCmd = m.table.Update(msg)
	if tableCmd != nil {
		cmds = append(cmds, tableCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateTableRows() {
	var rows []table.Row
	for _, item := range m.items {
		actVal := item.ActualValue
		if actVal == "" {
			actVal = "(empty)"
		}
		rows = append(rows, table.Row{
			item.Key,
			string(item.Status),
			actVal,
			item.ExampleValue,
		})
	}
	m.table.SetRows(rows)
}

func (m *Model) updateLayout() {
	contentHeight := m.height - 12
	if contentHeight < 6 {
		contentHeight = 6
	}
	m.table.SetHeight(contentHeight)

	availableWidth := m.width - 6
	if availableWidth > 80 {
		colWidth := (availableWidth - 10) / 4
		if colWidth < 15 {
			colWidth = 15
		}
		cols := []table.Column{
			{Title: "VARIABLE KEY", Width: colWidth + 5},
			{Title: "DRIFT STATUS", Width: colWidth - 2},
			{Title: "ACTIVE .ENV VALUE", Width: colWidth + 5},
			{Title: "EXAMPLE / DEFAULT", Width: colWidth},
		}
		m.table.SetColumns(cols)
	}
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	envBorder := theme.ColorBorder
	if !m.focusExample {
		envBorder = theme.ColorPrimary
	}
	envBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(envBorder).
		Padding(0, 1).
		Render(
			lipgloss.JoinHorizontal(lipgloss.Center,
				lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("Target .env: "),
				m.envPathInput.View(),
			),
		)

	examBorder := theme.ColorBorder
	if m.focusExample {
		examBorder = theme.ColorPrimary
	}
	examBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(examBorder).
		Padding(0, 1).
		Render(
			lipgloss.JoinHorizontal(lipgloss.Center,
				lipgloss.NewStyle().Bold(true).Foreground(theme.ColorSecondary).Render("Template .example: "),
				m.examPathInput.View(),
			),
		)

	var missing, empty, extra, valid int
	for _, item := range m.items {
		switch item.Status {
		case StatusMissing:
			missing++
		case StatusEmpty:
			empty++
		case StatusExtra:
			extra++
		case StatusValid:
			valid++
		}
	}

	statsBadge := lipgloss.JoinHorizontal(lipgloss.Center,
		theme.BadgeSuccess.Render(fmt.Sprintf(" %d Synced ", valid)),
		" ",
		theme.BadgeDanger.Render(fmt.Sprintf(" %d Missing ", missing)),
		" ",
		theme.BadgeWarning.Render(fmt.Sprintf(" %d Empty ", empty)),
		" ",
		theme.BadgeInfo.Render(fmt.Sprintf(" %d Extra ", extra)),
	)

	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center,
			theme.CardTitleStyle.Render("🌱 ENVIRONMENT DRIFT & .ENV VALIDATOR"),
			"   ",
			statsBadge,
			"   ",
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("[Tab: Switch File | Enter: Compare]"),
		),
		"",
		lipgloss.JoinHorizontal(lipgloss.Center,
			envBox,
			"  ",
			examBox,
		),
		"",
		m.table.View(),
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Padding(0, 1).
		Width(contentWidth).
		Render(body)
}
