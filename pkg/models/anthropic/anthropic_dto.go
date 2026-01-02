package anthropic

// MessagesRequest represents the request structure for POST /v1/messages endpoint
type MessagesRequest struct {
	Model       string         `json:"model"`                    // Required: model name
	Messages    []MessageParam `json:"messages"`                 // Required: messages of the conversation
	MaxTokens   int            `json:"max_tokens"`               // Required: maximum number of tokens to generate
	Temperature float64        `json:"temperature,omitempty"`    // Optional: 0.0 to 1.0
	TopP        float64        `json:"top_p,omitempty"`          // Optional: 0.0 to 1.0
	TopK        int            `json:"top_k,omitempty"`          // Optional: top-k sampling
	Stream      bool           `json:"stream,omitempty"`         // Optional: if true, returns streaming response
	System      interface{}    `json:"system,omitempty"`         // Optional: system prompt (string or array of content blocks)
	StopSeq     []string       `json:"stop_sequences,omitempty"` // Optional: stop sequences
}

// MessageParam represents a message in the request
type MessageParam struct {
	Role    string      `json:"role"`    // Required: "user" or "assistant"
	Content interface{} `json:"content"` // Required: string or array of content blocks
}

// ContentBlock represents a content block in a message (for structured content)
type ContentBlock struct {
	Type string `json:"type"`           // Required: "text", "image", etc.
	Text string `json:"text,omitempty"` // For text type
}

// MessagesResponse represents the response structure for POST /v1/messages endpoint
type MessagesResponse struct {
	ID           string            `json:"id"`                      // Required: unique ID
	Type         string            `json:"type"`                    // Required: "message"
	Role         string            `json:"role"`                    // Required: "assistant"
	Content      []ResponseContent `json:"content"`                 // Required: array of content blocks
	Model        string            `json:"model"`                   // Required: model used
	StopReason   string            `json:"stop_reason"`             // Optional: "end_turn", "max_tokens", "stop_sequence"
	StopSequence string            `json:"stop_sequence,omitempty"` // Optional: which stop sequence was generated
	Usage        UsageInfo         `json:"usage"`                   // Required: token usage
}

// ResponseContent represents a content block in the response
type ResponseContent struct {
	Type string `json:"type"` // Required: "text"
	Text string `json:"text"` // Required: content text
}

// UsageInfo represents token usage information
type UsageInfo struct {
	InputTokens  int `json:"input_tokens"`  // Required: input tokens used
	OutputTokens int `json:"output_tokens"` // Required: output tokens used
}

// StreamEvent represents a server-sent event in streaming responses
type StreamEvent struct {
	Type         string            `json:"type"`                    // Required: event type
	Index        int               `json:"index,omitempty"`         // For content block events
	Delta        *Delta            `json:"delta,omitempty"`         // For delta events
	ContentBlock *ResponseContent  `json:"content_block,omitempty"` // For content_block_start
	Message      *MessagesResponse `json:"message,omitempty"`       // For message_start
	Usage        *UsageInfo        `json:"usage,omitempty"`         // For message_delta
}

// Delta represents incremental changes in streaming
type Delta struct {
	Type         string `json:"type,omitempty"`
	Text         string `json:"text,omitempty"`
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

// ModelsListResponse represents the response for GET /v1/models endpoint
type ModelsListResponse struct {
	Data []ModelInfo `json:"data"` // Required: list of models
}

// ModelInfo represents information about a model
type ModelInfo struct {
	ID          string `json:"id"`           // Required: model ID
	CreatedAt   string `json:"created_at"`   // Required: RFC 3339 datetime
	DisplayName string `json:"display_name"` // Required: human-readable name
	Type        string `json:"type"`         // Required: "model"
}

// ErrorResponse represents an error response from the API
type ErrorResponse struct {
	Type  string    `json:"type"`  // Required: "error"
	Error *APIError `json:"error"` // Required: error details
}

// APIError represents error details
type APIError struct {
	Type    string `json:"type"`    // Required: error type
	Message string `json:"message"` // Required: error message
}
