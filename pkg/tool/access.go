package tool

import (
	"fmt"
	"strings"

	"github.com/codesnort/codesnort-swe/pkg/shared"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
)

// AccessControlTool is a wrapper that controls access to tools based on access flags.
// It checks access permissions before delegating to the underlying tool implementation.
type AccessControlTool struct {
	tool       Tool
	privileges map[string]shared.AccessFlag
}

// NewAccessControlTool creates a new AccessControlTool that wraps the given tool.
// The privileges map specifies access flags for different tool name patterns.
// Supported patterns:
//   - Exact name: "vfs.read" matches only "vfs.read"
//   - Single segment wildcard: "vfs.*" matches "vfs.read", "vfs.write" etc., but not "vfs.local.read"
//   - Multi-segment wildcard: "vfs.**" matches "vfs.read", "vfs.local.read", "vfs.local.impl.read" etc.
//   - Partial match with wildcard: "vfs.r*" matches "vfs.read", "vfs.remove" but not "vfs.write"
//   - Global default: "**" matches any tool name
//
// If multiple patterns match a tool name, the most specific one is used.
// If no pattern matches and there is no "**" default, access is denied.
func NewAccessControlTool(tool Tool, privileges map[string]shared.AccessFlag) *AccessControlTool {
	return &AccessControlTool{
		tool:       tool,
		privileges: privileges,
	}
}

// Info returns the tool information from the underlying tool.
func (a *AccessControlTool) Info() ToolInfo {
	return a.tool.Info()
}

// Execute checks access permissions before executing the underlying tool.
// Returns an error if access is denied.
func (a *AccessControlTool) Execute(call ToolCall) ToolResponse {
	toolName := call.Function
	flag := a.resolveAccessFlag(toolName)

	switch flag {
	case shared.AccessAllow:
		resp := a.tool.Execute(call)
		if resp.Error != nil {
			if perr, ok := resp.Error.(*vfs.PermissionError); ok {
				return ToolResponse{
					Call: &call,
					Error: &ToolPermissionsQuery{
						Id:      call.ID,
						Tool:    &call,
						Title:   "Permission Required",
						Details: fmt.Sprintf("Access to file %s required for %s", perr.Path, perr.Operation),
						Options: []string{"Allow", "Deny"},
						Meta: map[string]string{
							"type":      "vfs",
							"path":      perr.Path,
							"operation": perr.Operation,
						},
					},
					Done: true,
				}
			}
		}
		return resp
	case shared.AccessDeny:
		return ToolResponse{
			Call:  &call,
			Error: fmt.Errorf("access denied for tool: %s", toolName),
			Done:  true,
		}
	case shared.AccessAsk:
		// Return ToolPermissionsQuery as error
		return ToolResponse{
			Call: &call,
			Error: &ToolPermissionsQuery{
				Id:      call.ID, // Use tool call ID as query ID for now, or generate new one
				Tool:    &call,
				Title:   "Permission Required",
				Details: fmt.Sprintf("Tool %s requires permission", toolName),
				Options: []string{"Allow", "Deny"},
			},
			Done: true,
		}
	default:
		return ToolResponse{
			Call:  &call,
			Error: fmt.Errorf("unknown access flag for tool: %s", toolName),
			Done:  true,
		}
	}
}

// resolveAccessFlag determines the access flag for a given tool name.
// It finds the most specific matching pattern and returns its access flag.
// If no pattern matches, it returns AccessDeny as the default.
func (a *AccessControlTool) resolveAccessFlag(toolName string) shared.AccessFlag {
	var bestFlag shared.AccessFlag
	bestSpecificity := -1

	for pattern, flag := range a.privileges {
		if matches, specificity := matchPattern(pattern, toolName); matches {
			if specificity > bestSpecificity {
				bestFlag = flag
				bestSpecificity = specificity
			}
		}
	}

	if bestSpecificity >= 0 {
		return bestFlag
	}

	// No match found, return default deny
	return shared.AccessDeny
}

// SetPermission sets the permission for a specific tool pattern.
func (a *AccessControlTool) SetPermission(pattern string, flag shared.AccessFlag) {
	if a.privileges == nil {
		a.privileges = make(map[string]shared.AccessFlag)
	}
	a.privileges[pattern] = flag
}

// matchPattern checks if a pattern matches a tool name and returns the specificity.
// Higher specificity means a more specific match.
// Returns (matches, specificity) where specificity is used to determine the best match.
func matchPattern(pattern, toolName string) (bool, int) {
	// Handle exact match first (highest specificity)
	if pattern == toolName {
		return true, len(toolName) * 1000
	}

	patternSegments := strings.Split(pattern, ".")
	nameSegments := strings.Split(toolName, ".")

	// Calculate specificity based on exact segments matched
	specificity := 0
	matched := true

	for i := 0; i < len(patternSegments); i++ {
		patternSeg := patternSegments[i]

		// Handle ** (multi-segment wildcard)
		if patternSeg == "**" {
			// ** can match zero or more segments
			// If ** is the last segment, it matches everything remaining
			if i == len(patternSegments)-1 {
				return true, specificity
			}
			// ** in the middle would need more complex logic
			// For simplicity, treat ** as only valid at the end
			return true, specificity
		}

		// Check if we've run out of name segments
		if i >= len(nameSegments) {
			matched = false
			break
		}

		nameSeg := nameSegments[i]

		// Handle * (single segment wildcard)
		if patternSeg == "*" {
			// * matches exactly one segment
			specificity += 10 // Lower specificity than exact match
			continue
		}

		// Handle partial match with wildcards (e.g., "r*", "ba*")
		if strings.Contains(patternSeg, "*") {
			if matchSegmentPattern(patternSeg, nameSeg) {
				// Partial wildcard match has moderate specificity
				specificity += 50 + len(patternSeg) - strings.Count(patternSeg, "*")
				continue
			} else {
				matched = false
				break
			}
		}

		// Exact segment match
		if patternSeg == nameSeg {
			specificity += 100 // High specificity for exact segment
		} else {
			matched = false
			break
		}
	}

	// If pattern has fewer segments than name, check if last segment allows extra
	if matched && len(patternSegments) < len(nameSegments) {
		// Only ** allows matching more segments
		// * and other patterns require exact segment count
		matched = false
	}

	// If pattern has more segments than name (and we didn't hit **), no match
	if matched && len(patternSegments) > len(nameSegments) {
		matched = false
	}

	return matched, specificity
}

// matchSegmentPattern checks if a pattern segment (with *) matches a name segment.
// For example: "r*" matches "read", "ba*" matches "bar" and "baz".
func matchSegmentPattern(pattern, segment string) bool {
	// Split pattern by * and check if segment contains all parts in order
	parts := strings.Split(pattern, "*")

	pos := 0
	for i, part := range parts {
		if part == "" {
			continue
		}

		idx := strings.Index(segment[pos:], part)
		if idx == -1 {
			return false
		}

		// First part must be at the beginning
		if i == 0 && idx != 0 {
			return false
		}

		// Last part must be at the end
		if i == len(parts)-1 {
			if pos+idx+len(part) != len(segment) {
				return false
			}
		}

		pos += idx + len(part)
	}

	return true
}
