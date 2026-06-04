package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bprendie/weazlinspekt/internal/config"
)

func TestOpenAIChatCompletionLogprobsFixture(t *testing.T) {
	raw := []byte(`{
		"choices": [
			{
				"delta": {"content": ".town"},
				"logprobs": {
					"content": [
						{
							"token": ".town",
							"logprob": -0.0001,
							"top_logprobs": [
								{"token": ".town", "logprob": -0.0001},
								{"token": ".village", "logprob": -5.321}
							]
						}
					]
				}
			}
		]
	}`)
	var chunk openAIStreamChunk
	if err := json.Unmarshal(raw, &chunk); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	frames := chunk.Choices[0].Logprobs.frames()
	if len(frames) != 1 {
		t.Fatalf("frames = %d, want 1", len(frames))
	}
	if frames[0].Token != ".town" {
		t.Fatalf("token = %q, want .town", frames[0].Token)
	}
	if len(frames[0].Alternatives) != 2 {
		t.Fatalf("alternatives = %d, want 2", len(frames[0].Alternatives))
	}
	if frames[0].Alternatives[1].Token != ".village" {
		t.Fatalf("second token = %q, want .village", frames[0].Alternatives[1].Token)
	}
	if frames[0].Alternatives[0].Probability <= 99 {
		t.Fatalf("top probability = %.4f, want near certain", frames[0].Alternatives[0].Probability)
	}
}

func TestOpenAIStreamRequestsTopSixLogprobs(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s, want /v1/chat/completions", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("Decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := New(config.Provider{Type: "vllm", ServerURL: server.URL, Model: "test-model"})
	if _, err := client.streamOpenAICompat(context.Background(), []ChatMessage{{Role: "user", Content: "hello"}}, func(StreamEvent) {}); err != nil {
		t.Fatalf("streamOpenAICompat: %v", err)
	}
	if got := requestBody["top_logprobs"]; got != float64(topLogprobs) {
		t.Fatalf("top_logprobs = %#v, want %d", got, topLogprobs)
	}
	if got := requestBody["top_p"]; got != float64(1) {
		t.Fatalf("top_p = %#v, want 1.0", got)
	}
}
