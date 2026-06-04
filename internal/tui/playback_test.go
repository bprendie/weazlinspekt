package tui

import (
	"testing"

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
