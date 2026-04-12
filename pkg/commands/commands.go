package commands

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/rlewczuk/csw/pkg/runner"
	"gopkg.in/yaml.v3"
)

var positionalArgPattern = regexp.MustCompile(`\$(\d+)`)
var shellPattern = regexp.MustCompile("!`([^`]+)`")
var defaultScriptPattern = regexp.MustCompile("(^|\\s)!([^\\s!`][^\\s`]*)")
var hostScriptPattern = regexp.MustCompile("(^|\\s)!!([^\\s`]*)")
var filePattern = regexp.MustCompile(`(^|\s)@([^\s]+)`)

const embeddedCommandsDir = "data"

//go:embed all:data
var embeddedCommandsFS embed.FS

// Metadata describes supported command frontmatter fields.
type Metadata struct {
	Description string `yaml:"description"`
	Agent       string `yaml:"agent"`
	Model       string `yaml:"model"`
}

// Command stores loaded command definition.
type Command struct {
	Name     string
	Path     string
	Metadata Metadata
	Template string
}

// Invocation stores parsed slash-command invocation.
type Invocation struct {
	Name      string
	Arguments []string
}

// ParseInvocation parses slash command invocation from prompt and extra CLI args.
func ParseInvocation(prompt string, extraArgs []string) (*Invocation, bool, error) {
	trimmed := strings.TrimSpace(prompt)
	if !strings.HasPrefix(trimmed, "/") {
		return nil, false, nil
	}

	tokens, err := splitCommandLine(trimmed)
	if err != nil {
		return nil, true, fmt.Errorf("ParseInvocation() [commands.go]: failed to parse command invocation: %w", err)
	}
	if len(tokens) == 0 {
		return nil, true, fmt.Errorf("ParseInvocation() [commands.go]: command name cannot be empty")
	}

	name := strings.TrimPrefix(tokens[0], "/")
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, true, fmt.Errorf("ParseInvocation() [commands.go]: command name cannot be empty")
	}

	args := make([]string, 0, len(tokens)-1+len(extraArgs))
	if len(tokens) > 1 {
		args = append(args, tokens[1:]...)
	}
	if len(extraArgs) > 0 {
		args = append(args, extraArgs...)
	}

	return &Invocation{Name: name, Arguments: args}, true, nil
}

// LoadFromDir loads command definition from commands directory.
func LoadFromDir(commandsDir string, name string) (*Command, error) {
	trimmedName, err := normalizeCommandName(name)
	if err != nil {
		return nil, err
	}

	localPath := filepath.Join(commandsDir, filepath.FromSlash(trimmedName)+".md")
	content, err := os.ReadFile(localPath)
	resolvedPath := localPath
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("LoadFromDir() [commands.go]: failed to read command file %q: %w", localPath, err)
		}

		embeddedPath := path.Join(embeddedCommandsDir, trimmedName+".md")
		content, err = fs.ReadFile(embeddedCommandsFS, embeddedPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil, fmt.Errorf("LoadFromDir() [commands.go]: command %q not found in %q and embedded %q", trimmedName, commandsDir, embeddedCommandsDir)
			}

			return nil, fmt.Errorf("LoadFromDir() [commands.go]: failed to read embedded command file %q: %w", embeddedPath, err)
		}
		resolvedPath = "embedded:" + embeddedPath
	}

	metadata, template := parseFrontmatter(string(content))
	if strings.TrimSpace(template) == "" {
		return nil, fmt.Errorf("LoadFromDir() [commands.go]: command %q template is empty", trimmedName)
	}

	return &Command{
		Name:     trimmedName,
		Path:     resolvedPath,
		Metadata: metadata,
		Template: template,
	}, nil
}

func normalizeCommandName(name string) (string, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return "", fmt.Errorf("LoadFromDir() [commands.go]: command name cannot be empty")
	}

	normalized := strings.ReplaceAll(trimmedName, "\\", "/")
	normalized = strings.TrimPrefix(normalized, "/")
	normalized = path.Clean(normalized)
	if normalized == "." || normalized == "" {
		return "", fmt.Errorf("LoadFromDir() [commands.go]: command name cannot be empty")
	}

	segments := strings.Split(normalized, "/")
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("LoadFromDir() [commands.go]: command name %q is invalid", name)
		}
	}

	return normalized, nil
}

