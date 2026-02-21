package models

// OpenaiChatCompletionRequest represents the request structure for /v1/chat/completions endpoint
type OpenaiChatCompletionRequest struct {
	Model            string                        `json:"model"`                       // Required: model name
	Messages         []OpenaiChatCompletionMessage `json:"messages"`                    // Required: messages of the chat
	FrequencyPenalty float64                       `json:"frequency_penalty,omitempty"` // Optional: -2.0 to 2.0
	PresencePenalty  float64                       `json:"presence_penalty,omitempty"`  // Optional: -2.0 to 2.0
	ReasoningEffort  string                        `json:"reasoning_effort,omitempty"`  // Optional: reasoning effort for o1 models (low, medium, high)
	ResponseFormat   *OpenaiResponseFormat         `json:"response_format,omitempty"`   // Optional: JSON mode
	Seed             int                           `json:"seed,omitempty"`              // Optional: for reproducible outputs
	Stop             []string                      `json:"stop,omitempty"`              // Optional: stop sequences
	Stream           bool                          `json:"stream,omitempty"`            // Optional: if true, returns streaming response
	StreamOptions    *OpenaiStreamOptions          `json:"stream_options,omitempty"`    // Optional: streaming options
	Temperature      float64                       `json:"temperature,omitempty"`       // Optional: 0.0 to 2.0
	TopP             float64                       `json:"top_p,omitempty"`             // Optional: 0.0 to 1.0
	MaxTokens        int                           `json:"max_tokens,omitempty"`        // Optional: max tokens to generate
	Tools            []OpenaiTool                  `json:"tools,omitempty"`             // Optional: tools for function calling
	ToolChoice       interface{}                   `json:"tool_choice,omitempty"`       // Optional: tool choice strategy
	User             string                        `json:"user,omitempty"`              // Optional: user identifier
	N                int                           `json:"n,omitempty"`                 // Optional: number of completions
}

// OpenaiStreamOptions represents options for streaming responses.
type OpenaiStreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"` // Optional: include usage in streaming response
}

// OpenaiChatCompletionMessage represents a message in the chat
type OpenaiChatCompletionMessage struct {
	Role             string           `json:"role"`                        // Required: system, user, assistant, or tool
	Content          interface{}      `json:"content"`                     // Required: string or array of content parts
	ReasoningContent string           `json:"reasoning_content,omitempty"` // Optional: reasoning content (for thinking models)
	ToolCalls        []OpenaiToolCall `json:"tool_calls,omitempty"`        // Optional: tools the model wants to use
	ToolCallID       string           `json:"tool_call_id,omitempty"`      // Optional: for tool responses
}

// OpenaiContentPart represents a part of the message content (text or image)
type OpenaiContentPart struct {
	Type     string          `json:"type"`                // Required: "text" or "image_url"
	Text     string          `json:"text,omitempty"`      // For text type
	ImageURL *OpenaiImageURL `json:"image_url,omitempty"` // For image_url type
}

// OpenaiImageURL represents an image in the message
type OpenaiImageURL struct {
	URL string `json:"url"` // Required: URL or base64-encoded image
}

// OpenaiResponseFormat defines the response format
type OpenaiResponseFormat struct {
	Type string `json:"type"` // "json_object" for JSON mode
}

// OpenaiTool represents a tool definition for function calling
type OpenaiTool struct {
	Type     string             `json:"type"`     // Required: "function"
	Function OpenaiToolFunction `json:"function"` // Required: function definition
}

// OpenaiToolFunction represents a function tool definition
type OpenaiToolFunction struct {
	Name        string                 `json:"name"`                  // Required: function name
	Description string                 `json:"description,omitempty"` // Optional: function description
	Parameters  map[string]interface{} `json:"parameters,omitempty"`  // Optional: JSON schema for parameters
}

// OpenaiToolCall represents a tool call made by the model
type OpenaiToolCall struct {
	Index    int                    `json:"index,omitempty"` // Optional: index in streaming responses
	ID       string                 `json:"id"`              // Required: tool call ID
	Type     string                 `json:"type"`            // Required: "function"
	Function OpenaiToolCallFunction `json:"function"`        // Required: function details
}

