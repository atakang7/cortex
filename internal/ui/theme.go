package ui

import "github.com/charmbracelet/lipgloss"

var (
	colorBrand = lipgloss.AdaptiveColor{Light: "#C2410C", Dark: "#FDBA74"}
	colorMuted = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#6B7280"}
	colorFaint = lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#4B5563"}
	colorTool  = lipgloss.AdaptiveColor{Light: "#0369A1", Dark: "#7DD3FC"}
	colorOK    = lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#86EFAC"}
	colorBad   = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#FCA5A5"}

	// colorUser and colorAssistant give the two speakers in the transcript
	// their own identity, so a scroll back through a long session reads as a
	// conversation at a glance rather than a wall of one color.
	colorUser      = lipgloss.AdaptiveColor{Light: "#0E7490", Dark: "#67E8F9"}
	colorAssistant = lipgloss.AdaptiveColor{Light: "#4338CA", Dark: "#A5B4FC"}

	// colorPruner marks the curator as a distinct third voice — neither the
	// user's nor the model's — so its activity doesn't read as generic system
	// chatter.
	colorPruner = lipgloss.AdaptiveColor{Light: "#7E22CE", Dark: "#D8B4FE"}
)

var (
	styleUserMark = lipgloss.NewStyle().Foreground(colorBrand).Bold(true)

	styleUserText = lipgloss.NewStyle().Foreground(colorUser).Bold(true)

	styleAssistant = lipgloss.NewStyle().Foreground(colorAssistant)

	styleReasoning = lipgloss.NewStyle().Foreground(colorFaint).Italic(true)

	stylePruner = lipgloss.NewStyle().Foreground(colorPruner)
)

var (
	styleToolName = lipgloss.NewStyle().Foreground(colorTool).Bold(true)

	styleToolArgs = lipgloss.NewStyle().Foreground(colorMuted)

	styleToolRail = lipgloss.NewStyle().Foreground(colorFaint)

	styleToolOK = lipgloss.NewStyle().Foreground(colorOK)

	styleToolBad = lipgloss.NewStyle().Foreground(colorBad)
)

var (
	styleInputFrame = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorFaint).
			Padding(0, 1)

	styleInputFrameBusy = styleInputFrame.BorderForeground(colorBrand)

	styleStatus = lipgloss.NewStyle().Foreground(colorFaint)

	styleStatusKey = lipgloss.NewStyle().Foreground(colorMuted)

	styleBanner = lipgloss.NewStyle().Foreground(colorBrand).Bold(true)

	styleNotice = lipgloss.NewStyle().Foreground(colorMuted)

	styleError = lipgloss.NewStyle().Foreground(colorBad)

	styleSpinner = lipgloss.NewStyle().Foreground(colorBrand)
)

const (
	glyphPrompt   = "❯"
	glyphToolHead = "▐"
	glyphToolRail = "│"
	glyphToolTail = "└"
	glyphOK       = "✓"
	glyphBad      = "✗"
	glyphPrune    = "✂"
)
