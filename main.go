package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type lineKind int

const (
	kSystem lineKind = iota
	kUser
	kAgent
	kOK
	kErr
)

type chatLine struct {
	kind        lineKind
	agent, text string
}
type taskStatus string

const (
	tsPending taskStatus = "pending"
	tsRun     taskStatus = "in_progress"
	tsDone    taskStatus = "done"
)

type task struct {
	id, title, owner string
	deps             []string
	status           taskStatus
}

type flowStep struct {
	delay       time.Duration
	agent, text string
	action      string
	done        bool
}
type flowMsg struct{ step flowStep }

var (
	colorBorder = lipgloss.Color("#4B3470")
	colorText   = lipgloss.Color("#ECE7FA")
	colorSubtle = lipgloss.Color("#9D8CC4")
	colorPurple = lipgloss.Color("#B084FF")
	colorPink   = lipgloss.Color("#EC7EF6")
	colorGreen  = lipgloss.Color("#34D399")
	colorRed    = lipgloss.Color("#FB7185")
	colorBlue   = lipgloss.Color("#60A5FA")
	colorAmber  = lipgloss.Color("#F59E0B")

	styleRoot = lipgloss.NewStyle().Foreground(colorText)
	styleTop  = lipgloss.NewStyle().BorderBottom(true).BorderForeground(colorBorder).Padding(0, 1)
	styleBox  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorBorder)
	styleIn   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorPurple).Padding(0, 1)
	styleFoot = lipgloss.NewStyle().Foreground(colorSubtle).Padding(0, 1)
	styleSep  = lipgloss.NewStyle().Foreground(colorBorder)

	styleAccent = lipgloss.NewStyle().Foreground(colorPurple).Bold(true)
	styleSubtle = lipgloss.NewStyle().Foreground(colorSubtle)
	styleUser   = lipgloss.NewStyle().Foreground(colorPink).Bold(true)
	styleOK     = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	styleErr    = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
)

type model struct {
	w, h int

	vp   viewport.Model
	in   textinput.Model
	spin spinner.Model

	busy  bool
	phase string

	chat  []chatLine
	tasks []task

	agents map[string]string
	queue  []flowStep
}

func newModel() model {
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = "Type command: /plan /divide /work /tasks /clear /help"
	in.TextStyle = lipgloss.NewStyle().Foreground(colorText)
	in.PlaceholderStyle = styleSubtle
	in.Cursor.Style = lipgloss.NewStyle().Foreground(colorPurple)
	in.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = styleAccent

	vp := viewport.New(80, 20)
	vp.MouseWheelEnabled = true

	m := model{
		in:    in,
		spin:  sp,
		vp:    vp,
		phase: "idle",
		agents: map[string]string{
			"judge":    "ready",
			"claude":   "ready",
			"opencode": "ready",
		},
	}

	m.push(kSystem, "", banner())
	m.push(kSystem, "", "OpenCodeTUI (Aros CLI future UI) — dark purple two-split prototype")
	m.push(kSystem, "", "Run /plan, /divide, /work")
	m.refresh()

	return m
}

func (m model) Init() tea.Cmd { return tea.Batch(m.spin.Tick, textinput.Blink) }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.relayout()
		m.refresh()

	case spinner.TickMsg:
		var c tea.Cmd
		m.spin, c = m.spin.Update(msg)
		cmds = append(cmds, c)

	case flowMsg:
		m.applyStep(msg.step)
		if len(m.queue) > 0 {
			n := m.queue[0]
			m.queue = m.queue[1:]
			cmds = append(cmds, tickStep(n))
		}

	case tea.KeyMsg:
		s := msg.String()
		switch s {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "pgup", "pgdown", "home", "end", "ctrl+u", "ctrl+d":
			var c tea.Cmd
			m.vp, c = m.vp.Update(msg)
			return m, c
		}

		if msg.Type == tea.KeyEnter {
			t := strings.TrimSpace(m.in.Value())
			m.in.SetValue("")
			if t == "" {
				return m, nil
			}
			m.push(kUser, "", t)
			m.runCommand(strings.ToLower(t))
			m.refresh()
			if len(m.queue) > 0 && m.busy {
				n := m.queue[0]
				m.queue = m.queue[1:]
				cmds = append(cmds, tickStep(n))
			}
			return m, tea.Batch(cmds...)
		}

		var c tea.Cmd
		m.in, c = m.in.Update(msg)
		cmds = append(cmds, c)
	}

	return m, tea.Batch(cmds...)
}

func (m *model) relayout() {
	if m.w == 0 || m.h == 0 {
		return
	}
	leftW, _, mainH := panelLayout(m.w, m.h)
	m.vp.Width = max(18, leftW-4)
	m.vp.Height = max(6, mainH-4)
	m.in.Width = max(20, m.w-10)
}