// ApplyArguments replaces $ARGUMENTS and positional placeholders in template.
func ApplyArguments(template string, args []string) string {
	rendered := strings.ReplaceAll(template, "$ARGUMENTS", strings.Join(args, " "))
	rendered = positionalArgPattern.ReplaceAllStringFunc(rendered, func(match string) string {
		group := positionalArgPattern.FindStringSubmatch(match)
		if len(group) != 2 {
			return match
		}
		index, err := strconv.Atoi(group[1])
		if err != nil || index <= 0 || index > len(args) {
			return ""
		}
		return args[index-1]
	})

	return rendered
}

// ExpandPrompt expands shell and file references in rendered command template.
func ExpandPrompt(prompt string, workDir string, shellRunner runner.CommandRunner, hostShellRunner runner.CommandRunner) (string, error) {
	withShell, err := expandShellExpressions(prompt, workDir, shellRunner)
	if err != nil {
		return "", err
	}

	withScripts, err := expandScripts(withShell, workDir, shellRunner, hostShellRunner)
	if err != nil {
		return "", err
	}

	withFiles, err := expandFiles(withScripts, workDir)
	if err != nil {
		return "", err
	}

	return withFiles, nil
}

// HasDefaultRuntimeShellExpansion reports whether prompt contains expansion that uses default runtime runner.
func HasDefaultRuntimeShellExpansion(prompt string) bool {
	if strings.Contains(prompt, "!`") {
		return true
	}

	return defaultScriptPattern.FindStringIndex(prompt) != nil
}

func expandShellExpressions(prompt string, workDir string, shellRunner runner.CommandRunner) (string, error) {
	if !strings.Contains(prompt, "!`") {
		return prompt, nil
	}
	if shellRunner == nil {
		return "", fmt.Errorf("expandShell() [commands.go]: shell runner is nil")
	}

	matches := shellPattern.FindAllStringSubmatchIndex(prompt, -1)
	if len(matches) == 0 {
		return prompt, nil
	}

	var out strings.Builder
	last := 0
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		out.WriteString(prompt[last:match[0]])
		command := prompt[match[2]:match[3]]
		output, code, err := shellRunner.RunCommandWithOptions(command, runner.CommandOptions{Workdir: workDir})
		if err != nil {
			return "", fmt.Errorf("expandShellExpressions() [commands.go]: failed to run shell command %q: %w", command, err)
		}
		if code != 0 {
			return "", fmt.Errorf("expandShellExpressions() [commands.go]: shell command %q failed with exit code %d: %s", command, code, strings.TrimSpace(output))
		}
		out.WriteString(strings.TrimRight(output, "\n"))
		last = match[1]
	}
	out.WriteString(prompt[last:])

	return out.String(), nil
}

func expandScripts(prompt string, workDir string, shellRunner runner.CommandRunner, hostShellRunner runner.CommandRunner) (string, error) {
	withHostScripts, err := expandScriptPattern(prompt, hostScriptPattern, workDir, hostShellRunner, "!!")
	if err != nil {
		return "", err
	}

	withDefaultScripts, err := expandScriptPattern(withHostScripts, defaultScriptPattern, workDir, shellRunner, "!")
	if err != nil {
		return "", err
	}

	return withDefaultScripts, nil
}

