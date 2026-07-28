package tui

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
	// {"/", "Filter tracks"}, // not implemented in MVP
	{"Esc", "Go back / cancel"},
	{"q", "Quit"},
	{"?", "Toggle help"},
}

// helpView renders a simple help overlay. The width parameter is reserved
// for future layout-aware rendering.
func helpView(width int) string {
	return ""
}