func (m *model) runCommand(cmd string) {
	switch strings.TrimSpace(cmd) {
	case "/help":
		m.push(kSystem, "", "Commands: /plan /divide /work /tasks /clear /help /quit")
	case "/clear":
		m.chat = nil
	case "/tasks":
		if len(m.tasks) == 0 {
			m.push(kSystem, "", "No tasks yet. Run /divide first.")
			return
		}
		for _, t := range m.tasks {
			m.push(kSystem, "", fmt.Sprintf("%s %s %s", t.id, t.title, t.status))
		}
	case "/plan":
		if m.busy {
			m.push(kErr, "", "Workflow already running")
			return
		}
		m.phase, m.busy = "plan", true
		m.setAgent("claude", "running")
		m.setAgent("opencode", "running")
		m.queue = []flowStep{
			{260 * time.Millisecond, "claude", "Plan: architecture + state transitions.", "", false},
			{300 * time.Millisecond, "opencode", "Plan: implementation + testing strategy.", "", false},
			{300 * time.Millisecond, "judge", "Judge merged the best parts into one plan.", "", false},
			{220 * time.Millisecond, "", "Plan approved in prototype flow.", "approve", false},
		}
	case "/divide":
		if m.busy {
			m.push(kErr, "", "Workflow already running")
			return
		}
		m.phase, m.busy = "divide", true
		m.setAgent("judge", "running")
		m.queue = []flowStep{
			{250 * time.Millisecond, "judge", "Generated dependency-safe manifest.", "mkTasks", false},
			{180 * time.Millisecond, "", "Assignments approved.", "approve", false},
		}
	case "/work":
		if m.busy {
			m.push(kErr, "", "Workflow already running")
			return
		}
		if len(m.tasks) == 0 {
			m.push(kErr, "", "No tasks found. Run /divide first.")
			return
		}
		m.phase, m.busy = "work", true
		m.setAgent("claude", "running")
		m.setAgent("opencode", "running")
		m.queue = []flowStep{
			{260 * time.Millisecond, "claude", "[task-001] board engine complete", "t1", false},
			{260 * time.Millisecond, "opencode", "[task-002] input parser complete", "t2", false},
			{260 * time.Millisecond, "judge", "[task-003] final polish complete", "t3", false},
			{160 * time.Millisecond, "", "Workflow complete.", "", true},
		}
	case "/quit":
		m.push(kSystem, "", "Press Esc or Ctrl+C to quit.")
	default:
		m.push(kSystem, "", "Unknown command. Try /help")
	}
}

func (m *model) applyStep(s flowStep) {
	if s.agent != "" {
		m.push(kAgent, s.agent, s.text)
		m.setAgent(s.agent, "active")
	} else {
		m.push(kOK, "", s.text)
	}

	switch s.action {
	case "approve":
		m.setAgent("judge", "ready")
		m.setAgent("claude", "ready")
		m.setAgent("opencode", "ready")
	case "mkTasks":
		m.tasks = []task{
			{"task-001", "Build board engine", "claude", nil, tsPending},
			{"task-002", "Build input parser", "opencode", []string{"task-001"}, tsPending},
			{"task-003", "Finalize and docs", "judge", []string{"task-002"}, tsPending},
		}
	case "t1":
		m.setTask("task-001", tsDone)
		m.setTask("task-002", tsRun)
	case "t2":
		m.setTask("task-002", tsDone)
		m.setTask("task-003", tsRun)
	case "t3":
		m.setTask("task-003", tsDone)
	}

	if s.done {
		m.phase = "idle"
		m.busy = false
		m.setAgent("judge", "ready")
		m.setAgent("claude", "ready")
		m.setAgent("opencode", "ready")
	}

	m.refresh()
}

func (m *model) setTask(id string, st taskStatus) {
	for i := range m.tasks {
		if m.tasks[i].id == id {
			m.tasks[i].status = st
			return
		}
	}
}

func (m *model) setAgent(name, status string) {
	if _, ok := m.agents[name]; ok {
		m.agents[name] = status
	}
}

func (m *model) push(k lineKind, agentName, text string) {
	m.chat = append(m.chat, chatLine{kind: k, agent: agentName, text: text})
}

func (m *model) refresh() {
	var rows []string
	for _, c := range m.chat {
		switch c.kind {
		case kSystem:
			rows = append(rows, styleSubtle.Render(c.text))
		case kUser:
			rows = append(rows, styleUser.Render("[you] "+c.text))
		case kAgent:
			rows = append(rows, agentLabel(c.agent)+" "+c.text)
		case kOK:
			rows = append(rows, styleOK.Render("[ok] "+c.text))
		case kErr:
			rows = append(rows, styleErr.Render("[error] "+c.text))
		}
	}
	m.vp.SetContent(strings.Join(rows, "\n"))
	m.vp.GotoBottom()
}

