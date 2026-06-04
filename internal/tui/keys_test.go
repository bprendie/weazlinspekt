package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bprendie/weazlinspekt/internal/config"
	"github.com/bprendie/weazlinspekt/internal/llm"
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

func TestInspektPlaybackLegendShowsStepAndSpeedKeys(t *testing.T) {
	m := New(config.Default(), "", nil, nil).(model)
	if legend := m.inspektPlaybackLegend(); legend != "" {
		t.Fatalf("legend outside Inspektor = %q, want empty", legend)
	}
	m.inspektMode = true
	m.playback = newPlaybackState(true)
	legend := m.inspektPlaybackLegend()
	if !strings.Contains(legend, "[ ]") || !strings.Contains(legend, "'") {
		t.Fatalf("legend = %q, want step and speed keys", legend)
	}
	if lipgloss.Width(legend) != len("[ ] step ' speed") {
		t.Fatalf("legend width = %d, want %d", lipgloss.Width(legend), len("[ ] step ' speed"))
	}
}

func TestDieRollToggleIsInspektorOnly(t *testing.T) {
	m := New(config.Default(), "", nil, nil).(model)
	m.mode = modeChat
	m.inspektDieRoll = true
	if _, _, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlG}); handled {
		t.Fatal("die roll toggle should not be handled outside Inspektor mode")
	}

	m.inspektMode = true
	updated, _, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlG})
	if !handled {
		t.Fatal("die roll toggle was not handled in Inspektor mode")
	}
	next := updated.(model)
	if next.inspektDieRoll {
		t.Fatal("die roll should be disabled after toggle")
	}
}

func TestDieRollStateShowsToggleValue(t *testing.T) {
	m := New(config.Default(), "", nil, nil).(model)
	m.inspektMode = true
	m.playback = newPlaybackState(true)
	m.inspektDieRoll = true
	if state := m.dieRollState(); !strings.Contains(state, "die on") {
		t.Fatalf("die state = %q, want die on", state)
	}
	m.inspektDieRoll = false
	if state := m.dieRollState(); !strings.Contains(state, "die off") {
		t.Fatalf("die state = %q, want die off", state)
	}
}

func TestDieRollRankDetectsNonArgmax(t *testing.T) {
	frame := llm.LogprobFrame{
		Token: "Schedule",
		Alternatives: []llm.TokenProbability{
			{Token: "Make", Probability: 36.2},
			{Token: "Schedule", Probability: 27.8},
			{Token: "Plan", Probability: 7.1},
		},
	}
	rank, ok := dieRollRank(frame)
	if !ok || rank != 2 {
		t.Fatalf("rank/ok = %d/%v, want 2/true", rank, ok)
	}

	frame.Token = "Make"
	rank, ok = dieRollRank(frame)
	if ok || rank != 1 {
		t.Fatalf("argmax rank/ok = %d/%v, want 1/false", rank, ok)
	}
}

func TestDieRollLayerCanBeDisabled(t *testing.T) {
	m := New(config.Default(), "", nil, nil).(model)
	m.inspektMode = true
	m.inspektDieRoll = true
	m.inspektFrame = llm.LogprobFrame{
		Token: "Schedule",
		Alternatives: []llm.TokenProbability{
			{Token: "Make", Probability: 36.2},
			{Token: "Schedule", Probability: 27.8},
		},
	}
	if !m.dieRollActive() {
		t.Fatal("die roll should be active for non-argmax token")
	}
	m.inspektDieRoll = false
	if m.dieRollActive() || m.dieRollTag() != "" {
		t.Fatal("die roll should be hidden when disabled")
	}
}

func TestWeazlArtScalesAndColorizes(t *testing.T) {
	source := strings.Split(inspektWeazlArt, "\n")
	clipped := clippedWeazl(200, 200)
	wantHeight := (len(source) + weazlArtScale - 1) / weazlArtScale
	if len(clipped) != wantHeight {
		t.Fatalf("clipped height = %d, want %d", len(clipped), wantHeight)
	}
	wantWidth := (lipgloss.Width(source[0]) + weazlArtScale - 1) / weazlArtScale
	if lipgloss.Width(clipped[0].text) != wantWidth {
		t.Fatalf("clipped width = %d, want %d", lipgloss.Width(clipped[0].text), wantWidth)
	}
	if clipped[1].row != weazlArtScale {
		t.Fatalf("second clipped source row = %d, want %d", clipped[1].row, weazlArtScale)
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

func TestInspektPaneWidthReservesRenderedColumns(t *testing.T) {
	for _, total := range []int{48, 60, 80, 120} {
		chatWidth := inspektChatWidth(total)
		paneWidth := inspektPaneWidth(total, chatWidth)
		rendered := chatWidth + inspektGapWidth + paneWidth + inspektPaneOverhead
		if rendered > total {
			t.Fatalf("split renders %d columns into %d", rendered, total)
		}
		if paneWidth < inspektPaneMinWidth {
			t.Fatalf("pane width = %d, want >= %d", paneWidth, inspektPaneMinWidth)
		}
	}
}
