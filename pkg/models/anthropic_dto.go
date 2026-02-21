package models

// AnthropicMessagesRequest represents the request structure for POST /v1/messages endpoint
type AnthropicMessagesRequest struct {
	Model            string                  `json:"model"`                    // Required: model name
	Messages         []AnthropicMessageParam `json:"messages"`                 // Required: messages of the conversation
	MaxTokens        int                     `json:"max_tokens"`               // Required: maximum number of tokens to generate
	Temperature      float64                 `json:"temperature,omitempty"`    // Optional: 0.0 to 1.0
	TopP             float64                 `json:"top_p,omitempty"`          // Optional: 0.0 to 1.0
	TopK             int                     `json:"top_k,omitempty"`          // Optional: top-k sampling
	Stream           bool                    `json:"stream,omitempty"`         // Optional: if true, returns streaming response
	System           interface{}             `json:"system,omitempty"`         // Optional: system prompt (string or array of content blocks)
	StopSeq          []string                `json:"stop_sequences,omitempty"` // Optional: stop sequences
	Tools            []AnthropicTool         `json:"tools,omitempty"`          // Optional: tools available for the model to use
	Thinking         *AnthropicThinking      `json:"thinking,omitempty"`       // Optional: thinking configuration for Claude 3.7+
}

// AnthropicThinking represents thinking configuration for Claude 3.7+ models.
type AnthropicThinking struct {
	Type         string `json:"type"`           // Required: "enabled" or "disabled"
	BudgetTokens int    `json:"budget_tokens"`  // Required when enabled: max tokens for thinking
}

// AnthropicMessageParam represents a message in the request
type AnthropicMessageParam struct {
	Role    string      `json:"role"`    // Required: "user" or "assistant"
	Content interface{} `json:"content"` // Required: string or array of content blocks
}

// AnthropicContentBlock represents a content block in a message (for structured content)
type AnthropicContentBlock struct {
	Type      string                 `json:"type"`                  // Required: "text", "image", "tool_use", "tool_result"
	Text      string                 `json:"text,omitempty"`        // For text type
	ID        string                 `json:"id,omitempty"`          // For tool_use and tool_result types
	Name      string                 `json:"name,omitempty"`        // For tool_use type
	Input     map[string]interface{} `json:"input,omitempty"`       // For tool_use type
	Content   interface{}            `json:"content,omitempty"`     // For tool_result type (string or array of content blocks)
	IsError   bool                   `json:"is_error,omitempty"`    // For tool_result type
	ToolUseID string                 `json:"tool_use_id,omitempty"` // For tool_result type
}

// AnthropicTool represents a tool definition
type AnthropicTool struct {
	Name        string                 `json:"name"`                  // Required: tool name
	Description string                 `json:"description,omitempty"` // Optional: tool description
	InputSchema map[string]interface{} `json:"input_schema"`          // Required: JSON schema for tool input
}

// AnthropicMessagesResponse represents the response structure for POST /v1/messages endpoint
type AnthropicMessagesResponse struct {
	ID           string                     `json:"id"`                      // Required: unique ID
	Type         string                     `json:"type"`                    // Required: "message"
	Role         string                     `json:"role"`                    // Required: "assistant"
	Content      []AnthropicResponseContent `json:"content"`                 // Required: array of content blocks
	Model        string                     `json:"model"`                   // Required: model used
	StopReason   string                     `json:"stop_reason"`             // Optional: "end_turn", "max_tokens", "stop_sequence", "tool_use"
	StopSequence string                     `json:"stop_sequence,omitempty"` // Optional: which stop sequence was generated
	Usage        AnthropicUsageInfo         `json:"usage"`                   // Required: token usage
}

// AnthropicResponseContent represents a content block in the response
type AnthropicResponseContent struct {
	Type  string                 `json:"type"`            // Required: "text" or "tool_use"
	Text  string                 `json:"text,omitempty"`  // For text type
	ID    string                 `json:"id,omitempty"`    // For tool_use type
	Name  string                 `json:"name,omitempty"`  // For tool_use type
	Input map[string]interface{} `json:"input,omitempty"` // For tool_use type
}

// AnthropicUsageInfo represents token usage information
type AnthropicUsageInfo struct {
	InputTokens  int `json:"input_tokens"`  // Required: input tokens used
	OutputTokens int `json:"output_tokens"` // Required: output tokens used
}

// AnthropicStreamEvent represents a server-sent event in streaming responses
type AnthropicStreamEvent struct {
	Type         string                     `json:"type"`                    // Required: event type
	Index        int                        `json:"index,omitempty"`         // For content block events
	Delta        *AnthropicDelta            `json:"delta,omitempty"`         // For delta events
	ContentBlock *AnthropicResponseContent  `json:"content_block,omitempty"` // For content_block_start
	Message      *AnthropicMessagesResponse `json:"message,omitempty"`       // For message_start
	Usage        *AnthropicUsageInfo        `json:"usage,omitempty"`         // For message_delta
}

// AnthropicDelta represents incremental changes in streaming
type AnthropicDelta struct {
	Type         string `json:"type,omitempty"`
	Text         string `json:"text,omitempty"`
	PartialJSON  string `json:"partial_json,omitempty"` // For tool_use streaming
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

// AnthropicModelsListResponse represents the response for GET /v1/models endpoint
type AnthropicModelsListResponse struct {
	Data []AnthropicModelInfo `json:"data"` // Required: list of models
}

// AnthropicModelInfo represents information about a model
type AnthropicModelInfo struct {
	ID          string `json:"id"`           // Required: model ID
	CreatedAt   string `json:"created_at"`   // Required: RFC 3339 datetime
	DisplayName string `json:"display_name"` // Required: human-readable name
	Type        string `json:"type"`         // Required: "model"
}

// AnthropicErrorResponse represents an error response from the API
type AnthropicErrorResponse struct {
	Type  string             `json:"type"`  // Required: "error"
	Error *AnthropicAPIError `json:"error"` // Required: error details
}

// AnthropicAPIError represents error details
type AnthropicAPIError struct {
	Type    string `json:"type"`    // Required: error type
	Message string `json:"message"` // Required: error message
}
