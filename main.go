package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Screens ────────────────────────────────────────────────────────────────────

type screenID int

const (
	scrHome screenID = iota
	scrWork
	scrStatus
)

// ── Agent / Task types ─────────────────────────────────────────────────────────

type agentState struct {
	name, model, status string
	isJudge             bool
}

type taskState struct {
	id, title, owner string
	deps             []string
	status           string // pending, running, done
	progress         float64
}

// ── Flow step for scripted demo ────────────────────────────────────────────────

type flowStep struct {
	delay       time.Duration
	agent, text string
	action      string
	done        bool
}
type flowTick struct{ step flowStep }

func scheduleStep(s flowStep) tea.Cmd {
	return tea.Tick(s.delay, func(time.Time) tea.Msg { return flowTick{s} })
}

// ── Log line ───────────────────────────────────────────────────────────────────

type logKind int

const (
	logSys logKind = iota
	logUser
	logAgent
	logOk
	logErr
)

type logLine struct {
	kind  logKind
	agent string
	text  string
}

// ── Model ──────────────────────────────────────────────────────────────────────

type model struct {
	w, h   int
	screen screenID

	in   textinput.Model
	vp   viewport.Model
	spin spinner.Model

	project string
	phase   string // idle, plan, divide, work
	busy    bool

	log    []logLine
	tasks  []taskState
	agents []agentState
	queue  []flowStep
}

func newModel() model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = `enter project name or command (try: help)`
	ti.TextStyle = lipgloss.NewStyle().Foreground(cText)
	ti.PlaceholderStyle = sDim
	ti.Cursor.Style = sPrimary
	ti.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = sPrimary

	vp := viewport.New(80, 20)

	m := model{
		screen:  scrHome,
		in:      ti,
		spin:    sp,
		vp:      vp,
		phase:   "idle",
		project: "",
		agents: []agentState{
			{"claude", "sonnet-4-5", "ready", true},
			{"copilot", "sonnet-4.5", "ready", false},
			{"opencode", "gpt-4o-mini", "ready", false},
		},
	}

	m.pushLog(logSys, "", banner())
	m.pushLog(logSys, "", sPrimary.Render("AROS CLI")+" — Multi-agent AI orchestrator  •  v0.1.0")
	m.pushLog(logSys, "", "")
	m.pushLog(logSys, "", sMute.Render("Enter a project name to init, or type ")+sPrimary.Render("help"))
	m.syncViewport()
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, textinput.Blink)
}

// ── Update ─────────────────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.relayout()
		m.syncViewport()

	case spinner.TickMsg:
		var c tea.Cmd
		m.spin, c = m.spin.Update(msg)
		cmds = append(cmds, c)

	case flowTick:
		m.applyFlow(msg.step)
		m.syncViewport()
		if len(m.queue) > 0 {
			next := m.queue[0]
			m.queue = m.queue[1:]
			cmds = append(cmds, scheduleStep(next))
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.screen != scrHome {
				m.screen = scrHome
				return m, nil
			}
			return m, tea.Quit
		case "ctrl+s":
			m.screen = scrStatus
			return m, nil
		case "pgup", "pgdown", "up", "down":
			var c tea.Cmd
			m.vp, c = m.vp.Update(msg)
			return m, c
		}

		if msg.Type == tea.KeyEnter {
			raw := strings.TrimSpace(m.in.Value())
			m.in.SetValue("")
			if raw != "" {
				m.pushLog(logUser, "", raw)
				m.handleCmd(raw)
				m.syncViewport()
			}
			if len(m.queue) > 0 && m.busy {
				next := m.queue[0]
				m.queue = m.queue[1:]
				cmds = append(cmds, scheduleStep(next))
			}
			return m, tea.Batch(cmds...)
		}

		var c tea.Cmd
		m.in, c = m.in.Update(msg)
		cmds = append(cmds, c)
	}

	return m, tea.Batch(cmds...)
}

// ── Command handler ────────────────────────────────────────────────────────────

