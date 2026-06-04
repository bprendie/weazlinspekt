package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/bprendie/weazlinspekt/internal/llm"
)

func TestPlaybackCursorControlsVisibleText(t *testing.T) {
	m := model{playback: newPlaybackState(true)}
	m.appendPlayback("hello", []llm.LogprobFrame{{Token: "hello"}})
	m.appendPlayback(" world", []llm.LogprobFrame{{Token: " world"}})

	if m.streamText != "hello" {
		t.Fatalf("streamText = %q, want first token only", m.streamText)
	}
	if len(m.playback.tokens) != 2 {
		t.Fatalf("tokens = %d, want 2", len(m.playback.tokens))
	}
	m.advancePlayback()
	m.applyPlaybackCursor()
	if m.streamText != "hello world" {
		t.Fatalf("advanced streamText = %q, want full text", m.streamText)
	}
	m.rewindPlayback()
	m.applyPlaybackCursor()
	if m.streamText != "hello" {
		t.Fatalf("rewound streamText = %q, want first token only", m.streamText)
	}
}

func TestPlaybackFullTextUsesCapturedTokens(t *testing.T) {
	m := model{playback: newPlaybackState(true)}
	m.appendPlayback("a", []llm.LogprobFrame{{Token: "a"}})
	m.appendPlayback("b", []llm.LogprobFrame{{Token: "b"}})
	m.appendPlayback("c", []llm.LogprobFrame{{Token: "c"}})

	if m.streamText != "a" {
		t.Fatalf("visible streamText = %q, want cursor prefix", m.streamText)
	}
	if got := m.playbackFullText(); got != "abc" {
		t.Fatalf("playbackFullText = %q, want full capture", got)
	}
}

func TestPlaybackKeepsHesitationMarkers(t *testing.T) {
	m := model{playback: newPlaybackState(true)}
	m.appendPlayback("maybe", []llm.LogprobFrame{{
		Token: "maybe",
		Alternatives: []llm.TokenProbability{
			{Token: "maybe", Probability: 51},
			{Token: "perhaps", Probability: 49},
		},
	}})
	m.appendPlayback(" done", []llm.LogprobFrame{{
		Token: " done",
		Alternatives: []llm.TokenProbability{
			{Token: " done", Probability: 99},
			{Token: ".", Probability: 1},
		},
	}})
	m.advancePlayback()
	out := m.playbackText(true)
	if lipgloss.Width(out) != len("maybe done") {
		t.Fatalf("visible width = %d, want %d", lipgloss.Width(out), len("maybe done"))
	}
	if !strings.Contains(out, "maybe") || !strings.Contains(out, " done") {
		t.Fatalf("playback text missing tokens: %q", out)
	}
}
