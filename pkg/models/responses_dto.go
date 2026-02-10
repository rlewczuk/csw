package models

// ResponsesCreateRequest represents a request to the Responses API.
type ResponsesCreateRequest struct {
	Model           string          `json:"model,omitempty"`
	Input           []ResponsesItem `json:"input,omitempty"`
	Tools           []ResponsesTool `json:"tools,omitempty"`
	ToolChoice      interface{}     `json:"tool_choice,omitempty"`
	Stream          bool            `json:"stream,omitempty"`
	Temperature     float64         `json:"temperature,omitempty"`
	TopP            float64         `json:"top_p,omitempty"`
	MaxOutputTokens int             `json:"max_output_tokens,omitempty"`
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
	ID     string          `json:"id"`
	Object string          `json:"object"`
	Status string          `json:"status,omitempty"`
	Output []ResponsesItem `json:"output,omitempty"`
}

// ResponsesStreamEvent represents a streaming event from Responses API.
type ResponsesStreamEvent struct {
	Type      string         `json:"type"`
	Item      *ResponsesItem `json:"item,omitempty"`
	ItemID    string         `json:"item_id,omitempty"`
	Delta     string         `json:"delta,omitempty"`
	Arguments string         `json:"arguments,omitempty"`
}