func (m *model) handleCmd(raw string) {
	cmd := strings.ToLower(strings.TrimSpace(raw))

	switch {
	case cmd == "help" || cmd == "/help":
		m.pushLog(logSys, "", "")
		m.pushLog(logSys, "", sPrimary.Render("Commands:"))
		m.pushLog(logSys, "", "  "+sKey.Render("plan <desc>")+"   Start planning phase")
		m.pushLog(logSys, "", "  "+sKey.Render("divide")+"        Break plan into tasks")
		m.pushLog(logSys, "", "  "+sKey.Render("work")+"          Execute tasks")
		m.pushLog(logSys, "", "  "+sKey.Render("status")+"        Show task board")
		m.pushLog(logSys, "", "  "+sKey.Render("chat <msg>")+"    Chat with agent")
		m.pushLog(logSys, "", "  "+sKey.Render("clear")+"         Clear log")
		m.pushLog(logSys, "", "  "+sKey.Render("quit")+"          Exit")

	case cmd == "clear" || cmd == "/clear":
		m.log = nil

	case cmd == "quit" || cmd == "/quit":
		m.pushLog(logSys, "", "Press Ctrl+C or Esc to quit.")

	case cmd == "status" || cmd == "/status":
		m.screen = scrStatus

	case strings.HasPrefix(cmd, "plan") || strings.HasPrefix(cmd, "/plan"):
		if m.busy {
			m.pushLog(logErr, "", "Workflow already running.")
			return
		}
		m.phase = "plan"
		m.busy = true
		m.screen = scrHome
		m.setAgents("running")
		m.pushLog(logSys, "", "")
		m.pushLog(logSys, "", sPrimary.Render("── Plan Phase ──"))
		m.queue = []flowStep{
			{400 * time.Millisecond, "claude", "Analyzing requirements… 4-layer arch: storage → service → http → cli", "", false},
			{500 * time.Millisecond, "copilot", "Proposing Postgres + golang-migrate + JWT auth scaffold", "", false},
			{400 * time.Millisecond, "opencode", "Lean approach: single-binary, BoltDB, stdlib net/http", "", false},
			{600 * time.Millisecond, "claude", "◆ Judge synthesis: SQLite + chi router, JWT deferred to v2", "", false},
			{300 * time.Millisecond, "", "Plan synthesized. Type "+sPrimary.Render("y")+" to approve or "+sPrimary.Render("n")+" to revise.", "plan_done", false},
		}

	case cmd == "y" || cmd == "yes":
		if m.phase == "plan" {
			m.pushLog(logOk, "", "Plan approved ✓")
			m.phase = "divide"
			m.busy = true
			m.setAgents("ready")
			m.setAgent("claude", "running")
			m.pushLog(logSys, "", "")
			m.pushLog(logSys, "", sPrimary.Render("── Divide Phase ──"))
			m.queue = []flowStep{
				{500 * time.Millisecond, "claude", "Generating dependency-safe task manifest…", "mkTasks", false},
				{300 * time.Millisecond, "", "3 tasks created. Type "+sPrimary.Render("work")+" to start execution.", "divide_done", false},
			}
		} else {
			m.pushLog(logSys, "", "Nothing to approve.")
		}

	case cmd == "n" || cmd == "no":
		if m.phase == "plan" {
			m.pushLog(logSys, "", "Plan rejected. Type "+sPrimary.Render("plan <desc>")+" to re-plan.")
			m.phase = "idle"
			m.busy = false
			m.setAgents("ready")
		}

	case cmd == "divide" || cmd == "/divide":
		if m.busy {
			m.pushLog(logErr, "", "Workflow already running.")
			return
		}
		m.phase = "divide"
		m.busy = true
		m.setAgent("claude", "running")
		m.pushLog(logSys, "", "")
		m.pushLog(logSys, "", sPrimary.Render("── Divide Phase ──"))
		m.queue = []flowStep{
			{500 * time.Millisecond, "claude", "Generating dependency-safe task manifest…", "mkTasks", false},
			{300 * time.Millisecond, "", "3 tasks created. Type "+sPrimary.Render("work")+" to start.", "divide_done", false},
		}

	case cmd == "work" || cmd == "/work":
		if m.busy {
			m.pushLog(logErr, "", "Workflow already running.")
			return
		}
		if len(m.tasks) == 0 {
			m.pushLog(logErr, "", "No tasks. Run "+sPrimary.Render("divide")+" first.")
			return
		}
		m.phase = "work"
		m.busy = true
		m.screen = scrWork
		m.setAgent("claude", "running")
		m.setAgent("opencode", "running")
		m.pushLog(logSys, "", "")
		m.pushLog(logSys, "", sPrimary.Render("── Work Phase ──"))
		m.queue = []flowStep{
			{600 * time.Millisecond, "claude", "[task-001] Defining data models… store.go + types", "t1_prog", false},
			{800 * time.Millisecond, "claude", "[task-001] ✓ Complete — 124 lines, 3 types", "t1_done", false},
			{500 * time.Millisecond, "opencode", "[task-002] Building storage layer… sqlite.go", "t2_prog", false},
			{700 * time.Millisecond, "opencode", "[task-002] Running tests… 4/4 pass", "t2_prog2", false},
			{500 * time.Millisecond, "opencode", "[task-002] ✓ Complete — 287 lines", "t2_done", false},
			{400 * time.Millisecond, "copilot", "[task-003] Writing docs + smoke tests…", "t3_prog", false},
			{600 * time.Millisecond, "copilot", "[task-003] ✓ Complete — README + test client", "t3_done", false},
			{300 * time.Millisecond, "", "All tasks complete ✓", "all_done", true},
		}

	case strings.HasPrefix(cmd, "chat") || strings.HasPrefix(cmd, "/chat"):
		msg := strings.TrimPrefix(strings.TrimPrefix(cmd, "/chat"), "chat")
		msg = strings.TrimSpace(msg)
		if msg == "" {
			msg = "Hello, what can you help me with?"
		}
		m.pushLog(logSys, "", "")
		m.pushLog(logAgent, "claude", "I can help you with planning, coding, and debugging. What would you like to work on?")

	default:
		// If no project, treat as project name
		if m.project == "" {
			m.project = raw
			m.pushLog(logSys, "", "")
			m.pushLog(logOk, "", "Project "+sPrimary.Render(m.project)+" initialized ✓")
			m.pushLog(logSys, "", sMute.Render("Config: ")+"~/.aros/config.toml")
			m.pushLog(logSys, "", sMute.Render("Judge:  ")+sClaude.Render("claude")+" · "+sMute.Render("Memory: ")+sOk.Render("connected"))
			m.pushLog(logSys, "", "")
			m.pushLog(logSys, "", sMute.Render("Ready. Try ")+sPrimary.Render(`plan "build a REST API"`))
			m.in.Placeholder = `type command (try: plan "build a REST API")`
		} else {
			m.pushLog(logErr, "", "Unknown command. Type "+sPrimary.Render("help")+" for available commands.")
		}
	}
}

