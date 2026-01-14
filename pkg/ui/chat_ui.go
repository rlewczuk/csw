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
}

// ChatMessageUI represents a chat message as seen by the user in UI.
type ChatMessageUI struct {
	// Unique message ID
	Id string

	// Role of the message (assistant or user)
	Role ChatRoleUI

	// Text content of the message
	Text string

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

// IChatView is an interface for rendering chat conversation.
type IChatView interface {
	// Init initializes the view with all messages from the session.
	Init(session *ChatSessionUI) error

	// AddMessage adds a new message to the view.
	AddMessage(msg *ChatMessageUI) error

	// UpdateMessage updates an existing message in the view.
	UpdateMessage(msg *ChatMessageUI) error

	// UpdateTool updates an existing tool in the view.
	UpdateTool(tool *ToolUI) error

	// MoveToBottom scrolls the view to the bottom.
	MoveToBottom() error

	// QueryPermission queries user for permission to use a tool.
	QueryPermission(query *PermissionQueryUI) error
}

// IChatPresenter is an interface for propagating user input from UI to the chat session.
type IChatPresenter interface {
	// SetView sets the view to render the chat conversation.
	SetView(view IChatView) error

	// SendUserMessage sends a user message to the chat session and starts processing.
	SendUserMessage(message *ChatMessageUI) error

	// SaveUserMessage saves a user message to the chat session but doesn't start processing.
	SaveUserMessage(message *ChatMessageUI) error

	// Pause pauses the chat session (i.e. stops processing).
	Pause() error

	// Resume resumes the chat session (i.e. starts processing).
	Resume() error

	// PermissionResponse sends user response to permission query.
	PermissionResponse(response string) error

	// SetModel sets the model used for the chat session.
	// model string should be formatted as `provider/model-name`.
	SetModel(model string) error
}
