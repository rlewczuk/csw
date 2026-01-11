package ui

type CompositeNotification string

const (
	CompositeNotificationRefresh CompositeNotification = "refresh"
)

// CompositeWidget is an interface for widgets that can be composed together.
type CompositeWidget interface {
	// Notify sends a notification to the widget.
	Notify(msg CompositeNotification)

	// SetParent sets the parent widget.
	SetParent(parent CompositeWidget)
}
