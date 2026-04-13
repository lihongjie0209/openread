// Package tui provides a Bubbletea TUI for real-time generation progress,
// matching the original zread_cli interaction model.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// ── Message types ──────────────────────────────────────────────────────────────

// CatalogStartMsg signals the catalog phase has begun.
type CatalogStartMsg struct{}

// CatalogStatusMsg carries a real-time agent status for the catalog phase.
type CatalogStatusMsg struct{ Status string }

// CatalogDoneMsg signals the catalog phase completed successfully.
type CatalogDoneMsg struct{ Pages, Sections int }

// PagesInitMsg sets up the page list for phase 2.
type PagesInitMsg struct{ Pages []PageRow }

// PageStartMsg marks a page as started (sent right before generation).
type PageStartMsg struct{ Slug string }

// PageStatusMsg carries a real-time per-page agent status.
type PageStatusMsg struct{ Slug, Status string }

// PageDoneMsg marks a page as successfully completed.
type PageDoneMsg struct{ Slug string }

// PageFailedMsg marks a page as failed.
type PageFailedMsg struct{ Slug, Err string }

// PageRetryingMsg resets a failed page back to retrying state.
type PageRetryingMsg struct{ Slug string }

// GenerationDoneMsg signals the full generation (including retries) is done.
type GenerationDoneMsg struct {
	VersionID  string
	TotalPages int
}

// ── Page row ──────────────────────────────────────────────────────────────────

// PageStatus represents the display state of a single page.
type PageStatus int

const (
	StatusPending  PageStatus = iota
	StatusRunning             // active generation
	StatusRetrying            // waiting to retry
	StatusDone
	StatusFailed
)

// PageRow holds display data for one wiki page.
type PageRow struct {
	Idx         int
	Title       string
	Slug        string
	Status      PageStatus
	AgentStatus string // "[requesting]", "[tool: list_dir]", etc.
	Err         string
}

// ── Styles ────────────────────────────────────────────────────────────────────

// Table column widths (in terminal visual columns).
const (
	colNum    = 3  // row number, e.g. " 1 "
	colTitle  = 36 // page title (CJK-aware)
	colStatus = 20 // status badge
)

var (
	styleDim     = lipgloss.NewStyle().Faint(true)
	styleCyan    = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	styleHeader  = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	styleSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	styleCursor  = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)

	// Per-status badge styles.
	styleSWaiting  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleSRunning  = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	styleSThinking = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleSTool     = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	styleSAnswering = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	styleSRetrying  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleSDone      = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	styleSFailed    = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)

	// Table chrome.
	styleTblHdr = lipgloss.NewStyle().Faint(true).Bold(true)
	styleTblSep = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// styledStatus returns a colored icon+text badge for the given page row.
func styledStatus(p PageRow) string {
	switch p.Status {
	case StatusPending:
		return styleSWaiting.Render("·  waiting")
	case StatusRetrying:
		return styleSRetrying.Render("↻  retrying")
	case StatusDone:
		return styleSDone.Render("✓  done")
	case StatusFailed:
		return styleSFailed.Render("✗  failed")
	case StatusRunning:
		s := p.AgentStatus
		switch {
		case s == "[thinking]":
			return styleSThinking.Render("◆  thinking")
		case s == "[answering]":
			return styleSAnswering.Render("⟳  answering")
		case strings.HasPrefix(s, "[tool:"):
			// extract tool name from "[tool: list_dir]"
			name := strings.TrimSuffix(strings.TrimPrefix(s, "[tool: "), "]")
			if runewidth.StringWidth(name) > 10 {
				name = runewidth.Truncate(name, 10, "…")
			}
			return styleSTool.Render("⚙  " + name)
		default:
			return styleSRunning.Render("⟳  requesting")
		}
	}
	return styleSWaiting.Render("·  waiting")
}

// ── Model ─────────────────────────────────────────────────────────────────────

type phase int

const (
	phaseCatalog phase = iota
	phasePages
	phaseDone
)

// Model is the Bubbletea model for generation TUI.
type Model struct {
	phase   phase
	spinner spinner.Model

	// Catalog phase
	catalogStatus string
	catalogDone   bool

	// Pages phase
	pages     []PageRow
	slugIndex map[string]int
	cursor    int // currently selected row (0-based)
	scrollTop int // index of first visible row

	// Done phase
	versionID  string
	totalPages int

	// Channels for communicating user intent back to the runner goroutine.
	// Channels are reference types — safe across Model copies.
	retryCh chan<- string
	skipCh  chan<- struct{}

	// Terminal size
	width  int
	height int

	// Progress counters
	doneCount int
	failCount int
}

// New creates a plain Model (no retry channel — for plain/quiet mode).
func New() Model {
	return newModel(nil, nil)
}

// NewWithChannels creates a Model that can send per-page retry and skip-all
// requests back to the runner goroutine.
func NewWithChannels(retryCh chan<- string, skipCh chan<- struct{}) Model {
	return newModel(retryCh, skipCh)
}

