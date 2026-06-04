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

func TestPlaybackKeysAreInspektorOnly(t *testing.T) {
	m := New(config.Default(), "", nil, nil).(model)
	m.mode = modeChat
	if _, _, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("'")}); handled {
		t.Fatal("playback key should not be handled outside Inspektor mode")
	}

	m.inspektMode = true
	m.playback = newPlaybackState(true)
	m.appendPlayback("a", nil)
	m.appendPlayback("b", nil)
	updated, _, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	if !handled {
		t.Fatal("step forward key was not handled in Inspektor mode")
	}
	next := updated.(model)
	if next.playback.cursor != 1 || next.streamText != "ab" {
		t.Fatalf("cursor/text = %d/%q, want 1/ab", next.playback.cursor, next.streamText)
	}
}

func TestWeazlArtKeepsSizeAndColorizes(t *testing.T) {
	source := strings.Split(inspektWeazlArt, "\n")
	clipped := clippedWeazl(200, 200)
	if len(clipped) != len(source) {
		t.Fatalf("clipped height = %d, want %d", len(clipped), len(source))
	}
	if lipgloss.Width(clipped[0].text) != lipgloss.Width(source[0]) {
		t.Fatalf("clipped width = %d, want %d", lipgloss.Width(clipped[0].text), lipgloss.Width(source[0]))
	}

	if fallbackWeazlRuneStyle('▓').GetForeground() == fallbackWeazlRuneStyle('▒').GetForeground() {
		t.Fatal("density glyphs should use distinct colors")
	}
	if fallbackWeazlRuneStyle('█').GetForeground() != lipgloss.Color("235") {
		t.Fatal("solid block should use ANSI 256-color palette index 235")
	}
	colored := colorWeazlLine("█▓▒░╣▀M", 0, 0)
	if lipgloss.Width(colored) != 7 {
		t.Fatalf("colored width = %d, want 7", lipgloss.Width(colored))
	}
	if weazlCellStyle(0, 0, '█').GetForeground() == fallbackWeazlRuneStyle('█').GetForeground() {
		t.Fatal("PNG palette should override fallback glyph colors")
	}
}
