package main

import "github.com/charmbracelet/lipgloss"

// ── Colors ─────────────────────────────────────────────────────────────────────
var (
	cBg      = lipgloss.Color("#0d0b14")
	cPanel   = lipgloss.Color("#13101c")
	cBorder  = lipgloss.Color("#2a2438")
	cText    = lipgloss.Color("#e8e3f4")
	cMute    = lipgloss.Color("#9a93ad")
	cDim     = lipgloss.Color("#5e576f")
	cPrimary = lipgloss.Color("#b89df0")
	cClaude  = lipgloss.Color("#f4c95d")
	cCopilot = lipgloss.Color("#7ed492")
	cOpen    = lipgloss.Color("#7ab8ff")
	cOk      = lipgloss.Color("#7ed492")
	cWarn    = lipgloss.Color("#f4c95d")
	cErr     = lipgloss.Color("#f47d7d")
)

// ── Styles (computed once) ─────────────────────────────────────────────────────
var (
	sBold    = lipgloss.NewStyle().Bold(true)
	sMute    = lipgloss.NewStyle().Foreground(cMute)
	sDim     = lipgloss.NewStyle().Foreground(cDim)
	sText    = lipgloss.NewStyle().Foreground(cText)
	sPrimary = lipgloss.NewStyle().Foreground(cPrimary).Bold(true)
	sOk      = lipgloss.NewStyle().Foreground(cOk)
	sWarn    = lipgloss.NewStyle().Foreground(cWarn)
	sErr     = lipgloss.NewStyle().Foreground(cErr).Bold(true)

	sClaude  = lipgloss.NewStyle().Foreground(cClaude).Bold(true)
	sCopilot = lipgloss.NewStyle().Foreground(cCopilot).Bold(true)
	sOpen    = lipgloss.NewStyle().Foreground(cOpen).Bold(true)

	sBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cBorder)
	sActiveBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cPrimary)
	sFooter = lipgloss.NewStyle().Foreground(cDim)
	sKey    = lipgloss.NewStyle().Foreground(cPrimary).Bold(true)
)

func agentSty(name string) lipgloss.Style {
	switch name {
	case "claude":
		return sClaude
	case "copilot":
		return sCopilot
	case "opencode":
		return sOpen
	}
	return sMute
}
