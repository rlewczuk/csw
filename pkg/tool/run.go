package tool

import (
	"fmt"
	"path/filepath"
	"regexp"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
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

// RunBashTool implements the runBash tool for executing bash commands.
type RunBashTool struct {
	runner      runner.CommandRunner
	privileges  map[string]conf.AccessFlag
	projectRoot string
}

// NewRunBashTool creates a new RunBashTool instance.
// runner is the CommandRunner to use for executing commands.
// privileges is a map of command regex patterns to access flags.
func NewRunBashTool(r runner.CommandRunner, privileges map[string]conf.AccessFlag) *RunBashTool {
	return &RunBashTool{
		runner:     r,
		privileges: privileges,
	}
}

// NewRunBashToolWithRoot creates a new RunBashTool instance with a project root.
// runner is the CommandRunner to use for executing commands.
// privileges is a map of command regex patterns to access flags.
// projectRoot is the project root directory for resolving relative paths and checking absolute paths.
func NewRunBashToolWithRoot(r runner.CommandRunner, privileges map[string]conf.AccessFlag, projectRoot string) *RunBashTool {
	return &RunBashTool{
		runner:      r,
		privileges:  privileges,
		projectRoot: projectRoot,
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

	// Parse optional workdir argument
	workdir := args.Arguments.String("workdir")
	var resolvedWorkdir string
	needsPermission := false

	if workdir != "" {
		if filepath.IsAbs(workdir) {
			// Absolute path requires permission
			resolvedWorkdir = workdir
			needsPermission = true
		} else if t.projectRoot != "" {
			// Relative path - resolve against project root
			resolvedWorkdir = filepath.Join(t.projectRoot, workdir)
		} else {
			// No project root set, use the provided relative path
			resolvedWorkdir = workdir
		}
	}

	// Parse optional timeout argument
	timeout := time.Duration(0)
	if timeoutSecs, ok := args.Arguments.IntOK("timeout"); ok {
		if timeoutSecs <= 0 {
			return ToolResponse{
				Call:  &args,
				Error: fmt.Errorf("RunBashTool.Execute() [run.go]: timeout must be positive, got %d", timeoutSecs),
				Done:  true,
			}
		}
		timeout = time.Duration(timeoutSecs) * time.Second
	}

	// Check permissions for absolute workdir
	if needsPermission {
		return t.createWorkdirPermissionQuery(args, command, resolvedWorkdir, timeout)
	}

	// Check permissions for command
	access := t.checkPermission(command)

	switch access {
	case conf.AccessDeny:
		return ToolResponse{
			Call: &args,
			Error: &RunCommandError{
				Command: command,
				Message: "permission denied",
			},
			Done: true,
		}
	case conf.AccessAsk:
		return t.createPermissionQuery(args, command, resolvedWorkdir, timeout)
	case conf.AccessAllow:
		// Execute the command
		return t.executeCommand(args, command, resolvedWorkdir, timeout)
	default:
		// Default to Ask if not specified
		return t.createPermissionQuery(args, command, resolvedWorkdir, timeout)
	}
}

// checkPermission checks if the command is allowed based on the privileges.
func (t *RunBashTool) checkPermission(command string) conf.AccessFlag {
	if t.privileges == nil {
		return conf.AccessAsk
	}

	// Try to match against all patterns
	var bestMatch string
	var bestAccess conf.AccessFlag
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
	return conf.AccessAsk
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
func (t *RunBashTool) createPermissionQuery(args ToolCall, command string, workdir string, timeout time.Duration) ToolResponse {
	details := fmt.Sprintf("Allow running command: %s", command)
	if workdir != "" {
		details += fmt.Sprintf("\nWorkdir: %s", workdir)
	}
	if timeout > 0 {
		details += fmt.Sprintf("\nTimeout: %v", timeout)
	}

	query := &ToolPermissionsQuery{
		Id:      shared.GenerateUUIDv7(),
		Tool:    &args,
		Title:   "Permission Required",
		Details: details,
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

// createWorkdirPermissionQuery creates a permission query for absolute workdir.
func (t *RunBashTool) createWorkdirPermissionQuery(args ToolCall, command string, workdir string, timeout time.Duration) ToolResponse {
	details := fmt.Sprintf("Allow running command with absolute path:\nCommand: %s\nWorkdir: %s", command, workdir)
	if timeout > 0 {
		details += fmt.Sprintf("\nTimeout: %v", timeout)
	}

	query := &ToolPermissionsQuery{
		Id:      shared.GenerateUUIDv7(),
		Tool:    &args,
		Title:   "Permission Required for Absolute Path",
		Details: details,
		Options: []string{
			"Allow",
			"Deny",
		},
		AllowCustomResponse: true,
		Meta: map[string]string{
			"type":    "run_absolute_workdir",
			"command": command,
			"workdir": workdir,
		},
	}
	return ToolResponse{
		Call:  &args,
		Error: query,
		Done:  true,
	}
}

// executeCommand executes the command using the runner.
func (t *RunBashTool) executeCommand(args ToolCall, command string, workdir string, timeout time.Duration) ToolResponse {
	var output string
	var exitCode int
	var err error

	if workdir == "" && timeout == 0 {
		// Use default method if no options
		output, exitCode, err = t.runner.RunCommand(command)
	} else {
		// Use options method
		options := runner.CommandOptions{
			Workdir: workdir,
			Timeout: timeout,
		}
		output, exitCode, err = t.runner.RunCommandWithOptions(command, options)
	}

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
