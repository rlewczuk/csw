package ui

type ToolStatusUI string

// ToolStatusUI represents the status of a tool call.
const (
	ToolStatusStarted   ToolStatusUI = "started"
	ToolStatusExecuting ToolStatusUI = "executing"
	ToolStatusSucceeded ToolStatusUI = "succeeded"
	ToolStatusFailed    ToolStatusUI = "failed"
)

// ChatRoleUI represents who sent a message in chat.
type ChatRoleUI string

const (
	ChatRoleAssistant ChatRoleUI = "assistant"
	ChatRoleUser      ChatRoleUI = "user"
)

// ToolUI represents the state of a tool call as seen by the user in UI.
type ToolUI struct {
	// Unique tool call ID
	Id string `json:"id"`

	// Status of the tool call
	Status ToolStatusUI `json:"status"`

	// Tool name
	Name string `json:"name"`

	// Tool arguments
	Message string `json:"message"`

	// Tool arguments
	Props [][]string `json:"props"`

	// Summary is the one-line summary of the tool call result from Tool.Render()
	Summary string `json:"summary"`

	// Details is the full information of the tool call result from Tool.Render()
	Details string `json:"details"`

	// JSONL is the JSONL representation of the tool call result from Tool.Render()
	JSONL string `json:"jsonl"`

	// Meta contains additional properties from Tool.Render() that can be used to display in the UI
	Meta map[string]string `json:"meta"`
}

// ChatMessageUI represents a chat message as seen by the user in UI.
type ChatMessageUI struct {
	// Unique message ID
	Id string

	// Role of the message (assistant or user)
	Role ChatRoleUI

	// Text content of the message
	Text string

	// Thinking content of the message (for thinking models)
	Thinking string

	// List of tools in the message
	Tools []*ToolUI
}

// ChatSessionUI represents a chat session as seen by the user in UI.
// It is a subset of information from SweSession suitable for rendering on all platforms.
type ChatSessionUI struct {

	// Unique session ID
	Id string

	// Model used for the session
	Model string

	// Agent role
	Role string

	// Working directory
	WorkDir string

	// List of messages in the chat
	Messages []*ChatMessageUI
}
