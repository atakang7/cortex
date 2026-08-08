package ui

import "github.com/charmbracelet/lipgloss"

// theme.go owns every colour and every style in cortex. Nothing else in this
// package may construct a lipgloss.Style or name a colour — if a new element
// needs styling, it gets a style here and uses it by name. One owner means
// the palette can be retuned in one place and the whole UI moves together.

// The palette. Adaptive pairs carry a light-terminal value and a dark one, so
// the UI stays legible without asking the user which theme they run.
var (
	colorBrand = lipgloss.AdaptiveColor{Light: "#C2410C", Dark: "#FDBA74"}
	colorText  = lipgloss.AdaptiveColor{Light: "#1F2937", Dark: "#E5E7EB"}
	colorMuted = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#6B7280"}
	colorFaint = lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#4B5563"}
	colorTool  = lipgloss.AdaptiveColor{Light: "#0369A1", Dark: "#7DD3FC"}
	colorOK    = lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#86EFAC"}
	colorBad   = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#FCA5A5"}
)

// Transcript styles — the committed record of the conversation.
var (
	styleUserMark = lipgloss.NewStyle().Foreground(colorBrand).Bold(true)

	styleUserText = lipgloss.NewStyle().Foreground(colorText).Bold(true)

	styleAssistant = lipgloss.NewStyle().Foreground(colorText)

	// Reasoning is the model thinking aloud. It is deliberately the least
	// legible thing on screen: present if you look for it, ignorable if not.
	styleReasoning = lipgloss.NewStyle().Foreground(colorFaint).Italic(true)
)

// Tool-card styles. A card is a bordered block: a header naming the tool, the
// arguments that were passed, and a preview of what came back.
var (
	styleToolName = lipgloss.NewStyle().Foreground(colorTool).Bold(true)

	styleToolArgs = lipgloss.NewStyle().Foreground(colorMuted)

	styleToolRail = lipgloss.NewStyle().Foreground(colorFaint)

	styleToolOK = lipgloss.NewStyle().Foreground(colorOK)

	styleToolBad = lipgloss.NewStyle().Foreground(colorBad)
)

// Chrome — the parts of the screen that are not transcript.
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

// Glyphs. Named here so the whole UI speaks one visual vocabulary, and so a
// terminal that renders one of them badly can be fixed in one edit.
const (
	glyphPrompt   = "❯"
	glyphToolHead = "▐"
	glyphToolRail = "│"
	glyphToolTail = "└"
	glyphOK       = "✓"
	glyphBad      = "✗"
)