func newModel(retryCh chan<- string, skipCh chan<- struct{}) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styleCyan
	return Model{
		phase:     phaseCatalog,
		spinner:   sp,
		slugIndex: make(map[string]int),
		retryCh:   retryCh,
		skipCh:    skipCh,
	}
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	// ── Catalog messages ────────────────────────────────────────────────────

	case CatalogStartMsg:
		m.phase = phaseCatalog

	case CatalogStatusMsg:
		m.catalogStatus = msg.Status

	case CatalogDoneMsg:
		m.catalogDone = true
		m.catalogStatus = "[done]"

	// ── Pages messages ──────────────────────────────────────────────────────

	case PagesInitMsg:
		m.phase = phasePages
		m.pages = msg.Pages
		m.slugIndex = make(map[string]int, len(msg.Pages))
		for i, p := range m.pages {
			m.slugIndex[p.Slug] = i
		}

	case PageStartMsg:
		if i, ok := m.slugIndex[msg.Slug]; ok {
			m.pages[i].Status = StatusRunning
			m.pages[i].AgentStatus = "[requesting]"
		}

	case PageStatusMsg:
		if i, ok := m.slugIndex[msg.Slug]; ok {
			m.pages[i].AgentStatus = msg.Status
		}

	case PageDoneMsg:
		if i, ok := m.slugIndex[msg.Slug]; ok {
			m.pages[i].Status = StatusDone
			m.pages[i].AgentStatus = "[done]"
			m.doneCount++
		}

	case PageFailedMsg:
		if i, ok := m.slugIndex[msg.Slug]; ok {
			m.pages[i].Status = StatusFailed
			m.pages[i].Err = msg.Err
			m.failCount++
		}

	case PageRetryingMsg:
		if i, ok := m.slugIndex[msg.Slug]; ok {
			m.pages[i].Status = StatusRetrying
			m.pages[i].AgentStatus = "[waiting to retry]"
			// This page is no longer "failed" while waiting for retry
			m.failCount--
		}

	case GenerationDoneMsg:
		m.versionID = msg.VersionID
		m.totalPages = msg.TotalPages
		m.phase = phaseDone
		return m, tea.Quit

	// ── Key input ───────────────────────────────────────────────────────────

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}

	if m.phase != phasePages {
		return m, nil
	}

	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.scrollTop {
				m.scrollTop = m.cursor
			}
		}

	case "down", "j":
		if m.cursor < len(m.pages)-1 {
			m.cursor++
			maxVis := m.visibleRows()
			if m.cursor >= m.scrollTop+maxVis {
				m.scrollTop = m.cursor - maxVis + 1
			}
		}

	case "r":
		if m.retryCh != nil && m.cursor >= 0 && m.cursor < len(m.pages) {
			p := &m.pages[m.cursor]
			if p.Status == StatusFailed {
				slug := p.Slug
				p.Status = StatusRetrying
				p.AgentStatus = "[waiting to retry]"
				m.failCount--
				ch := m.retryCh
				return m, func() tea.Msg {
					ch <- slug
					return nil
				}
			}
		}

	case "s":
		if m.skipCh != nil && m.failCount > 0 {
			ch := m.skipCh
			return m, func() tea.Msg {
				ch <- struct{}{}
				return nil
			}
		}
	}

	return m, nil
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	var sb strings.Builder
	switch m.phase {
	case phaseCatalog:
		m.renderCatalog(&sb)
	case phasePages:
		m.renderPages(&sb)
	case phaseDone:
		m.renderDone(&sb)
	}
	return sb.String()
}

func (m *Model) renderCatalog(sb *strings.Builder) {
	sb.WriteString(styleHeader.Render("Phase 1 — Generate Catalog"))
	sb.WriteByte('\n')

	sb.WriteString("  ")
	if m.catalogDone {
		sb.WriteString(styleDim.Render("[done]"))
	} else {
		sb.WriteString(m.spinner.View())
		sb.WriteString(" ")
		sb.WriteString(styleCyan.Render("⟳ Generating Catalog"))
		if m.catalogStatus != "" {
			sb.WriteString("  ")
			sb.WriteString(styleDim.Render(m.catalogStatus))
		}
	}
	sb.WriteByte('\n')
	sb.WriteByte('\n')
	sb.WriteString(styleDim.Render("ctrl+c: quit"))
	sb.WriteByte('\n')
}

