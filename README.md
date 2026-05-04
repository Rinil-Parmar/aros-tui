# AROS CLI TUI

Minimal, professional TUI prototype for the AROS multi-agent AI orchestrator.
Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Lip Gloss](https://github.com/charmbracelet/lipgloss).

## Run

```bash
cd aros-tui
go run .
```

## Layout

```
◆ AROS │ project-name             PLAN ── DIVIDE ── WORK
────────────────────────────────────────────────────────
                               │ ─── Agents ───────────
  Scrollable log/output        │ ● claude   sonnet-4-5 ★
  (viewport)                   │ ● copilot  sonnet-4.5
                               │ ● opencode gpt-4o-mini
                               │
                               │ ─── Tasks ────────────
                               │ ● task-001 ...
                               │
                               │ ─── Info ─────────────
                               │   phase   idle
                               │   memory  ● connected
╭──────────────────────────────────────────────────────╮
│ ❯ type a command...                                  │
╰──────────────────────────────────────────────────────╯
Ctrl+S status · Ctrl+C quit · pgup/dn scroll
```

## Commands

| Command | Description |
|---------|-------------|
| `<name>` | Initialize project (when no project set) |
| `plan "desc"` | Start plan phase — agents propose, judge merges |
| `y` / `n` | Approve or reject plan |
| `divide` | Break plan into dependency-safe tasks |
| `work` | Execute tasks — agents work in parallel |
| `status` | Show task board (also `Ctrl+S`) |
| `chat <msg>` | Chat with the active agent |
| `help` | Show all commands |
| `clear` | Clear the log |
| `quit` | Exit (or `Ctrl+C` / `Esc`) |

## Demo Flow

1. Type a project name → initializes project
2. `plan "build a REST API"` → 3 agents propose, judge synthesizes
3. `y` → approve → auto-divides into tasks
4. `work` → agents execute tasks with live progress
5. `Ctrl+S` → view task board with dependency graph

## Files

| File | Purpose |
|------|---------|
| `main.go` | Model, update logic, command handler, flow engine |
| `ui.go` | View rendering — header, body, sidebar, input, footer |
| `theme.go` | Color palette + pre-computed styles |
