package ui

// PermissionQueryUI represents the structure for querying user permissions in a UI context.
type PermissionQueryUI struct {
	// Unique query ID
	Id string

	// Title content of the query
	Title string

	// Additional details to display
	Details string

	// List of options to display to user
	Options []string

	// Custom response from user (directed to LLM instead of unlocking tool)
	AllowCustomResponse string
}
