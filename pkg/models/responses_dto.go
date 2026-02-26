package models

// ResponsesCreateRequest represents a request to the Responses API.
type ResponsesCreateRequest struct {
	Model           string              `json:"model,omitempty"`
	Input           []ResponsesItem     `json:"input,omitempty"`
	Tools           []ResponsesTool     `json:"tools,omitempty"`
	ToolChoice      interface{}         `json:"tool_choice,omitempty"`
	Store           *bool               `json:"store,omitempty"`
	Instructions    string              `json:"instructions,omitempty"`
	Include         []string            `json:"include,omitempty"`
	PromptCacheKey  string              `json:"prompt_cache_key,omitempty"`
	Reasoning       *ResponsesReasoning `json:"reasoning,omitempty"`
	Stream          bool                `json:"stream,omitempty"`
	Temperature     float64             `json:"temperature,omitempty"`
	TopP            float64             `json:"top_p,omitempty"`
	MaxOutputTokens int                 `json:"max_output_tokens,omitempty"`
}

// ResponsesTool represents a tool definition for Responses API.
type ResponsesTool struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ResponsesItem represents an item in Responses API requests or responses.
type ResponsesItem struct {
	ID        string             `json:"id,omitempty"`
	Type      string             `json:"type"`
	Role      string             `json:"role,omitempty"`
	Content   []ResponsesContent `json:"content,omitempty"`
	CallID    string             `json:"call_id,omitempty"`
	Name      string             `json:"name,omitempty"`
	Arguments string             `json:"arguments,omitempty"`
	Output    interface{}        `json:"output,omitempty"`
}

// ResponsesContent represents a content part for Responses API.
type ResponsesContent struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	Refusal string `json:"refusal,omitempty"`
}

// ResponsesResponse represents a Responses API response.
type ResponsesResponse struct {
	ID                   string               `json:"id"`
	Object               string               `json:"object"`
	CreatedAt            int64                `json:"created_at,omitempty"`
	Status               string               `json:"status,omitempty"`
	Background           bool                 `json:"background,omitempty"`
	CompletedAt          *int64               `json:"completed_at,omitempty"`
	Error                interface{}          `json:"error,omitempty"`
	FrequencyPenalty     float64              `json:"frequency_penalty,omitempty"`
	IncompleteDetails    interface{}          `json:"incomplete_details,omitempty"`
	Instructions         string               `json:"instructions,omitempty"`
	MaxOutputTokens      *int                 `json:"max_output_tokens,omitempty"`
	MaxToolCalls         *int                 `json:"max_tool_calls,omitempty"`
	Model                string               `json:"model,omitempty"`
	Output               []ResponsesItem      `json:"output,omitempty"`
	ParallelToolCalls    *bool                `json:"parallel_tool_calls,omitempty"`
	PresencePenalty      float64              `json:"presence_penalty,omitempty"`
	PreviousResponseID   string               `json:"previous_response_id,omitempty"`
	PromptCacheKey       string               `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention interface{}          `json:"prompt_cache_retention,omitempty"`
	Reasoning            *ResponsesReasoning  `json:"reasoning,omitempty"`
	SafetyIdentifier     string               `json:"safety_identifier,omitempty"`
	ServiceTier          string               `json:"service_tier,omitempty"`
	Store                *bool                `json:"store,omitempty"`
	Temperature          float64              `json:"temperature,omitempty"`
	Text                 *ResponsesTextConfig `json:"text,omitempty"`
	ToolChoice           interface{}          `json:"tool_choice,omitempty"`
	Tools                []ResponsesTool      `json:"tools,omitempty"`
	Usage                *ResponsesUsage      `json:"usage,omitempty"`
}

// ResponsesStreamEvent represents a streaming event from Responses API.
type ResponsesStreamEvent struct {
	Type      string             `json:"type"`
	Response  *ResponsesResponse `json:"response,omitempty"`
	Item      *ResponsesItem     `json:"item,omitempty"`
	ItemID    string             `json:"item_id,omitempty"`
	Delta     string             `json:"delta,omitempty"`
	Arguments string             `json:"arguments,omitempty"`
}

// ResponsesReasoning represents reasoning configuration or metadata.
type ResponsesReasoning struct {
	Effort  string `json:"effort,omitempty"`
	Summary string `json:"summary,omitempty"`
}

// ResponsesUsage represents token usage information returned by Responses API.
type ResponsesUsage struct {
	InputTokens        int                         `json:"input_tokens,omitempty"`
	OutputTokens       int                         `json:"output_tokens,omitempty"`
	TotalTokens        int                         `json:"total_tokens,omitempty"`
	InputTokensDetails *ResponsesInputTokenDetails `json:"input_tokens_details,omitempty"`
}

// ResponsesInputTokenDetails represents details for input tokens.
type ResponsesInputTokenDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
}

// ResponsesTextConfig represents text response configuration.
type ResponsesTextConfig struct {
	Format    *ResponsesTextFormat `json:"format,omitempty"`
	Verbosity string               `json:"verbosity,omitempty"`
}

// ResponsesTextFormat represents text format configuration.
type ResponsesTextFormat struct {
	Type string `json:"type,omitempty"`
}
