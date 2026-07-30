package tui

import (
	"fmt"
	"strings"
)

// keyBinding represents a labeled keybinding for the help overlay.
type keyBinding struct {
	Key  string
	Desc string
}

// helpContent lists all available keybindings.
var helpContent = []keyBinding{
	{"↑/k", "Move up"},
	{"↓/j", "Move down"},
	{"Enter", "Confirm / select"},
	{"Space", "Toggle selection"},
	{"a", "Select all"},
	{"n", "Deselect all"},
	{"/", "Filter tracks"},
	{"Esc", "Go back / cancel"},
	{"s", "Toggle search mode"},
	{"q", "Quit"},
	{"?", "Toggle help"},
}

// helpView renders the help overlay with all available keybindings.
func helpView(width int) string {
	lines := []string{"  Keybindings:", ""}

	// Group keys and descriptions in aligned columns
	maxKeyLen := 0
	for _, kb := range helpContent {
		if len(kb.Key) > maxKeyLen {
			maxKeyLen = len(kb.Key)
		}
	}

	for _, kb := range helpContent {
		line := fmt.Sprintf("    %-*s  %s", maxKeyLen, kb.Key, kb.Desc)
		lines = append(lines, line)
	}

	lines = append(lines, "", "  Tips:")
	lines = append(lines, "    Downloads run sequentially, one track at a time.")
	lines = append(lines, "    Press 'r' on the Done screen to start a new URL.")

	return helpStyle.Render(strings.Join(lines, "\n"))
}
