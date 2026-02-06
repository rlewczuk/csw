package tool

import (
	"fmt"
	"path/filepath"

	"github.com/codesnort/codesnort-swe/pkg/shared"
)

const (
	PermissionTitleRequired       = "Permission Required"
	PermissionTitleAbsolutePath   = "Permission Required for Absolute Path"
	PermissionOptionAllow         = "Allow"
	PermissionOptionDeny          = "Deny"
	PermissionOptionAllowRemember = "Allow and remember (add to privileges)"
)

// NewPermissionQuery builds a standardized permission query response.
func NewPermissionQuery(call *ToolCall, title, details string, options []string, meta map[string]string) *ToolResponse {
	if title == "" {
		title = PermissionTitleRequired
	}
	normalizedOptions := normalizePermissionOptions(options)
	query := &ToolPermissionsQuery{
		Id:                  shared.GenerateUUIDv7(),
		Tool:                call,
		Title:               title,
		Details:             details,
		Options:             normalizedOptions,
		AllowCustomResponse: true,
		Meta:                meta,
	}
	return &ToolResponse{
		Call:  call,
		Error: query,
		Done:  true,
	}
}

// NewVFSPermissionQuery builds a standard VFS permission query response.
func NewVFSPermissionQuery(call *ToolCall, path, action, operation string) *ToolResponse {
	details := fmt.Sprintf("Allow %s at path: %s", action, path)
	options := PermissionOptions(
		fmt.Sprintf("Allow in %s*", filepath.Dir(path)),
		fmt.Sprintf("Allow for %s/*", path),
	)
	return NewPermissionQuery(call, PermissionTitleRequired, details, options, map[string]string{
		"type":      "vfs",
		"path":      path,
		"operation": operation,
	})
}

// PermissionOptions returns the default options with any extras appended.
func PermissionOptions(extra ...string) []string {
	options := []string{PermissionOptionAllow, PermissionOptionDeny}
	if len(extra) == 0 {
		return options
	}
	return append(options, extra...)
}

func normalizePermissionOptions(options []string) []string {
	if len(options) == 0 {
		return PermissionOptions()
	}
	if len(options) >= 2 && options[0] == PermissionOptionAllow && options[1] == PermissionOptionDeny {
		return options
	}
	normalized := PermissionOptions()
	for _, option := range options {
		if option == PermissionOptionAllow || option == PermissionOptionDeny {
			continue
		}
		normalized = append(normalized, option)
	}
	return normalized
}

func vfsActionFromOperation(operation string) string {
	switch operation {
	case "read":
		return "reading file"
	case "write":
		return "writing to file"
	case "delete":
		return "deleting file"
	case "list":
		return "listing files"
	case "find":
		return "finding files"
	case "move":
		return "moving file"
	default:
		return operation
	}
}
