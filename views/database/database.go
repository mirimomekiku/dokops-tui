package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"dok-ops/internal/theme"
)

type DBType string

const (
	DBPostgres DBType = "postgres"
	DBMySQL    DBType = "mysql"
)

type QueryResultMsg struct {
	Columns  []string
	Rows     [][]string
	Duration time.Duration
	Err      error
}

type HealthStatsMsg struct {
	ActiveConns int
	IdleConns   int
	TotalConns  int
	DBSize      string
	Err         error
}

type Model struct {
	dbType        DBType
	connInput     textinput.Model
	queryInput    textinput.Model
	db            *sql.DB
	isConnected   bool
	table         table.Model
	viewport      viewport.Model
	activeConns   int
	dbSize        string
	execDuration  time.Duration
	statusMessage string
	focusQuery    bool
	width         int
	height        int
	err           error
}

func New() Model {
	ci := textinput.New()
	ci.Placeholder = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	ci.SetValue("postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable")
	ci.CharLimit = 1000
	ci.Width = 60

	qi := textinput.New()
	qi.Placeholder = "SELECT datname, numbackends, xact_commit, blks_read FROM pg_stat_database LIMIT 10;"
	qi.SetValue("SELECT pid, usename, client_addr, state, query FROM pg_stat_activity WHERE state IS NOT NULL LIMIT 10;")
	qi.CharLimit = 2000
	qi.Width = 60

	t := table.New(table.WithFocused(true), table.WithHeight(8))
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

	vp := viewport.New(80, 10)

	return Model{
		dbType:      DBPostgres,
		connInput:   ci,
		queryInput:  qi,
		table:       t,
		viewport:    vp,
		focusQuery:  true,
		isConnected: false,
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) ExecuteQuery(queryStr string) tea.Cmd {
	return func() tea.Msg {
		if m.db == nil {
			return QueryResultMsg{Err: fmt.Errorf("not connected to database. Press Enter on connection string first")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		start := time.Now()
		rows, err := m.db.QueryContext(ctx, queryStr)
		if err != nil {
			return QueryResultMsg{Duration: time.Since(start), Err: err}
		}
		defer rows.Close()

		cols, err := rows.Columns()
		if err != nil {
			return QueryResultMsg{Duration: time.Since(start), Err: err}
		}

		var resultRows [][]string
		for rows.Next() {
			values := make([]interface{}, len(cols))
			valuePtrs := make([]interface{}, len(cols))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := rows.Scan(valuePtrs...); err != nil {
				continue
			}

			var rowStr []string
			for _, val := range values {
				if val == nil {
					rowStr = append(rowStr, "NULL")
				} else {
					switch v := val.(type) {
					case []byte:
						rowStr = append(rowStr, string(v))
					default:
						rowStr = append(rowStr, fmt.Sprintf("%v", v))
					}
				}
			}
			resultRows = append(resultRows, rowStr)
		}

		return QueryResultMsg{
			Columns:  cols,
			Rows:     resultRows,
			Duration: time.Since(start),
			Err:      rows.Err(),
		}
	}
}

func (m Model) ConnectDB() tea.Cmd {
	connStr := strings.TrimSpace(m.connInput.Value())
	driver := "pgx"
	if strings.HasPrefix(connStr, "mysql://") || strings.Contains(connStr, "@tcp(") {
		driver = "mysql"
		connStr = strings.TrimPrefix(connStr, "mysql://")
	}

	return func() tea.Msg {
		db, err := sql.Open(driver, connStr)
		if err != nil {
			return HealthStatsMsg{Err: err}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			db.Close()
			return HealthStatsMsg{Err: fmt.Errorf("ping failed: %v", err)}
		}

		return HealthStatsMsg{ActiveConns: 1, TotalConns: 10}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case HealthStatsMsg:
		if msg.Err != nil {
			m.isConnected = false
			m.err = msg.Err
			m.statusMessage = fmt.Sprintf("Connection failed: %v", msg.Err)
		} else {
			m.isConnected = true
			m.err = nil
			m.activeConns = msg.ActiveConns
			m.statusMessage = "Connected successfully to database"
			// Execute initial health query
			cmds = append(cmds, m.ExecuteQuery(m.queryInput.Value()))
		}

	case QueryResultMsg:
		m.execDuration = msg.Duration
		if msg.Err != nil {
			m.err = msg.Err
			m.statusMessage = fmt.Sprintf("Query error (took %s): %v", theme.FormatDuration(msg.Duration), msg.Err)
		} else {
			m.err = nil
			m.statusMessage = fmt.Sprintf("Returned %d rows in %s", len(msg.Rows), theme.FormatDuration(msg.Duration))
			m.renderTableResults(msg.Columns, msg.Rows)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m.focusQuery = !m.focusQuery
			if m.focusQuery {
				m.queryInput.Focus()
				m.connInput.Blur()
			} else {
				m.connInput.Focus()
				m.queryInput.Blur()
			}
			return m, nil

		case "enter":
			if !m.focusQuery {
				// Connect
				m.statusMessage = "Connecting to database..."
				driver := "pgx"
				connStr := strings.TrimSpace(m.connInput.Value())
				if strings.HasPrefix(connStr, "mysql://") || strings.Contains(connStr, "@tcp(") {
					driver = "mysql"
					connStr = strings.TrimPrefix(connStr, "mysql://")
				}
				if m.db != nil {
					_ = m.db.Close()
				}
				db, err := sql.Open(driver, connStr)
				if err == nil {
					m.db = db
				}
				return m, m.ConnectDB()
			} else {
				// Execute Query
				return m, m.ExecuteQuery(m.queryInput.Value())
			}
		}

		var cmd tea.Cmd
		if m.focusQuery {
			m.queryInput, cmd = m.queryInput.Update(msg)
		} else {
			m.connInput, cmd = m.connInput.Update(msg)
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

func (m *Model) renderTableResults(cols []string, data [][]string) {
	if len(cols) == 0 {
		return
	}

	colWidth := (m.width - 10) / len(cols)
	if colWidth < 12 {
		colWidth = 12
	}

	var tableCols []table.Column
	for _, c := range cols {
		tableCols = append(tableCols, table.Column{
			Title: strings.ToUpper(c),
			Width: colWidth,
		})
	}
	m.table.SetColumns(tableCols)

	var rows []table.Row
	for _, r := range data {
		rows = append(rows, table.Row(r))
	}
	m.table.SetRows(rows)
}

func (m *Model) updateLayout() {
	inputWidth := m.width - 24
	if inputWidth < 30 {
		inputWidth = 30
	}
	m.connInput.Width = inputWidth
	m.queryInput.Width = inputWidth

	tableHeight := m.height - 14
	if tableHeight < 5 {
		tableHeight = 5
	}
	m.table.SetHeight(tableHeight)
}

func (m Model) View() string {
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	var statusBadge string
	if m.isConnected {
		statusBadge = theme.BadgeSuccess.Render(" CONNECTED ")
	} else {
		statusBadge = theme.BadgeDanger.Render(" DISCONNECTED ")
	}

	connBorder := theme.ColorBorder
	if !m.focusQuery {
		connBorder = theme.ColorPrimary
	}
	connBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(connBorder).
		Padding(0, 1).
		Render(
			lipgloss.JoinHorizontal(lipgloss.Center,
				lipgloss.NewStyle().Bold(true).Foreground(theme.ColorPrimary).Render("Conn URI: "),
				m.connInput.View(),
				"  ",
				statusBadge,
			),
		)

	queryBorder := theme.ColorBorder
	if m.focusQuery {
		queryBorder = theme.ColorPrimary
	}
	queryBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(queryBorder).
		Padding(0, 1).
		Render(
			lipgloss.JoinHorizontal(lipgloss.Center,
				lipgloss.NewStyle().Bold(true).Foreground(theme.ColorSecondary).Render("SQL Query: "),
				m.queryInput.View(),
			),
		)

	statusLine := ""
	if m.statusMessage != "" {
		statusLine = lipgloss.NewStyle().Foreground(theme.ColorHighlight).Render(m.statusMessage)
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center,
			theme.CardTitleStyle.Render("🗄️ DATABASE HEALTH & QUERY RUNNER (Postgres/MySQL)"),
			"   ",
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("[Tab: Switch Conn/Query | Enter: Run | j/k: Table]"),
		),
		"",
		connBox,
		queryBox,
		statusLine,
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