// ── Flow step handler ──────────────────────────────────────────────────────────

func (m *model) applyFlow(s flowStep) {
	if s.agent != "" {
		m.pushLog(logAgent, s.agent, s.text)
	} else {
		m.pushLog(logOk, "", s.text)
	}

	switch s.action {
	case "plan_done":
		m.setAgents("ready")
		m.busy = false
	case "divide_done":
		m.setAgents("ready")
		m.busy = false
	case "mkTasks":
		m.tasks = []taskState{
			{"task-001", "Define data models", "claude", nil, "pending", 0},
			{"task-002", "Build storage layer", "opencode", []string{"task-001"}, "pending", 0},
			{"task-003", "Finalize + docs", "copilot", []string{"task-002"}, "pending", 0},
		}
	case "t1_prog":
		m.setTask("task-001", "running", 0.5)
	case "t1_done":
		m.setTask("task-001", "done", 1.0)
		m.setTask("task-002", "running", 0.0)
	case "t2_prog":
		m.setTask("task-002", "running", 0.3)
	case "t2_prog2":
		m.setTask("task-002", "running", 0.7)
	case "t2_done":
		m.setTask("task-002", "done", 1.0)
		m.setTask("task-003", "running", 0.0)
		m.setAgent("opencode", "ready")
		m.setAgent("copilot", "running")
	case "t3_prog":
		m.setTask("task-003", "running", 0.5)
	case "t3_done":
		m.setTask("task-003", "done", 1.0)
	case "all_done":
		// done flag on flowStep handles reset
	}

	if s.done {
		m.phase = "idle"
		m.busy = false
		m.setAgents("ready")
	}
}

// ── Helpers ────────────────────────────────────────────────────────────────────

func (m *model) pushLog(k logKind, agent, text string) {
	m.log = append(m.log, logLine{k, agent, text})
}

func (m *model) syncViewport() {
	var lines []string
	for _, l := range m.log {
		lines = append(lines, renderLogLine(l))
	}
	m.vp.SetContent(strings.Join(lines, "\n"))
	m.vp.GotoBottom()
}

func (m *model) relayout() {
	_, _, mainH := layout(m.w, m.h)
	leftW, _, _ := layout(m.w, m.h)
	m.vp.Width = max(20, leftW-4)
	m.vp.Height = max(4, mainH-2)
	m.in.Width = max(20, m.w-8)
}

func (m *model) setAgents(status string) {
	for i := range m.agents {
		m.agents[i].status = status
	}
}

func (m *model) setAgent(name, status string) {
	for i := range m.agents {
		if m.agents[i].name == name {
			m.agents[i].status = status
		}
	}
}

func (m *model) setTask(id, status string, progress float64) {
	for i := range m.tasks {
		if m.tasks[i].id == id {
			m.tasks[i].status = status
			m.tasks[i].progress = progress
		}
	}
}

func renderLogLine(l logLine) string {
	switch l.kind {
	case logUser:
		return sPrimary.Render("❯ ") + sText.Render(l.text)
	case logAgent:
		return agentSty(l.agent).Render("["+l.agent+"]") + " " + l.text
	case logOk:
		return sOk.Render("✓ ") + l.text
	case logErr:
		return sErr.Render("✗ ") + l.text
	default:
		return l.text
	}
}

func layout(w, h int) (leftW, rightW, mainH int) {
	mainH = max(6, h-5)
	if w < 80 {
		return max(20, w-2), 0, mainH
	}
	rightW = 32
	if w > 140 {
		rightW = 38
	}
	leftW = w - rightW - 3
	return
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func banner() string {
	return sPrimary.Render(`  █████╗ ██████╗  ██████╗ ███████╗
 ██╔══██╗██╔══██╗██╔═══██╗██╔════╝
 ███████║██████╔╝██║   ██║███████╗
 ██╔══██║██╔══██╗██║   ██║╚════██║
 ██║  ██║██║  ██║╚██████╔╝███████║
 ╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝ ╚══════╝`)
}

func main() {
	p := tea.NewProgram(newModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
