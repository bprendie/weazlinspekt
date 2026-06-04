package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bprendie/weazlinspekt/internal/config"
)

func TestTabTogglesInspektMode(t *testing.T) {
	m := New(config.Default(), "", nil, nil).(model)
	m.mode = modeChat

	updated, _, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyTab})
	if !handled {
		t.Fatal("tab key was not handled")
	}
	next := updated.(model)
	if !next.inspektMode {
		t.Fatal("inspekt mode was not enabled")
	}

	updated, _, handled = next.handleGlobalKey(tea.KeyMsg{Type: tea.KeyTab})
	if !handled {
		t.Fatal("second tab key was not handled")
	}
	next = updated.(model)
	if next.inspektMode {
		t.Fatal("inspekt mode was not disabled")
	}
}

func TestWeazlArtScalesAndColorizes(t *testing.T) {
	source := strings.Split(inspektWeazlArt, "\n")
	scaled := scaleWeazlArt(source, inspektWeazlScale)
	if len(scaled) >= len(source) {
		t.Fatalf("scaled height = %d, want less than %d", len(scaled), len(source))
	}
	if lipgloss.Width(scaled[0]) >= lipgloss.Width(source[0]) {
		t.Fatalf("scaled width = %d, want less than %d", lipgloss.Width(scaled[0]), lipgloss.Width(source[0]))
	}

	if weazlRuneStyle('▓').GetForeground() == weazlRuneStyle('▒').GetForeground() {
		t.Fatal("density glyphs should use distinct colors")
	}
	colored := colorWeazlLine("█▓▒░╣▀M")
	if lipgloss.Width(colored) != 7 {
		t.Fatalf("colored width = %d, want 7", lipgloss.Width(colored))
	}
}
