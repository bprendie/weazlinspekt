package tui

import (
	"strings"
	"testing"

	"github.com/bprendie/weazlinspekt/internal/config"
	"github.com/bprendie/weazlinspekt/internal/storage"
)

func TestAutoCompactThresholdScalesWithContextWindow(t *testing.T) {
	tests := []struct {
		window int
		want   int
	}{
		{window: 8192, want: 7536},
		{window: 16384, want: 13926},
		{window: 32768, want: 24576},
		{window: 128000, want: 49152},
	}
	for _, tt := range tests {
		if got := autoCompactThreshold(tt.window); got != tt.want {
			t.Fatalf("autoCompactThreshold(%d) = %d, want %d", tt.window, got, tt.want)
		}
	}
}

func TestSummaryTargetTokensScalesForLargeWindows(t *testing.T) {
	tests := []struct {
		window int
		want   int
	}{
		{window: 8192, want: 500},
		{window: 32768, want: 1365},
		{window: 128000, want: 5333},
		{window: 262144, want: 6000},
	}
	for _, tt := range tests {
		if got := summaryTargetTokens(tt.window); got != tt.want {
			t.Fatalf("summaryTargetTokens(%d) = %d, want %d", tt.window, got, tt.want)
		}
	}
}

func TestEstimateMessagesIncludesToolCalls(t *testing.T) {
	messages := []storage.Message{
		{
			Role:      "assistant",
			Content:   "",
			ToolCalls: strings.Repeat("tool ", 30),
		},
	}
	if got := estimateMessages(messages); got == 0 {
		t.Fatal("estimateMessages = 0, want tool call metadata counted")
	}
}

func TestInspektModeSkipsPromptTrim(t *testing.T) {
	history := []storage.Message{
		{Role: "user", Content: strings.Repeat("token ", 9000)},
		{Role: "assistant", Content: strings.Repeat("token ", 9000)},
	}
	cfg := config.Default()
	cfg.Providers[cfg.ActiveProvider] = config.Provider{ContextWindow: 8192}
	normal := model{cfg: cfg}
	if !normal.shouldTrimForPrompt(history) {
		t.Fatal("normal chat should trim oversized history")
	}
	inspekt := model{cfg: cfg, inspektMode: true}
	if inspekt.shouldTrimForPrompt(history) {
		t.Fatal("Inspektor mode should skip context trimming")
	}
}
