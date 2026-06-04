package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
)

const topLogprobs = 6

func (c Client) streamOpenAICompat(ctx context.Context, messages []ChatMessage, onEvent func(StreamEvent)) (Usage, error) {
	reqBody := map[string]any{
		"model":        c.provider.Model,
		"messages":     messages,
		"temperature":  0.7,
		"stream":       true,
		"logprobs":     true,
		"top_logprobs": topLogprobs,
		"stream_options": map[string]any{
			"include_usage": true,
		},
	}

	if len(c.tools) > 0 {
		reqBody["tools"] = c.tools
		reqBody["tool_choice"] = "auto"
	}

	resp, err := c.post(ctx, "/v1/chat/completions", reqBody)
	if err != nil {
		return Usage{}, err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var usage Usage
	toolCallsMap := make(map[int]*ToolCall)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return usage, err
		}
		if chunk.Error != nil {
			return usage, errors.New(chunk.Error.Message)
		}
		if chunk.Usage != nil {
			usage.InputTokens = chunk.Usage.PromptTokens
			usage.OutputTokens = chunk.Usage.CompletionTokens
		}
		for _, choice := range chunk.Choices {
			frames := choice.Logprobs.frames()
			if choice.Delta.Content != "" || len(frames) > 0 {
				onEvent(StreamEvent{Type: "content", Content: choice.Delta.Content, Logprobs: frames})
			}
			for _, tc := range choice.Delta.ToolCalls {
				if _, exists := toolCallsMap[tc.Index]; !exists {
					toolCallsMap[tc.Index] = &ToolCall{ID: tc.ID, Type: tc.Type}
				}
				call := toolCallsMap[tc.Index]
				if tc.Function.Name != "" {
					call.Function.Name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					call.Function.Arguments += tc.Function.Arguments
				}
			}
			if choice.FinishReason == "tool_calls" && len(toolCallsMap) > 0 {
				onEvent(StreamEvent{Type: "tool_call", ToolCalls: orderedToolCalls(toolCallsMap)})
			}
		}
	}
	return usage, scanner.Err()
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string          `json:"finish_reason"`
		Logprobs     logprobsPayload `json:"logprobs"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type logprobsPayload struct {
	Content []tokenLogprob `json:"content"`
}

type tokenLogprob struct {
	Token       string            `json:"token"`
	Logprob     float64           `json:"logprob"`
	TopLogprobs []topTokenLogprob `json:"top_logprobs"`
}

type topTokenLogprob struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
}

func (p logprobsPayload) frames() []LogprobFrame {
	if len(p.Content) == 0 {
		return nil
	}
	frames := make([]LogprobFrame, 0, len(p.Content))
	for _, item := range p.Content {
		alts := make([]TokenProbability, 0, len(item.TopLogprobs))
		for _, top := range item.TopLogprobs {
			alts = append(alts, TokenProbability{
				Token:       top.Token,
				Logprob:     top.Logprob,
				Probability: math.Exp(top.Logprob) * 100,
			})
		}
		if len(alts) == 0 && item.Token != "" {
			alts = append(alts, TokenProbability{
				Token:       item.Token,
				Logprob:     item.Logprob,
				Probability: math.Exp(item.Logprob) * 100,
			})
		}
		if len(alts) > 0 {
			frames = append(frames, LogprobFrame{Token: item.Token, Alternatives: alts})
		}
	}
	return frames
}

func orderedToolCalls(calls map[int]*ToolCall) []ToolCall {
	toolCalls := make([]ToolCall, 0, len(calls))
	for i := 0; i < len(calls); i++ {
		if call, ok := calls[i]; ok {
			toolCalls = append(toolCalls, *call)
		}
	}
	return toolCalls
}
