package tool

import (
	"fmt"
	"strings"

	"github.com/rlewczuk/csw/pkg/apis"
)

const (
	PermissionOptionAllow = "Allow"
	PermissionOptionDeny  = "Deny"
)

// NewPermissionDeniedResponse builds a standardized permission denied response.
func NewPermissionDeniedResponse(call *ToolCall, details string) *ToolResponse {
	err := error(apis.ErrPermissionDenied)
	if strings.TrimSpace(details) != "" {
		err = fmt.Errorf("%w: %s", apis.ErrPermissionDenied, strings.TrimSpace(details))
	}
	return &ToolResponse{
		Call:  call,
		Error: fmt.Errorf("NewPermissionDeniedResponse() [permissions.go]: %w", err),
		Done:  true,
	}
}

// NewVFSPermissionDeniedResponse builds a standard VFS permission denied response.
func NewVFSPermissionDeniedResponse(call *ToolCall, path, operation string) *ToolResponse {
	details := strings.TrimSpace(operation)
	if strings.TrimSpace(path) != "" {
		details = fmt.Sprintf("%s at path: %s", strings.TrimSpace(operation), strings.TrimSpace(path))
	}

	return NewPermissionDeniedResponse(call, details)
}

// PermissionOptions returns the default options with any extras appended.
func PermissionOptions(extra ...string) []string {
	options := []string{PermissionOptionAllow, PermissionOptionDeny}
	if len(extra) == 0 {
		return options
	}

	return append(options, extra...)
}