func expandScriptPattern(prompt string, pattern *regexp.Regexp, workDir string, shellRunner runner.CommandRunner, marker string) (string, error) {
	if !strings.Contains(prompt, marker) {
		return prompt, nil
	}
	if shellRunner == nil {
		return "", fmt.Errorf("expandScriptPattern() [commands.go]: shell runner is nil for %s scripts", marker)
	}

	matches := pattern.FindAllStringSubmatchIndex(prompt, -1)
	if len(matches) == 0 {
		return prompt, nil
	}

	var out strings.Builder
	last := 0
	for _, match := range matches {
		if len(match) < 6 {
			continue
		}

		out.WriteString(prompt[last:match[0]])
		prefix := prompt[match[2]:match[3]]
		rawPath := prompt[match[4]:match[5]]
		scriptPath := strings.TrimSpace(strings.TrimRight(rawPath, ",.;:"))
		if scriptPath == "" {
			out.WriteString(prompt[match[0]:match[1]])
			last = match[1]
			continue
		}

		command := buildScriptCommand(scriptPath)
		output, code, err := shellRunner.RunCommandWithOptions(command, runner.CommandOptions{Workdir: workDir})
		if err != nil {
			return "", fmt.Errorf("expandScriptPattern() [commands.go]: failed to run %s script %q: %w", marker, scriptPath, err)
		}
		if code != 0 {
			return "", fmt.Errorf("expandScriptPattern() [commands.go]: %s script %q failed with exit code %d: %s", marker, scriptPath, code, strings.TrimSpace(output))
		}

		out.WriteString(prefix)
		out.WriteString(strings.TrimRight(output, "\n"))
		last = match[1]
	}

	out.WriteString(prompt[last:])
	return out.String(), nil
}

func buildScriptCommand(path string) string {
	return "bash " + shellQuote(path)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func expandFiles(prompt string, workDir string) (string, error) {
	if !strings.Contains(prompt, "@") {
		return prompt, nil
	}

	matches := filePattern.FindAllStringSubmatchIndex(prompt, -1)
	if len(matches) == 0 {
		return prompt, nil
	}

	var out strings.Builder
	last := 0
	for _, match := range matches {
		if len(match) < 6 {
			continue
		}
		out.WriteString(prompt[last:match[0]])
		prefix := prompt[match[2]:match[3]]
		rawPath := prompt[match[4]:match[5]]
		resolvedPath := strings.TrimSpace(strings.TrimRight(rawPath, ",.;:"))
		if resolvedPath == "" {
			out.WriteString(prompt[match[0]:match[1]])
			last = match[1]
			continue
		}

		readPath := resolvedPath
		if !filepath.IsAbs(readPath) {
			readPath = filepath.Join(workDir, resolvedPath)
		}
		content, err := os.ReadFile(readPath)
		if err != nil {
			return "", fmt.Errorf("expandFiles() [commands.go]: failed to read referenced file %q: %w", resolvedPath, err)
		}
		out.WriteString(prefix)
		out.WriteString(string(content))
		last = match[1]
	}
	out.WriteString(prompt[last:])

	return out.String(), nil
}

func parseFrontmatter(content string) (Metadata, string) {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return Metadata{}, content
	}

	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return Metadata{}, content
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return Metadata{}, content
	}

	frontmatterText := strings.Join(lines[1:end], "\n")
	body := strings.Join(lines[end+1:], "\n")

	var metadata Metadata
	if err := yaml.Unmarshal([]byte(frontmatterText), &metadata); err != nil {
		return Metadata{}, content
	}

	return metadata, body
}

func splitCommandLine(input string) ([]string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, nil
	}

	args := make([]string, 0)
	var current strings.Builder
	inSingle := false
	inDouble := false
	escape := false

	flush := func() {
		if current.Len() == 0 {
			return
		}
		args = append(args, current.String())
		current.Reset()
	}

	for _, r := range trimmed {
		if escape {
			current.WriteRune(r)
			escape = false
			continue
		}

		switch r {
		case '\\':
			escape = true
		case '\'':
			if inDouble {
				current.WriteRune(r)
				continue
			}
			inSingle = !inSingle
		case '"':
			if inSingle {
				current.WriteRune(r)
				continue
			}
			inDouble = !inDouble
		case ' ', '\t', '\n':
			if inSingle || inDouble {
				current.WriteRune(r)
				continue
			}
			flush()
		default:
			current.WriteRune(r)
		}
	}

	if escape || inSingle || inDouble {
		return nil, fmt.Errorf("splitCommandLine() [commands.go]: unterminated quoted command")
	}

	flush()
	return args, nil
}
