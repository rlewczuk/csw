package tool

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/runner"
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
	runner         runner.CommandRunner
	privileges     map[string]conf.AccessFlag
	sessionWorkdir string
	defaultTimeout time.Duration
}

// NewRunBashTool creates a new RunBashTool instance.
// runner is the CommandRunner to use for executing commands.
// privileges is a map of command regex patterns to access flags.
func NewRunBashTool(r runner.CommandRunner, privileges map[string]conf.AccessFlag, defaultTimeout ...time.Duration) *RunBashTool {
	timeout := time.Duration(0)
	if len(defaultTimeout) > 0 {
		timeout = defaultTimeout[0]
	}

	return &RunBashTool{
		runner:         r,
		privileges:     privileges,
		defaultTimeout: timeout,
	}
}



// NewRunBashToolWithSessionWorkdir creates a new RunBashTool instance with a session workdir.
// runner is the CommandRunner to use for executing commands.
// privileges is a map of command regex patterns to access flags.
// sessionWorkdir is the default working directory for command execution when workdir is not specified.
func NewRunBashToolWithSessionWorkdir(r runner.CommandRunner, privileges map[string]conf.AccessFlag, sessionWorkdir string, defaultTimeout ...time.Duration) *RunBashTool {
	timeout := time.Duration(0)
	if len(defaultTimeout) > 0 {
		timeout = defaultTimeout[0]
	}

	return &RunBashTool{
		runner:         r,
		privileges:     privileges,
		sessionWorkdir: sessionWorkdir,
		defaultTimeout: timeout,
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
		} else if t.sessionWorkdir != "" {
			// Relative path - resolve against session workdir
			resolvedWorkdir = filepath.Join(t.sessionWorkdir, workdir)
		} else {
			// No session workdir set, use the provided relative path
			resolvedWorkdir = workdir
		}
	} else if t.sessionWorkdir != "" {
		// If no workdir provided, use session workdir as default
		resolvedWorkdir = t.sessionWorkdir
	}

	// Parse optional timeout argument
	timeout := t.defaultTimeout
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

	// Parse optional limit argument (default: 200 lines, 0 means no limit)
	limit := 200
	if limitArg, ok := args.Arguments.IntOK("limit"); ok {
		if limitArg < 0 {
			return &ToolResponse{
				Call:  args,
				Error: fmt.Errorf("RunBashTool.Execute() [run.go]: limit must be non-negative, got %d", limitArg),
				Done:  true,
			}
		}
		limit = int(limitArg)
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
		return t.executeCommand(args, command, resolvedWorkdir, timeout, limit)
	default:
		// Default to Ask if not specified
		details := fmt.Sprintf("Allow running command: %s", command)
		if resolvedWorkdir != "" {
			details += fmt.Sprintf("\nWorkdir: %s", resolvedWorkdir)
		}
		if timeout > 0 {
			details += fmt.Sprintf("\nTimeout: %v", timeout)
		}
		if limit != 200 {
			details += fmt.Sprintf("\nLimit: %d", limit)
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
func (t *RunBashTool) executeCommand(args *ToolCall, command string, workdir string, timeout time.Duration, limit int) *ToolResponse {
	var stdout, stderr string
	var exitCode int
	var err error

	// Use detailed method to get separate stdout and stderr
	options := runner.CommandOptions{
		Workdir: workdir,
		Timeout: timeout,
	}
	stdout, stderr, exitCode, err = t.runner.RunCommandWithOptionsDetailed(command, options)

	// Combine stdout and stderr for output field
	output := stdout
	if stderr != "" {
		if output != "" {
			output += "\n"
		}
		output += stderr
	}

	if err != nil {
		if isTimeoutError(err) {
			if limit > 0 {
				output = truncateOutput(output, limit)
			}

			errMessage := fmt.Sprintf("command terminated due to timeout: %v", err)
			if output != "" {
				errMessage = fmt.Sprintf("%s\nPartial output:\n%s", errMessage, output)
			}

			return &ToolResponse{
				Call:  args,
				Error: fmt.Errorf("RunBashTool.executeCommand() [run.go]: %s", errMessage),
				Done:  true,
			}
		}

		// If there's an error from the runner itself (timeout, etc.), return it
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("RunBashTool.executeCommand() [run.go]: %w", err),
			Done:  true,
		}
	}

	// Apply line limit if specified (limit > 0)
	if limit > 0 {
		output = truncateOutput(output, limit)
	}

	// Return the output, stderr, and exit code
	var result ToolValue
	result.Set("output", output)
	result.Set("stdout", stdout)
	result.Set("stderr", stderr)
	result.Set("exit_code", exitCode)

	return &ToolResponse{
		Call:   args,
		Result: result,
		Done:   true,
	}
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(strings.ToLower(err.Error()), "timed out")
}

// Render returns a string representation of the tool call.
func (t *RunBashTool) Render(call *ToolCall) (string, string, map[string]string) {
	command, _ := call.Arguments.StringOK("command")
	exitCode := call.Arguments.Int("exit_code")
	output := call.Arguments.String("output")
	stderr := call.Arguments.String("stderr")

	oneLiner := truncateString("bash: "+command, 128)
	full := command + "\n\n"

	// Check if there was an error (non-zero exit code)
	if exitCode != 0 {
		errorLine := fmt.Sprintf("ERROR: exit code %d", exitCode)

		// Add stderr if it has up to 1 line (no newlines)
		if stderr != "" && !strings.Contains(stderr, "\n") {
			errorLine += ", " + stderr
		}

		// For one-liner, append error info
		oneLiner = truncateString("bash: "+command+" ("+errorLine+")", 128)

		// For full output, add error line before output
		full += errorLine + "\n"
	}

	// Add output if available
	if output != "" {
		full += output
	}

	return oneLiner, full, make(map[string]string)
}
