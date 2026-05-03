package tool

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/runner"
)

const (
	defaultRunBashMaxOutputBytes = 2048
	runBashWorktmpDir            = ".cswdata/worktmp"
	runBashOutputTempPattern     = "runbash-output-*.txt"
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
	logger         *slog.Logger
}

func (t *RunBashTool) GetDescription() (string, bool) {
	return "", false
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

// SetLogger sets the logger for the tool.
func (t *RunBashTool) SetLogger(logger *slog.Logger) {
	t.logger = logger
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

	maxOutput := defaultRunBashMaxOutputBytes
	if maxOutputArg, ok := args.Arguments.IntOK("max_output"); ok {
		if maxOutputArg < 0 {
			return &ToolResponse{
				Call:  args,
				Error: fmt.Errorf("RunBashTool.Execute() [run.go]: max_output must be non-negative, got %d", maxOutputArg),
				Done:  true,
			}
		}
		maxOutput = int(maxOutputArg)
	}

	background := -1
	if backgroundArg, ok := args.Arguments.IntOK("background"); ok {
		if backgroundArg < 0 {
			return &ToolResponse{
				Call:  args,
				Error: fmt.Errorf("RunBashTool.Execute() [run.go]: background must be non-negative, got %d", backgroundArg),
				Done:  true,
			}
		}
		background = int(backgroundArg)
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
			details := fmt.Sprintf("running command with absolute path denied: command=%s workdir=%s", command, resolvedWorkdir)
			return NewPermissionDeniedResponse(args, details)
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
		return NewPermissionDeniedResponse(args, fmt.Sprintf("running command denied: %s", command))
	case conf.AccessAllow:
		// Execute the command
		return t.executeCommand(args, command, resolvedWorkdir, timeout, maxOutput, background)
	default:
		return NewPermissionDeniedResponse(args, fmt.Sprintf("running command denied: %s", command))
	}
}

// checkPermission checks if the command is allowed based on the privileges.
func (t *RunBashTool) checkPermission(command string) conf.AccessFlag {
	if t.privileges == nil {
		return conf.AccessDeny
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

	// No match found, return deny
	return conf.AccessDeny
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
func (t *RunBashTool) executeCommand(args *ToolCall, command string, workdir string, timeout time.Duration, maxOutput int, background int) *ToolResponse {
	var stdout, stderr string
	var exitCode int
	var pid int
	var stillRunning bool
	var err error

	// Use detailed method to get separate stdout and stderr
	options := runner.CommandOptions{
		Workdir: workdir,
		Timeout: timeout,
	}
	if background >= 0 {
		stdout, stderr, exitCode, pid, stillRunning, err = t.runner.RunCommandWithOptionsBackgroundDetailed(command, options, time.Duration(background)*time.Second)
	} else {
		stdout, stderr, exitCode, err = t.runner.RunCommandWithOptionsDetailed(command, options)
	}

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

	if background >= 0 {
		output, stdout, stderr, outputFile, spillErr := t.safeguardOutput(output, stdout, stderr, maxOutput, workdir)
		if spillErr != nil {
			return &ToolResponse{
				Call:  args,
				Error: fmt.Errorf("RunBashTool.executeCommand() [run.go]: %w", spillErr),
				Done:  true,
			}
		}

		var result ToolValue
		result.Set("output", output)
		result.Set("stdout", stdout)
		result.Set("stderr", stderr)
		result.Set("exit_code", exitCode)
		result.Set("pid", pid)
		result.Set("running", stillRunning)
		setRunBashOutputMetadata(&result, output, maxOutput, args.Arguments, outputFile)
		if stillRunning {
			result.Set("status", "running")
		} else {
			result.Set("status", "finished")
		}

		return &ToolResponse{
			Call:   args,
			Result: result,
			Done:   true,
		}
	}

	// Return the output, stderr, and exit code
	output, stdout, stderr, outputFile, spillErr := t.safeguardOutput(output, stdout, stderr, maxOutput, workdir)
	if spillErr != nil {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("RunBashTool.executeCommand() [run.go]: %w", spillErr),
			Done:  true,
		}
	}

	var result ToolValue
	result.Set("output", output)
	result.Set("stdout", stdout)
	result.Set("stderr", stderr)
	result.Set("exit_code", exitCode)
	setRunBashOutputMetadata(&result, output, maxOutput, args.Arguments, outputFile)

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

func setRunBashOutputMetadata(result *ToolValue, output string, maxOutput int, args ToolValue, outputFile string) {
	if result == nil {
		return
	}

	result.Set("output_bytes", len([]byte(output)))
	if _, ok := args.IntOK("max_output"); ok && maxOutput != defaultRunBashMaxOutputBytes {
		result.Set("max_output", maxOutput)
	}
	if outputFile != "" {
		result.Set("max_output_triggered", true)
		result.Set("output_file", filepath.Base(outputFile))
	}
}

func (t *RunBashTool) safeguardOutput(output string, stdout string, stderr string, maxOutput int, workdir string) (string, string, string, string, error) {
	if maxOutput <= 0 || len([]byte(output)) <= maxOutput {
		return output, stdout, stderr, "", nil
	}

	path, err := t.saveLargeOutput(output, workdir)
	if err != nil {
		return "", "", "", "", err
	}

	message := fmt.Sprintf("Output was too big (%d bytes; max_output=%d bytes). It was saved instead of returned in full. It should be grepped or processed with tools/scripts, or only partially read rather than read in its entirety.\nSaved full output to temporary file: %s", len([]byte(output)), maxOutput, path)
	return message, message, "", path, nil
}

func (t *RunBashTool) saveLargeOutput(output string, workdir string) (string, error) {
	baseDir := workdir
	if baseDir == "" {
		baseDir = t.sessionWorkdir
	}
	if baseDir == "" {
		baseDir = "."
	}

	tempDir := filepath.Join(baseDir, runBashWorktmpDir)
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return "", fmt.Errorf("RunBashTool.saveLargeOutput() [run.go]: failed to create worktmp directory: %w", err)
	}

	tempFile, err := os.CreateTemp(tempDir, runBashOutputTempPattern)
	if err != nil {
		return "", fmt.Errorf("RunBashTool.saveLargeOutput() [run.go]: failed to create output file: %w", err)
	}
	defer tempFile.Close()

	if _, err := tempFile.WriteString(output); err != nil {
		return "", fmt.Errorf("RunBashTool.saveLargeOutput() [run.go]: failed to write output file: %w", err)
	}

	return tempFile.Name(), nil
}

func runBashRenderMetaMap(meta runBashRenderMetadata) map[string]string {
	result := map[string]string{
		"output_bytes": fmt.Sprintf("%d", meta.outputBytes),
	}
	if meta.hasMaxOutput {
		result["max_output"] = fmt.Sprintf("%d", meta.maxOutput)
	}
	if meta.maxOutputTriggered {
		result["max_output_triggered"] = "true"
	}
	if meta.outputFile != "" {
		result["output_file"] = meta.outputFile
	}
	return result
}

type runBashRenderMetadata struct {
	outputBytes        int
	hasMaxOutput       bool
	maxOutput          int64
	maxOutputTriggered bool
	outputFile         string
}

func buildRunBashRenderMetadata(args ToolValue, output string) (runBashRenderMetadata, map[string]any) {
	meta := runBashRenderMetadata{
		outputBytes: len([]byte(output)),
	}
	if maxOutput, ok := args.IntOK("max_output"); ok && maxOutput != defaultRunBashMaxOutputBytes {
		meta.hasMaxOutput = true
		meta.maxOutput = maxOutput
	}
	if triggered, ok := args.BoolOK("max_output_triggered"); ok && triggered {
		meta.maxOutputTriggered = true
	}
	if outputFile := strings.TrimSpace(args.String("output_file")); outputFile != "" {
		meta.outputFile = filepath.Base(outputFile)
	}

	jsonMeta := map[string]any{
		"output_bytes": meta.outputBytes,
	}
	if meta.hasMaxOutput {
		jsonMeta["max_output"] = meta.maxOutput
	}
	if meta.maxOutputTriggered {
		jsonMeta["max_output_triggered"] = true
	}
	if meta.outputFile != "" {
		jsonMeta["output_file"] = meta.outputFile
	}
	return meta, jsonMeta
}

func renderRunBashSummary(command string, errorLine string, meta runBashRenderMetadata) string {
	base := "bash: " + command
	if errorLine != "" {
		base += " (" + errorLine + ")"
	}
	metadata := runBashMetadataText(meta, true)
	if metadata == "" {
		return truncateString(base, 128)
	}

	metadata = " (" + metadata + ")"
	maxBaseLen := 128 - len(metadata)
	if maxBaseLen < 1 {
		return strings.TrimSpace(metadata)
	}
	return truncateString(base, maxBaseLen) + metadata
}

func appendRunBashRenderMetadata(full string, meta runBashRenderMetadata) string {
	metadata := runBashMetadataText(meta, false)
	if metadata == "" {
		return full
	}
	if full != "" && !strings.HasSuffix(full, "\n") {
		full += "\n"
	}
	return full + "Output metadata: " + metadata + "\n"
}

func runBashMetadataText(meta runBashRenderMetadata, short bool) string {
	parts := []string{fmt.Sprintf("%d bytes returned", meta.outputBytes)}
	if short {
		parts[0] = fmt.Sprintf("output: %d bytes", meta.outputBytes)
	}
	if meta.hasMaxOutput {
		parts = append(parts, fmt.Sprintf("max_output: %d bytes", meta.maxOutput))
	}
	if meta.maxOutputTriggered {
		parts = append(parts, "max_output limit triggered")
	}
	if meta.outputFile != "" {
		parts = append(parts, "file: "+meta.outputFile)
	}
	return strings.Join(parts, "; ")
}

// Render returns a string representation of the tool call.
func (t *RunBashTool) Render(call *ToolCall) (string, string, string, map[string]string) {
	command, _ := call.Arguments.StringOK("command")
	exitCode := call.Arguments.Int("exit_code")
	stdout := call.Arguments.String("stdout")
	output := call.Arguments.String("output")
	stderr := call.Arguments.String("stderr")
	renderErr := call.Arguments.String("error")
	renderMeta, jsonMeta := buildRunBashRenderMetadata(call.Arguments, output)

	oneLiner := renderRunBashSummary(command, "", renderMeta)
	full := command + "\n\n"

	if renderErr != "" {
		errorLine := fmt.Sprintf("ERROR: %s", renderErr)
		oneLiner = renderRunBashSummary(command, errorLine, renderMeta)
		full += errorLine + "\n"
		full = appendRunBashRenderMetadata(full, renderMeta)

		jsonExtra := map[string]any{
			"command": command,
			"error":   renderErr,
		}
		for key, value := range jsonMeta {
			jsonExtra[key] = value
		}
		jsonl := buildToolRenderJSONL("runBash", call, jsonExtra)

		return oneLiner, full, jsonl, runBashRenderMetaMap(renderMeta)
	}

	// Check if there was an error (non-zero exit code)
	if exitCode != 0 {
		errorLine := fmt.Sprintf("ERROR: exit code %d", exitCode)

		// Add stderr if it has up to 1 line (no newlines)
		if stderr != "" && !strings.Contains(stderr, "\n") {
			errorLine += ", " + stderr
		}

		// For one-liner, append error info
		oneLiner = renderRunBashSummary(command, errorLine, renderMeta)

		// For full output, add error line before output
		full += errorLine + "\n"
	}

	if stdout != "" {
		full += "STDOUT:\n" + stdout
		if !strings.HasSuffix(stdout, "\n") {
			full += "\n"
		}
	}

	if stderr != "" {
		if stdout != "" {
			full += "\n"
		}
		full += "STDERR:\n" + stderr
		if !strings.HasSuffix(stderr, "\n") {
			full += "\n"
		}
	}

	if stdout == "" && stderr == "" && output != "" {
		full += output
	}
	full = appendRunBashRenderMetadata(full, renderMeta)

	jsonExtra := map[string]any{
		"command":   command,
		"exit_code": exitCode,
		"stderr":    stderr,
		"output":    output,
	}
	for key, value := range jsonMeta {
		jsonExtra[key] = value
	}
	jsonl := buildToolRenderJSONL("runBash", call, jsonExtra)

	return oneLiner, full, jsonl, runBashRenderMetaMap(renderMeta)
}
