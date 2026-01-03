package openai

// ChatCompletionRequest represents the request structure for /v1/chat/completions endpoint
type ChatCompletionRequest struct {
	Model            string                  `json:"model"`                       // Required: model name
	Messages         []ChatCompletionMessage `json:"messages"`                    // Required: messages of the chat
	FrequencyPenalty float64                 `json:"frequency_penalty,omitempty"` // Optional: -2.0 to 2.0
	PresencePenalty  float64                 `json:"presence_penalty,omitempty"`  // Optional: -2.0 to 2.0
	ResponseFormat   *ResponseFormat         `json:"response_format,omitempty"`   // Optional: JSON mode
	Seed             int                     `json:"seed,omitempty"`              // Optional: for reproducible outputs
	Stop             []string                `json:"stop,omitempty"`              // Optional: stop sequences
	Stream           bool                    `json:"stream,omitempty"`            // Optional: if true, returns streaming response
	Temperature      float64                 `json:"temperature,omitempty"`       // Optional: 0.0 to 2.0
	TopP             float64                 `json:"top_p,omitempty"`             // Optional: 0.0 to 1.0
	MaxTokens        int                     `json:"max_tokens,omitempty"`        // Optional: max tokens to generate
	Tools            []Tool                  `json:"tools,omitempty"`             // Optional: tools for function calling
	ToolChoice       interface{}             `json:"tool_choice,omitempty"`       // Optional: tool choice strategy
	User             string                  `json:"user,omitempty"`              // Optional: user identifier
	N                int                     `json:"n,omitempty"`                 // Optional: number of completions
}

// ChatCompletionMessage represents a message in the chat
type ChatCompletionMessage struct {
	Role       string      `json:"role"`                   // Required: system, user, assistant, or tool
	Content    interface{} `json:"content"`                // Required: string or array of content parts
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`   // Optional: tools the model wants to use
	ToolCallID string      `json:"tool_call_id,omitempty"` // Optional: for tool responses
}

// ContentPart represents a part of the message content (text or image)
type ContentPart struct {
	Type     string    `json:"type"`                // Required: "text" or "image_url"
	Text     string    `json:"text,omitempty"`      // For text type
	ImageURL *ImageURL `json:"image_url,omitempty"` // For image_url type
}

// ImageURL represents an image in the message
type ImageURL struct {
	URL string `json:"url"` // Required: URL or base64-encoded image
}

// ResponseFormat defines the response format
type ResponseFormat struct {
	Type string `json:"type"` // "json_object" for JSON mode
}

// Tool represents a tool definition for function calling
type Tool struct {
	Type     string       `json:"type"`     // Required: "function"
	Function ToolFunction `json:"function"` // Required: function definition
}

// ToolFunction represents a function tool definition
type ToolFunction struct {
	Name        string                 `json:"name"`                  // Required: function name
	Description string                 `json:"description,omitempty"` // Optional: function description
	Parameters  map[string]interface{} `json:"parameters,omitempty"`  // Optional: JSON schema for parameters
}

// ToolCall represents a tool call made by the model
type ToolCall struct {
	Index    int              `json:"index,omitempty"` // Optional: index in streaming responses
	ID       string           `json:"id"`              // Required: tool call ID
	Type     string           `json:"type"`            // Required: "function"
	Function ToolCallFunction `json:"function"`        // Required: function details
}

// ToolCallFunction represents the function details in a tool call
type ToolCallFunction struct {
	Name      string `json:"name"`      // Required: function name
	Arguments string `json:"arguments"` // Required: JSON string of arguments
}

// ChatCompletionResponse represents the response structure for /v1/chat/completions endpoint
type ChatCompletionResponse struct {
	ID      string                 `json:"id"`              // Required: completion ID
	Object  string                 `json:"object"`          // Required: "chat.completion" or "chat.completion.chunk"
	Created int64                  `json:"created"`         // Required: unix timestamp
	Model   string                 `json:"model"`           // Required: model used
	Choices []ChatCompletionChoice `json:"choices"`         // Required: completion choices
	Usage   *ChatCompletionUsage   `json:"usage,omitempty"` // Optional: usage information
}

// ChatCompletionChoice represents a completion choice
type ChatCompletionChoice struct {
	Index        int                    `json:"index"`                   // Required: choice index
	Message      *ChatCompletionMessage `json:"message,omitempty"`       // For non-streaming
	Delta        *ChatCompletionMessage `json:"delta,omitempty"`         // For streaming
	FinishReason string                 `json:"finish_reason,omitempty"` // Optional: "stop", "length", "tool_calls", etc.
}

// ChatCompletionUsage represents usage information
type ChatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`     // Required: tokens in prompt
	CompletionTokens int `json:"completion_tokens"` // Required: tokens in completion
	TotalTokens      int `json:"total_tokens"`      // Required: total tokens
}

// EmbeddingRequest represents the request structure for /v1/embeddings endpoint
type EmbeddingRequest struct {
	Model          string      `json:"model"`                     // Required: model name
	Input          interface{} `json:"input"`                     // Required: string, array of strings, or array of tokens
	EncodingFormat string      `json:"encoding_format,omitempty"` // Optional: "float" or "base64"
	Dimensions     int         `json:"dimensions,omitempty"`      // Optional: number of dimensions
	User           string      `json:"user,omitempty"`            // Optional: user identifier
}

// EmbeddingResponse represents the response structure for /v1/embeddings endpoint
type EmbeddingResponse struct {
	Object string          `json:"object"` // Required: "list"
	Data   []EmbeddingData `json:"data"`   // Required: embedding data
	Model  string          `json:"model"`  // Required: model used
	Usage  *EmbeddingUsage `json:"usage"`  // Required: usage information
}

// EmbeddingData represents a single embedding
type EmbeddingData struct {
	Object    string    `json:"object"`    // Required: "embedding"
	Embedding []float64 `json:"embedding"` // Required: embedding vector
	Index     int       `json:"index"`     // Required: embedding index
}

// EmbeddingUsage represents usage information for embeddings
type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"` // Required: tokens in prompt
	TotalTokens  int `json:"total_tokens"`  // Required: total tokens
}

// ModelList represents the response structure for GET /v1/models endpoint
type ModelList struct {
	Object string      `json:"object"` // Required: "list"
	Data   []ModelData `json:"data"`   // Required: list of models
}

// ModelData represents information about a model
type ModelData struct {
	ID      string `json:"id"`       // Required: model ID
	Object  string `json:"object"`   // Required: "model"
	Created int64  `json:"created"`  // Required: unix timestamp (modified time)
	OwnedBy string `json:"owned_by"` // Required: owner (defaults to "library")
}

// ErrorResponse represents an error response from the API
type ErrorResponse struct {
	Error *APIError `json:"error"` // Required: error details
}

// APIError represents error details
type APIError struct {
	Message string      `json:"message"`         // Required: error message
	Type    string      `json:"type"`            // Required: error type
	Param   interface{} `json:"param,omitempty"` // Optional: parameter that caused the error
	Code    interface{} `json:"code,omitempty"`  // Optional: error code
}
