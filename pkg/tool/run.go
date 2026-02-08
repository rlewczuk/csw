package tool

import (
	"fmt"
	"path/filepath"
	"regexp"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/runner"
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
func (t *RunBashTool) Execute(args *ToolCall) *ToolResponse {
	command, ok := args.Arguments.StringOK("command")
	if !ok {
		return &ToolResponse{
			Call:  args,
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
			return &ToolResponse{
				Call:  args,
				Error: fmt.Errorf("RunBashTool.Execute() [run.go]: timeout must be positive, got %d", timeoutSecs),
				Done:  true,
			}
		}
		timeout = time.Duration(timeoutSecs) * time.Second
	}

	// Check permissions for absolute workdir
	if needsPermission {
		// Check if explicit access is granted
		if args.Access == conf.AccessAllow {
			// Proceed
		} else if args.Access == conf.AccessDeny {
			return &ToolResponse{
				Call: args,
				Error: &RunCommandError{
					Command: command,
					Message: "permission denied for absolute path",
				},
				Done: true,
			}
		} else {
			details := fmt.Sprintf("Allow running command with absolute path:\nCommand: %s\nWorkdir: %s", command, resolvedWorkdir)
			if timeout > 0 {
				details += fmt.Sprintf("\nTimeout: %v", timeout)
			}
			return NewPermissionQuery(args, PermissionTitleAbsolutePath, details, PermissionOptions(), map[string]string{
				"type":    "run_absolute_workdir",
				"command": command,
				"workdir": resolvedWorkdir,
			})
		}
	}

	// Check permissions for command
	access := t.checkPermission(command)

	// Override with explicit access from tool call if set
	if args.Access != "" && args.Access != conf.AccessAuto {
		access = args.Access
	}

	switch access {
	case conf.AccessDeny:
		return &ToolResponse{
			Call: args,
			Error: &RunCommandError{
				Command: command,
				Message: "permission denied",
			},
			Done: true,
		}
	case conf.AccessAsk:
		details := fmt.Sprintf("Allow running command: %s", command)
		if resolvedWorkdir != "" {
			details += fmt.Sprintf("\nWorkdir: %s", resolvedWorkdir)
		}
		if timeout > 0 {
			details += fmt.Sprintf("\nTimeout: %v", timeout)
		}
		return NewPermissionQuery(args, PermissionTitleRequired, details, PermissionOptions(PermissionOptionAllowRemember), map[string]string{
			"type":    "run",
			"command": command,
		})
	case conf.AccessAllow:
		// Execute the command
		return t.executeCommand(args, command, resolvedWorkdir, timeout)
	default:
		// Default to Ask if not specified
		details := fmt.Sprintf("Allow running command: %s", command)
		if resolvedWorkdir != "" {
			details += fmt.Sprintf("\nWorkdir: %s", resolvedWorkdir)
		}
		if timeout > 0 {
			details += fmt.Sprintf("\nTimeout: %v", timeout)
		}
		return NewPermissionQuery(args, PermissionTitleRequired, details, PermissionOptions(PermissionOptionAllowRemember), map[string]string{
			"type":    "run",
			"command": command,
		})
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

// executeCommand executes the command using the runner.
func (t *RunBashTool) executeCommand(args *ToolCall, command string, workdir string, timeout time.Duration) *ToolResponse {
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
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("RunBashTool.executeCommand() [run.go]: %w", err),
			Done:  true,
		}
	}

	// Return the output and exit code
	var result ToolValue
	result.Set("output", output)
	result.Set("exit_code", exitCode)

	return &ToolResponse{
		Call:   args,
		Result: result,
		Done:   true,
	}
}

// Display returns a string representation of the tool call.
func (t *RunBashTool) Display(mode DisplayMode, color bool) (string, map[string]string) {
	return "runBash", make(map[string]string)
}
