package llm

import (
	"encoding/json"
	"testing"
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
