package llm

type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type Usage struct {
	InputTokens  int
	OutputTokens int
}

type TokenProbability struct {
	Token       string  `json:"token"`
	Logprob     float64 `json:"logprob"`
	Probability float64 `json:"probability"`
}

type LogprobFrame struct {
	Token        string             `json:"token"`
	Alternatives []TokenProbability `json:"alternatives"`
}

type StreamEvent struct {
	Type      string
	Content   string
	ToolCalls []ToolCall
	Logprobs  []LogprobFrame
	Usage     Usage
	Error     error
}
