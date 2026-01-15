package tool

import (
	"fmt"
	"regexp"

	"github.com/codesnort/codesnort-swe/pkg/runner"
	"github.com/codesnort/codesnort-swe/pkg/shared"
)

// RunCommandError represents a permission error for running commands.
type RunCommandError struct {
	Command string
	Message string
}

// Error implements the error interface.
func (e *RunCommandError) Error() string {
	return fmt.Sprintf("RunCommandError [run.go]: %s: %s", e.Message, e.Command)
}

// RunBashTool implements the run.bash tool for executing bash commands.
type RunBashTool struct {
	runner     runner.CommandRunner
	privileges map[string]shared.AccessFlag
}

// NewRunBashTool creates a new RunBashTool instance.
// runner is the CommandRunner to use for executing commands.
// privileges is a map of command regex patterns to access flags.
func NewRunBashTool(r runner.CommandRunner, privileges map[string]shared.AccessFlag) *RunBashTool {
	return &RunBashTool{
		runner:     r,
		privileges: privileges,
	}
}

// Info returns information about the tool including its name, description, and argument schema.
func (t *RunBashTool) Info() ToolInfo {
	schema := NewToolSchema()
	schema.AddProperty("command", PropertySchema{
		Type:        SchemaTypeString,
		Description: "The bash command to execute.",
	}, true)

	return ToolInfo{
		Name:        "run.bash",
		Description: "Executes a bash command in the project directory and returns the output.",
		Schema:      schema,
	}
}

// Execute executes the tool with the given arguments and returns the response.
func (t *RunBashTool) Execute(args ToolCall) ToolResponse {
	command, ok := args.Arguments.StringOK("command")
	if !ok {
		return ToolResponse{
			Call:  &args,
			Error: fmt.Errorf("RunBashTool.Execute() [run.go]: missing required argument: command"),
			Done:  true,
		}
	}

	// Check permissions
	access := t.checkPermission(command)

	switch access {
	case shared.AccessDeny:
		return ToolResponse{
			Call: &args,
			Error: &RunCommandError{
				Command: command,
				Message: "permission denied",
			},
			Done: true,
		}
	case shared.AccessAsk:
		return t.createPermissionQuery(args, command)
	case shared.AccessAllow:
		// Execute the command
		return t.executeCommand(args, command)
	default:
		// Default to Ask if not specified
		return t.createPermissionQuery(args, command)
	}
}

// checkPermission checks if the command is allowed based on the privileges.
func (t *RunBashTool) checkPermission(command string) shared.AccessFlag {
	if t.privileges == nil {
		return shared.AccessAsk
	}

	// Try to match against all patterns
	var bestMatch string
	var bestAccess shared.AccessFlag
	bestSpecificity := -1

	for pattern, access := range t.privileges {
		// Calculate specificity (length of pattern without wildcards)
		specificity := len(pattern) - countWildcards(pattern)

		matched, err := regexp.MatchString(pattern, command)
		if err != nil {
			// Invalid regex, skip
			continue
		}

		if matched {
			// Use the most specific match
			if specificity > bestSpecificity {
				bestMatch = pattern
				bestAccess = access
				bestSpecificity = specificity
			}
		}
	}

	// If we found a match, return it
	if bestMatch != "" {
		return bestAccess
	}

	// No match found, return Ask
	return shared.AccessAsk
}

// countWildcards counts the number of wildcard characters in a pattern.
func countWildcards(pattern string) int {
	count := 0
	for _, ch := range pattern {
		if ch == '*' || ch == '?' || ch == '.' {
			count++
		}
	}
	return count
}

// createPermissionQuery creates a permission query for the user.
func (t *RunBashTool) createPermissionQuery(args ToolCall, command string) ToolResponse {
	query := &ToolPermissionsQuery{
		Id:      shared.GenerateUUIDv7(),
		Tool:    &args,
		Title:   "Permission Required",
		Details: fmt.Sprintf("Allow running command: %s", command),
		Options: []string{
			"Allow",
			"Deny",
			"Allow and remember (add to privileges)",
		},
		AllowCustomResponse: true,
		Meta: map[string]string{
			"type":    "run",
			"command": command,
		},
	}
	return ToolResponse{
		Call:  &args,
		Error: query,
		Done:  true,
	}
}

// executeCommand executes the command using the runner.
func (t *RunBashTool) executeCommand(args ToolCall, command string) ToolResponse {
	output, exitCode, err := t.runner.RunCommand(command)

	if err != nil {
		// If there's an error from the runner itself (timeout, etc.), return it
		return ToolResponse{
			Call:  &args,
			Error: fmt.Errorf("RunBashTool.executeCommand() [run.go]: %w", err),
			Done:  true,
		}
	}

	// Return the output and exit code
	var result ToolValue
	result.Set("output", output)
	result.Set("exit_code", exitCode)

	return ToolResponse{
		Call:   &args,
		Result: result,
		Done:   true,
	}
}