func (m model) View() string {
	w := m.w
	if w == 0 {
		w = 124
	}
	h := m.h
	if h == 0 {
		h = 36
	}

	head := styleTop.Width(w).Render(styleAccent.Render(" ◆ AROS CLI TUI Prototype ") + "   phase: " + strings.ToUpper(m.phase))

	leftW, rightW, mainH := panelLayout(w, h)

	left := styleBox.Width(leftW).Height(mainH).Render("Stream\n" + strings.Repeat("-", max(8, leftW-2)) + "\n" + m.vp.View())
	body := left
	if rightW > 0 {
		right := styleBox.Width(rightW).Height(mainH).Render(m.rightPane(rightW))
		sep := styleSep.Render(verticalSep(mainH + 2))
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, sep, right)
	}

	inputStyle := styleIn
	prefix := styleAccent.Render("❯") + " "
	if m.busy {
		inputStyle = inputStyle.BorderForeground(colorBorder)
		prefix = m.spin.View() + " "
	}
	in := inputStyle.Width(w).Render(prefix + m.in.View())
	foot := styleFoot.Width(w).Render("Two split layout • Dark purple theme • /plan /divide /work • Esc quit")

	return styleRoot.Render(head + "\n" + body + "\n" + in + "\n" + foot)
}

func panelLayout(w, h int) (leftW, rightW, mainH int) {
	mainH = max(8, h-7)

	// Avoid broken wrapping on narrow terminals: collapse to one column.
	if w < 90 {
		leftW = max(28, w)
		rightW = 0
		return leftW, rightW, mainH
	}

	leftW = (w * 68) / 100
	if leftW < 50 {
		leftW = 50
	}
	rightW = w - leftW - 1
	if rightW < 30 {
		rightW = 30
		leftW = w - rightW - 1
	}
	return leftW, rightW, mainH
}

func verticalSep(lines int) string {
	if lines <= 0 {
		return ""
	}
	var b strings.Builder
	for i := 0; i < lines; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("│")
	}
	return b.String()
}

func (m model) rightPane(width int) string {
	var b strings.Builder
	b.WriteString("Agents\n")
	b.WriteString(strings.Repeat("-", max(8, width-2)) + "\n")
	names := make([]string, 0, len(m.agents))
	for n := range m.agents {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		b.WriteString(agentLabel(n) + " " + statusLabel(m.agents[n]) + "\n")
	}
	b.WriteString("\nTask Board\n")
	b.WriteString(strings.Repeat("-", max(8, width-2)) + "\n")
	if len(m.tasks) == 0 {
		b.WriteString(styleSubtle.Render("No tasks yet. Run /divide first."))
		return b.String()
	}
	for _, t := range m.tasks {
		b.WriteString(styleAccent.Render(t.id) + " " + taskBadge(t.status) + "\n")
		b.WriteString(t.title + "\n")
		b.WriteString(styleSubtle.Render("owner: "+t.owner) + "\n")
		if len(t.deps) == 0 {
			b.WriteString(styleSubtle.Render("deps: none") + "\n\n")
		} else {
			b.WriteString(styleSubtle.Render("deps: "+strings.Join(t.deps, ", ")) + "\n\n")
		}
	}
	return b.String()
}

func taskBadge(st taskStatus) string {
	switch st {
	case tsRun:
		return styleAccent.Render(string(st))
	case tsDone:
		return styleOK.Render(string(st))
	default:
		return styleSubtle.Render(string(st))
	}
}

func statusLabel(s string) string {
	switch s {
	case "running", "active":
		return styleAccent.Render(s)
	case "error":
		return styleErr.Render(s)
	default:
		return styleSubtle.Render(s)
	}
}

func agentLabel(name string) string {
	s := lipgloss.NewStyle().Bold(true)
	switch name {
	case "judge":
		return s.Foreground(colorGreen).Render("[judge]")
	case "claude":
		return s.Foreground(colorAmber).Render("[claude]")
	case "opencode":
		return s.Foreground(colorBlue).Render("[opencode]")
	default:
		return s.Foreground(colorSubtle).Render("[agent]")
	}
}

func tickStep(step flowStep) tea.Cmd {
	return tea.Tick(step.delay, func(time.Time) tea.Msg { return flowMsg{step: step} })
}

func banner() string {
	return styleAccent.Render(`   █████╗ ██████╗  ██████╗ ███████╗
  ██╔══██╗██╔══██╗██╔═══██╗██╔════╝
  ███████║██████╔╝██║   ██║███████╗
  ██╔══██║██╔══██╗██║   ██║╚════██║
  ██║  ██║██║  ██║╚██████╔╝███████║
  ╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝ ╚══════╝`)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	p := tea.NewProgram(newModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
