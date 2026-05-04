package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ── View dispatch ──────────────────────────────────────────────────────────────

func (m model) View() string {
	w := m.w
	if w == 0 {
		w = 120
	}
	h := m.h
	if h == 0 {
		h = 36
	}

	head := renderHead(m.project, m.phase, w)
	body := m.renderBody(w, h)
	input := m.renderInput(w)
	foot := renderFoot(m.screen, w)

	return head + "\n" + body + "\n" + input + "\n" + foot
}

// ── Header ─────────────────────────────────────────────────────────────────────

func renderHead(project, phase string, w int) string {
	diamond := sPrimary.Render("◆")
	brand := sPrimary.Render("AROS")
	sep := sDim.Render("│")

	proj := sText.Render(project)
	if project == "" {
		proj = sDim.Render("(no project)")
	}

	left := diamond + " " + brand + " " + sep + " " + proj

	// Phase pills
	phases := []string{"plan", "divide", "work"}
	var pills []string
	active := false
	for _, p := range phases {
		if p == phase {
			active = true
			pills = append(pills, sPrimary.Render("● "+strings.ToUpper(p)))
		} else if !active {
			pills = append(pills, sOk.Render("✓ "+strings.ToUpper(p)))
		} else {
			pills = append(pills, sDim.Render("  "+strings.ToUpper(p)))
		}
	}
	if phase == "idle" {
		pills = nil
		for _, p := range phases {
			pills = append(pills, sDim.Render("  "+strings.ToUpper(p)))
		}
	}
	right := strings.Join(pills, sDim.Render(" ── "))

	gap := w - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}

	row := left + strings.Repeat(" ", gap) + right
	border := sDim.Render(strings.Repeat("─", max(1, w)))
	return row + "\n" + border
}

// ── Body ───────────────────────────────────────────────────────────────────────

func (m model) renderBody(w, h int) string {
	_, rightW, mainH := layout(w, h)

	// Left: log viewport
	left := m.vp.View()

	// Pad left to fill height
	leftLines := strings.Count(left, "\n") + 1
	if leftLines < mainH {
		left += strings.Repeat("\n", mainH-leftLines)
	}

	if rightW <= 0 {
		return left
	}

	// Right sidebar
	right := m.renderSidebar(rightW, mainH)

	// Vertical separator
	var sepLines []string
	for i := 0; i < mainH; i++ {
		sepLines = append(sepLines, sDim.Render("│"))
	}
	sep := strings.Join(sepLines, "\n")

	return lipgloss.JoinHorizontal(lipgloss.Top, left, " "+sep+" ", right)
}

// ── Sidebar ────────────────────────────────────────────────────────────────────

