package ollama

import (
	"errors"
)

var (
	// ErrInvalidInput is returned when input format is invalid
	ErrInvalidInput = errors.New("invalid input format: expected string or []string")
)

// ChatRequest represents the request structure for /api/chat endpoint
type ChatRequest struct {
	Model     string        `json:"model"`                // Required: model name
	Messages  []Message     `json:"messages"`             // Required: messages of the chat
	Tools     []Tool        `json:"tools,omitempty"`      // Optional: list of tools for the model
	Think     bool          `json:"think,omitempty"`      // Optional: for thinking models
	Format    string        `json:"format,omitempty"`     // Optional: "json" or JSON schema object
	Options   *ModelOptions `json:"options,omitempty"`    // Optional: additional model parameters
	Stream    bool          `json:"stream,omitempty"`     // Optional: if false, returns single response
	KeepAlive string        `json:"keep_alive,omitempty"` // Optional: how long to keep model loaded (default: "5m")
}

// ChatResponse represents the streaming response structure for /api/chat endpoint
type ChatResponse struct {
	Model              string  `json:"model"`
	CreatedAt          string  `json:"created_at"`
	Message            Message `json:"message"`
	Done               bool    `json:"done"`
	DoneReason         string  `json:"done_reason,omitempty"`
	TotalDuration      int64   `json:"total_duration,omitempty"`
	LoadDuration       int64   `json:"load_duration,omitempty"`
	PromptEvalCount    int     `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64   `json:"prompt_eval_duration,omitempty"`
	EvalCount          int     `json:"eval_count,omitempty"`
	EvalDuration       int64   `json:"eval_duration,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role      string     `json:"role"`                 // Required: system, user, assistant, or tool
	Content   string     `json:"content"`              // Required: content of the message
	Thinking  string     `json:"thinking,omitempty"`   // Optional: for thinking models
	Images    []string   `json:"images,omitempty"`     // Optional: base64-encoded images
	ToolCalls []ToolCall `json:"tool_calls,omitempty"` // Optional: tools the model wants to use
	ToolName  string     `json:"tool_name,omitempty"`  // Optional: name of tool that was executed
}

// Tool represents a tool definition for tool calling
type Tool struct {
	Type     string       `json:"type"`     // Type of tool, e.g., "function"
	Function ToolFunction `json:"function"` // Function definition
}

// ToolFunction represents a function tool definition
type ToolFunction struct {
	Name        string                 `json:"name"`        // Function name
	Description string                 `json:"description"` // Function description
	Parameters  map[string]interface{} `json:"parameters"`  // Function parameters (JSON schema)
}

// ToolCall represents a tool call made by the model
type ToolCall struct {
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction represents the function details in a tool call
type ToolCallFunction struct {
	Name      string                 `json:"name"`      // Function name
	Arguments map[string]interface{} `json:"arguments"` // Function arguments
}

// ModelOptions represents additional model parameters
type ModelOptions struct {
	NumKeep          int      `json:"num_keep,omitempty"`
	Seed             int      `json:"seed,omitempty"`
	NumPredict       int      `json:"num_predict,omitempty"`
	TopK             int      `json:"top_k,omitempty"`
	TopP             float64  `json:"top_p,omitempty"`
	MinP             float64  `json:"min_p,omitempty"`
	TypicalP         float64  `json:"typical_p,omitempty"`
	RepeatLastN      int      `json:"repeat_last_n,omitempty"`
	Temperature      float64  `json:"temperature,omitempty"`
	RepeatPenalty    float64  `json:"repeat_penalty,omitempty"`
	PresencePenalty  float64  `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64  `json:"frequency_penalty,omitempty"`
	PenalizeNewline  bool     `json:"penalize_newline,omitempty"`
	Stop             []string `json:"stop,omitempty"`
	NUMA             bool     `json:"numa,omitempty"`
	NumCtx           int      `json:"num_ctx,omitempty"`
	NumBatch         int      `json:"num_batch,omitempty"`
	NumGPU           int      `json:"num_gpu,omitempty"`
	MainGPU          int      `json:"main_gpu,omitempty"`
	UseMmap          bool     `json:"use_mmap,omitempty"`
	NumThread        int      `json:"num_thread,omitempty"`
}

// EmbedRequest represents the request structure for /api/embed endpoint
type EmbedRequest struct {
	Model      string        `json:"model"`                // Required: model name
	Input      []string      `json:"input"`                // Required: string or []string to generate embeddings for
	Truncate   bool          `json:"truncate,omitempty"`   // Optional: truncate to fit context length (default: true)
	Options    *ModelOptions `json:"options,omitempty"`    // Optional: additional model parameters
	KeepAlive  string        `json:"keep_alive,omitempty"` // Optional: how long to keep model loaded (default: "5m")
	Dimensions int           `json:"dimensions,omitempty"` // Optional: number of dimensions for embedding
}

// EmbedResponse represents the response structure for /api/embed endpoint
type EmbedResponse struct {
	Model           string      `json:"model"`
	Embeddings      [][]float64 `json:"embeddings"`
	TotalDuration   int64       `json:"total_duration,omitempty"`
	LoadDuration    int64       `json:"load_duration,omitempty"`
	PromptEvalCount int         `json:"prompt_eval_count,omitempty"`
}

// EmbeddingsRequest represents the deprecated request structure for /api/embeddings endpoint
type EmbeddingsRequest struct {
	Model     string        `json:"model"`                // Required: model name
	Prompt    string        `json:"prompt"`               // Required: text to generate embeddings for
	Options   *ModelOptions `json:"options,omitempty"`    // Optional: additional model parameters
	KeepAlive string        `json:"keep_alive,omitempty"` // Optional: how long to keep model loaded (default: "5m")
}

// EmbeddingsResponse represents the deprecated response structure for /api/embeddings endpoint
type EmbeddingsResponse struct {
	Embedding []float64 `json:"embedding"`
}

// ListModelsResponse represents the response structure for GET /api/tags endpoint
type ListModelsResponse struct {
	Models []ModelInfo `json:"models"`
}

// ModelInfo represents information about a locally available model
type ModelInfo struct {
	Name       string       `json:"name"`        // Model name with tag (e.g., "llama3.2:latest")
	Model      string       `json:"model"`       // Model identifier (same as Name)
	ModifiedAt string       `json:"modified_at"` // Timestamp when model was last modified
	Size       int64        `json:"size"`        // Size in bytes
	Digest     string       `json:"digest"`      // SHA256 digest of the model
	Details    ModelDetails `json:"details"`     // Additional model details
}

// ModelDetails represents detailed information about a model
type ModelDetails struct {
	ParentModel       string   `json:"parent_model"`       // Parent model if this is derived from another
	Format            string   `json:"format"`             // Model format (e.g., "gguf")
	Family            string   `json:"family"`             // Model family (e.g., "llama", "qwen2")
	Families          []string `json:"families"`           // List of model families
	ParameterSize     string   `json:"parameter_size"`     // Parameter size (e.g., "7.6B", "3.2B")
	QuantizationLevel string   `json:"quantization_level"` // Quantization level (e.g., "Q4_K_M")
}