// OpenaiToolCallFunction represents the function details in a tool call
type OpenaiToolCallFunction struct {
	Name      string `json:"name"`      // Required: function name
	Arguments string `json:"arguments"` // Required: JSON string of arguments
}

// OpenaiChatCompletionResponse represents the response structure for /v1/chat/completions endpoint
type OpenaiChatCompletionResponse struct {
	ID      string                       `json:"id"`              // Required: completion ID
	Object  string                       `json:"object"`          // Required: "chat.completion" or "chat.completion.chunk"
	Created int64                        `json:"created"`         // Required: unix timestamp
	Model   string                       `json:"model"`           // Required: model used
	Choices []OpenaiChatCompletionChoice `json:"choices"`         // Required: completion choices
	Usage   *OpenaiChatCompletionUsage   `json:"usage,omitempty"` // Optional: usage information
}

// OpenaiChatCompletionChoice represents a completion choice
type OpenaiChatCompletionChoice struct {
	Index        int                          `json:"index"`                   // Required: choice index
	Message      *OpenaiChatCompletionMessage `json:"message,omitempty"`       // For non-streaming
	Delta        *OpenaiChatCompletionMessage `json:"delta,omitempty"`         // For streaming
	FinishReason string                       `json:"finish_reason,omitempty"` // Optional: "stop", "length", "tool_calls", etc.
}

// OpenaiChatCompletionUsage represents usage information
type OpenaiChatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`     // Required: tokens in prompt
	CompletionTokens int `json:"completion_tokens"` // Required: tokens in completion
	TotalTokens      int `json:"total_tokens"`      // Required: total tokens
}

// OpenaiEmbeddingRequest represents the request structure for /v1/embeddings endpoint
type OpenaiEmbeddingRequest struct {
	Model          string      `json:"model"`                     // Required: model name
	Input          interface{} `json:"input"`                     // Required: string, array of strings, or array of tokens
	EncodingFormat string      `json:"encoding_format,omitempty"` // Optional: "float" or "base64"
	Dimensions     int         `json:"dimensions,omitempty"`      // Optional: number of dimensions
	User           string      `json:"user,omitempty"`            // Optional: user identifier
}

// OpenaiEmbeddingResponse represents the response structure for /v1/embeddings endpoint
type OpenaiEmbeddingResponse struct {
	Object string                `json:"object"` // Required: "list"
	Data   []OpenaiEmbeddingData `json:"data"`   // Required: embedding data
	Model  string                `json:"model"`  // Required: model used
	Usage  *OpenaiEmbeddingUsage `json:"usage"`  // Required: usage information
}

// OpenaiEmbeddingData represents a single embedding
type OpenaiEmbeddingData struct {
	Object    string    `json:"object"`    // Required: "embedding"
	Embedding []float64 `json:"embedding"` // Required: embedding vector
	Index     int       `json:"index"`     // Required: embedding index
}

// OpenaiEmbeddingUsage represents usage information for embeddings
type OpenaiEmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"` // Required: tokens in prompt
	TotalTokens  int `json:"total_tokens"`  // Required: total tokens
}

// OpenaiModelList represents the response structure for GET /v1/models endpoint
type OpenaiModelList struct {
	Object string            `json:"object"` // Required: "list"
	Data   []OpenaiModelData `json:"data"`   // Required: list of models
}

// OpenaiModelData represents information about a model
type OpenaiModelData struct {
	ID      string `json:"id"`       // Required: model ID
	Object  string `json:"object"`   // Required: "model"
	Created int64  `json:"created"`  // Required: unix timestamp (modified time)
	OwnedBy string `json:"owned_by"` // Required: owner (defaults to "library")
}

// OpenaiErrorResponse represents an error response from the API
type OpenaiErrorResponse struct {
	Error *OpenaiAPIError `json:"error"` // Required: error details
}

// OpenaiAPIError represents error details
type OpenaiAPIError struct {
	Message string      `json:"message"`         // Required: error message
	Type    string      `json:"type"`            // Required: error type
	Param   interface{} `json:"param,omitempty"` // Optional: parameter that caused the error
	Code    interface{} `json:"code,omitempty"`  // Optional: error code
}