func (m model) renderSidebar(w, h int) string {
	var lines []string

	// ── Agents section
	lines = append(lines, sMute.Render("─── Agents ")+sDim.Render(strings.Repeat("─", max(1, w-12))))
	for _, a := range m.agents {
		dot := "●"
		var dotStyle lipgloss.Style
		switch a.status {
		case "running":
			dotStyle = agentSty(a.name)
		case "ready":
			dotStyle = agentSty(a.name)
		default:
			dotStyle = sDim
		}

		name := agentSty(a.name).Render(a.name)
		model := sMute.Render(a.model)

		row := dotStyle.Render(dot) + " " + pad(name, 10) + " " + model
		if a.isJudge {
			row += " " + sPrimary.Render("★")
		}
		if a.status == "running" {
			row += " " + sPrimary.Render("⠋")
		}
		lines = append(lines, row)
	}

	// ── Tasks section
	lines = append(lines, "")
	if m.screen == scrStatus || m.screen == scrWork || len(m.tasks) > 0 {
		lines = append(lines, sMute.Render("─── Tasks ")+sDim.Render(strings.Repeat("─", max(1, w-11))))
		if len(m.tasks) == 0 {
			lines = append(lines, sDim.Render("  No tasks yet"))
		}
		for _, t := range m.tasks {
			icon := "○"
			var iconStyle lipgloss.Style
			switch t.status {
			case "done":
				icon = "✓"
				iconStyle = sOk
			case "running":
				icon = "●"
				iconStyle = sPrimary
			default:
				iconStyle = sDim
			}

			id := sDim.Render(t.id)
			title := sText.Render(t.title)
			owner := agentSty(t.owner).Render(t.owner)

			lines = append(lines, iconStyle.Render(icon)+" "+id)
			lines = append(lines, "  "+truncStr(title, w-4))
			lines = append(lines, "  "+owner)

			if t.status == "running" && t.progress > 0 {
				bar := progressBar(t.progress, max(6, w-6))
				lines = append(lines, "  "+bar)
			}
			lines = append(lines, "")
		}

		// Dependency graph (compact)
		if len(m.tasks) > 0 {
			lines = append(lines, sDim.Render("─── Deps ")+sDim.Render(strings.Repeat("─", max(1, w-10))))
			for _, t := range m.tasks {
				var deps string
				if len(t.deps) == 0 {
					deps = sDim.Render("none")
				} else {
					deps = sMute.Render(strings.Join(t.deps, ", "))
				}
				lines = append(lines, "  "+sDim.Render(t.id)+" → "+deps)
			}
		}
	} else {
		lines = append(lines, sMute.Render("─── Tasks ")+sDim.Render(strings.Repeat("─", max(1, w-11))))
		lines = append(lines, sDim.Render("  No tasks yet"))
		lines = append(lines, sDim.Render("  Run ")+sPrimary.Render("divide")+sDim.Render(" first"))
	}

	// ── Project info
	lines = append(lines, "")
	lines = append(lines, sMute.Render("─── Info ")+sDim.Render(strings.Repeat("─", max(1, w-10))))
	if m.project != "" {
		lines = append(lines, "  "+sDim.Render("project ")+" "+sText.Render(m.project))
	}
	lines = append(lines, "  "+sDim.Render("phase   ")+" "+phaseLabel(m.phase))
	lines = append(lines, "  "+sDim.Render("memory  ")+" "+sOk.Render("● connected"))

	content := strings.Join(lines, "\n")

	// Pad to fill height
	contentLines := strings.Count(content, "\n") + 1
	if contentLines < h {
		content += strings.Repeat("\n", h-contentLines)
	}

	return content
}

// ── Input ──────────────────────────────────────────────────────────────────────

func (m model) renderInput(w int) string {
	var prefix string
	if m.busy {
		prefix = m.spin.View() + " "
	} else {
		prefix = sPrimary.Render("❯") + " "
	}

	content := prefix + m.in.View()
	border := sActiveBox.Width(max(20, w-2))
	if m.busy {
		border = sBox.Width(max(20, w-2))
	}
	return border.Render(content)
}

// ── Footer ─────────────────────────────────────────────────────────────────────

func renderFoot(scr screenID, w int) string {
	var items []string
	items = append(items, sKey.Render("Ctrl+S")+sMute.Render(" status"))
	items = append(items, sKey.Render("Ctrl+C")+sMute.Render(" quit"))
	if scr != scrHome {
		items = append(items, sKey.Render("Esc")+sMute.Render(" back"))
	}
	items = append(items, sKey.Render("pgup/dn")+sMute.Render(" scroll"))
	return sFooter.Render(strings.Join(items, "  "+sDim.Render("·")+"  "))
}

// ── Helpers ────────────────────────────────────────────────────────────────────

func progressBar(val float64, w int) string {
	filled := int(val * float64(w))
	if filled > w {
		filled = w
	}
	bar := sPrimary.Render(strings.Repeat("█", filled))
	bar += sDim.Render(strings.Repeat("░", w-filled))
	bar += sMute.Render(fmt.Sprintf(" %d%%", int(val*100)))
	return bar
}

func phaseLabel(phase string) string {
	switch phase {
	case "plan":
		return sPrimary.Render("plan")
	case "divide":
		return sWarn.Render("divide")
	case "work":
		return sOk.Render("work")
	default:
		return sDim.Render("idle")
	}
}

func pad(s string, w int) string {
	vw := lipgloss.Width(s)
	if vw >= w {
		return s
	}
	return s + strings.Repeat(" ", w-vw)
}

func truncStr(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	if w <= 2 {
		return s[:w]
	}
	return s[:w-1] + "…"
}
