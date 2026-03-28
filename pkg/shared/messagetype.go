package shared

// MessageType defines a type of user-facing status message.
type MessageType string

const (
	// MessageTypeInfo indicates an informational message.
	MessageTypeInfo MessageType = "info"
	// MessageTypeWarning indicates a warning message.
	MessageTypeWarning MessageType = "warning"
	// MessageTypeError indicates an error message.
	MessageTypeError MessageType = "error"
)
