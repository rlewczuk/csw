package ui

type ToolStatus string

// ToolStatus represents the status of a tool call.
const (
	ToolStatusStarted   ToolStatus = "started"
	ToolStatusExecuting ToolStatus = "executing"
	ToolStatusSucceeded ToolStatus = "succeeded"
	ToolStatusFailed    ToolStatus = "failed"
)

// ChatRole represents who sent a message in chat.
type ChatRole string

const (
	ChatRoleAssistant ChatRole = "assistant"
	ChatRoleUser      ChatRole = "user"
)

// ToolState represents the state of a tool call as seen by the user in UI.
type ToolState struct {
	// Unique tool call ID
	Id string `json:"id"`

	// Status of the tool call
	Status ToolStatus `json:"status"`

	// Tool name
	Name string `json:"name"`

	// Tool arguments
	Message string `json:"message"`

	// Tool arguments
	Props [][]string `json:"props"`
}

// ChatMessage represents a chat message as seen by the user in UI.
type ChatMessage struct {
	// Unique message ID
	Id string

	// Role of the message (assistant or user)
	Role ChatRole

	// Text content of the message
	Text string

	// List of tools in the message
	Tools []*ToolState
}

// ChatSession represents a chat session as seen by the user in UI.
// It is a subset of information from SweSession suitable for rendering on all platforms.
type ChatSession struct {

	// Unique session ID
	Id string

	// Model used for the session
	Model string

	// Agent role
	Role string

	// Working directory
	WorkDir string

	// List of messages in the chat
	Messages []*ChatMessage
}

// ChatView is an interface for rendering chat conversation.
type ChatView interface {
	// Init initializes the view with all messages from the session.
	Init(session *ChatSession) error

	// AddMessage adds a new message to the view.
	AddMessage(msg *ChatMessage) error

	// UpdateMessage updates an existing message in the view.
	UpdateMessage(msg *ChatMessage) error

	// UpdateTool updates an existing tool in the view.
	UpdateTool(tool *ToolState) error

	// MoveToBottom scrolls the view to the bottom.
	MoveToBottom() error
}

// ChatPresenter is an interface for propagating user input from UI to the chat session.
type ChatPresenter interface {
	// SetView sets the view to render the chat conversation.
	SetView(view ChatView) error

	// SendUserMessage sends a user message to the chat session and starts processing.
	SendUserMessage(message *ChatMessage) error

	// SaveUserMessage saves a user message to the chat session but doesn't start processing.
	SaveUserMessage(message *ChatMessage) error

	// Pause pauses the chat session (i.e. stops processing).
	Pause() error

	// Resume resumes the chat session (i.e. starts processing).
	Resume() error
}
