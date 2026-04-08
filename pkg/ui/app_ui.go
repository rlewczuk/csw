package ui

import "github.com/rlewczuk/csw/pkg/shared"

// MessageType defines a type of user-facing status message.
type MessageType = shared.MessageType

const (
	// MessageTypeInfo indicates an informational message.
	MessageTypeInfo MessageType = "info"
	// MessageTypeWarning indicates a warning message.
	MessageTypeWarning MessageType = "warning"
	// MessageTypeError indicates an error message.
	MessageTypeError MessageType = "error"
)