func (m *Model) renderPages(sb *strings.Builder) {
	total := len(m.pages)
	counted := m.doneCount + m.failCount

	header := fmt.Sprintf("Phase 2 — Generate Pages (%d/%d)", counted, total)
	sb.WriteString(styleHeader.Render(header))
	sb.WriteString("\n\n")

	maxVis := m.visibleRows()
	start := m.scrollTop
	end := start + maxVis
	if end > total {
		end = total
	}

	// ── Table header ────────────────────────────────────────────────────────
	numHdr := lipgloss.NewStyle().Width(colNum).Align(lipgloss.Right).Render("#")
	titleHdr := lipgloss.NewStyle().Width(colTitle).Render("Page")
	statusHdr := lipgloss.NewStyle().Width(colStatus).Align(lipgloss.Right).Render("Status")
	sb.WriteString(styleTblHdr.Render("   " + numHdr + "  " + titleHdr + "  " + statusHdr))
	sb.WriteByte('\n')

	// Separator spanning: 3(cursor) + colNum + 2 + colTitle + 2 + colStatus
	sepW := 3 + colNum + 2 + colTitle + 2 + colStatus
	sb.WriteString(styleTblSep.Render(strings.Repeat("─", sepW)))
	sb.WriteByte('\n')

	// ── Scroll-up indicator ─────────────────────────────────────────────────
	if start > 0 {
		sb.WriteString(styleDim.Render(fmt.Sprintf("  ↑ %d more", start)))
		sb.WriteByte('\n')
	}

	// ── Rows ────────────────────────────────────────────────────────────────
	for i := start; i < end; i++ {
		p := m.pages[i]
		selected := i == m.cursor

		// Cursor glyph (2 visual cols)
		cur := "  "
		if selected {
			cur = styleCursor.Render("❯ ")
		}

		// Row number cell
		numCell := lipgloss.NewStyle().Width(colNum).Align(lipgloss.Right).Render(fmt.Sprintf("%d", i+1))

		// Title cell (CJK-aware truncation)
		titleText := abbrevVis(p.Title, colTitle-1)
		titleStyle := lipgloss.NewStyle().Width(colTitle)
		if selected {
			titleStyle = titleStyle.Bold(true).Foreground(lipgloss.Color("15"))
		}
		titleCell := titleStyle.Render(titleText)

		// Status cell (badge + right-align padding)
		badge := styledStatus(p)
		statusCell := lipgloss.NewStyle().Width(colStatus).Align(lipgloss.Right).Render(badge)

		sb.WriteString(cur + numCell + "  " + titleCell + "  " + statusCell)
		sb.WriteByte('\n')
	}

	// ── Scroll-down indicator ────────────────────────────────────────────────
	if end < total {
		sb.WriteString(styleDim.Render(fmt.Sprintf("  ↓ %d more", total-end)))
		sb.WriteByte('\n')
	}

	sb.WriteByte('\n')

	// Dynamic footer
	if m.failCount > 0 {
		sb.WriteString(styleDim.Render("↑/↓: navigate | r: retry | s: skip failed & commit | ctrl+c: quit"))
	} else {
		sb.WriteString(styleDim.Render("↑/↓: navigate | r: retry failed | ctrl+c: quit"))
	}
	sb.WriteByte('\n')
}

func (m *Model) renderDone(sb *strings.Builder) {
	n := m.totalPages
	if n == 0 {
		n = len(m.pages)
	}
	sb.WriteString(styleSuccess.Render(fmt.Sprintf("✓ Wiki generation complete! %d pages total", n)))
	sb.WriteByte('\n')
	if m.versionID != "" {
		sb.WriteString(styleDim.Render(fmt.Sprintf("  Version: %s", m.versionID)))
		sb.WriteByte('\n')
	}
	sb.WriteString(styleDim.Render("  Run zread browse to view your docs"))
	sb.WriteByte('\n')
	sb.WriteByte('\n')
	sb.WriteString(styleDim.Render("ctrl+c: quit"))
	sb.WriteByte('\n')
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (m *Model) visibleRows() int {
	rows := m.height - 8
	if rows < 3 {
		rows = 3
	}
	return rows
}

// FailedSlugs returns slugs of all pages still in failed state.
func (m Model) FailedSlugs() []string {
	var out []string
	for _, p := range m.pages {
		if p.Status == StatusFailed {
			out = append(out, p.Slug)
		}
	}
	return out
}

// AllPages returns the full page list.
func (m Model) AllPages() []PageRow { return m.pages }

// ── Plain verbose output (non-TUI) ────────────────────────────────────────────

// PlainLogFunc returns a tool-call callback that prints to stdout (for
// --verbose-catalog / --verbose-pages flags).
func PlainLogFunc() func(name, args, result string) {
	return func(name, args, result string) {
		fmt.Printf("  \033[36m[tool]\033[0m %s(%s)\n", name, abbrevPlain(args, 120))
		fmt.Printf("  \033[2m→ %s\033[0m\n", abbrevPlain(result, 200))
	}
}

func abbrevVis(s string, maxW int) string {
	s = strings.ReplaceAll(s, "\n", "←")
	if runewidth.StringWidth(s) <= maxW {
		return s
	}
	return runewidth.Truncate(s, maxW, "…")
}

func abbrevPlain(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", "←")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
