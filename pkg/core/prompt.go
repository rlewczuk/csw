package core

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/codesnort/codesnort-swe/pkg/conf"
)

// PromptScanner scans and loads prompt fragments from a source.
type PromptScanner interface {
	// GetFragments generates a prompt for the given tags, role and state.
	// returns map where keys are filenames (dir/file_prefix with role name or 'all' as dir and no extension)
	// and values are unprocessed contents of those files
	GetFragments(tags []string, role *conf.AgentRoleConfig) (map[string]string, error)

	// HasChanged returns true if any of the fragments has changed since last scan.
	HasChanged() bool

	// Close stops watching for changes and releases all resources.
	Close() error
}

// PromptGenerator generates a prompt for the given tags, role and state.
type PromptGenerator interface {
	// GetPrompt generates a prompt for the given tags, role and state.
	// Takes map of fragments from GetFragments, concatenates and processes using text/template it to create final prompt;
	// Also responsible for eventual result caching if any of files has changed or agent state data has changed;
	GetPrompt(tags []string, role *conf.AgentRoleConfig, state *AgentState) (string, error)
}

// FSPromptGenerator implements PromptGenerator interface.
// It accepts one or more PromptScanner instances and merges their fragments.
type FSPromptGenerator struct {
	scanners []PromptScanner
}

// ConfPromptGenerator implements PromptGenerator interface.
// It uses conf.ConfigStore to get prompt fragments from AgentRoleConfig.PromptFragments.
type ConfPromptGenerator struct {
	store conf.ConfigStore
}

// promptFragment represents a single prompt fragment file.
type promptFragment struct {
	order    int
	kind     string // "system" or "tools"
	toolName string // empty for system fragments
	tag      string // "all" for untagged fragments, or specific tag
	filename string
	content  string
	isAll    bool // true if from "all" directory
}

// NewFSPromptGenerator creates a new FSPromptGenerator with the given scanners.
func NewFSPromptGenerator(scanners ...PromptScanner) (*FSPromptGenerator, error) {
	if len(scanners) == 0 {
		return nil, fmt.Errorf("NewFSPromptGenerator() [prompt.go]: at least one scanner is required")
	}

	return &FSPromptGenerator{
		scanners: scanners,
	}, nil
}

// NewConfPromptGenerator creates a new ConfPromptGenerator with the given ConfigStore.
func NewConfPromptGenerator(store conf.ConfigStore) (*ConfPromptGenerator, error) {
	if store == nil {
		return nil, fmt.Errorf("NewConfPromptGenerator() [prompt.go]: store cannot be nil")
	}

	return &ConfPromptGenerator{
		store: store,
	}, nil
}

// parseFilename parses a prompt fragment filename.
// Returns: order, kind, toolName, tag, ok
// Format: <num>-system-<tag>.md or <num>-system.md or <num>-tools-<toolname>-<tag>.md
func parseFilename(filename string) (int, string, string, string, bool) {
	// Remove .md extension
	if !strings.HasSuffix(filename, ".md") {
		return 0, "", "", "", false
	}
	name := strings.TrimSuffix(filename, ".md")

	// Split by dash
	parts := strings.Split(name, "-")
	if len(parts) < 2 {
		return 0, "", "", "", false
	}

	// Parse order
	order, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", "", "", false
	}

	kind := parts[1]

	switch kind {
	case "system":
		// Format: <num>-system-<tag>.md or <num>-system.md
		tag := "all"
		if len(parts) > 2 {
			tag = strings.Join(parts[2:], "-")
		}
		return order, kind, "", tag, true

	case "tools":
		// Format: <num>-tools-<toolname>-<tag>.md or <num>-tools-<toolname>.md
		if len(parts) < 3 {
			return 0, "", "", "", false
		}
		toolName := parts[2]
		tag := "all"
		if len(parts) > 3 {
			tag = strings.Join(parts[3:], "-")
		}
		return order, kind, toolName, tag, true

	default:
		return 0, "", "", "", false
	}
}

// filterDuplicates filters out fragments from "all" directory that have corresponding
// fragments in role-specific directory with the same order, kind, toolName, and tag.
func filterDuplicates(fragments []promptFragment) []promptFragment {
	// Build a set of role-specific fragments
	roleFragments := make(map[string]bool)
	for _, f := range fragments {
		if !f.isAll {
			key := fmt.Sprintf("%d-%s-%s-%s", f.order, f.kind, f.toolName, f.tag)
			roleFragments[key] = true
		}
	}

	// Filter out "all" fragments that have role-specific counterparts
	var result []promptFragment
	for _, f := range fragments {
		key := fmt.Sprintf("%d-%s-%s-%s", f.order, f.kind, f.toolName, f.tag)
		if f.isAll && roleFragments[key] {
			continue
		}
		result = append(result, f)
	}

	return result
}

// GetPrompt generates a prompt for the given tags, role and state.
// Takes map of fragments from scanners, concatenates and processes using text/template
// to create final prompt.
func (g *FSPromptGenerator) GetPrompt(tags []string, role *conf.AgentRoleConfig, state *AgentState) (string, error) {
	// Get fragments from all scanners
	fragments := make(map[string]string)
	// Merge fragments from all scanners in order
	for _, scanner := range g.scanners {
		scannerFragments, err := scanner.GetFragments(tags, role)
		if err != nil {
			return "", fmt.Errorf("GetPrompt() [prompt.go]: failed to get fragments: %w", err)
		}
		// Merge: later scanners override earlier ones
		for key, value := range scannerFragments {
			fragments[key] = value
		}
	}

	// Sort fragment keys by extracting the order number
	type keyOrder struct {
		key   string
		order int
	}

	keyOrders := make([]keyOrder, 0, len(fragments))
	for key := range fragments {
		// Extract order from key (e.g., "all/10-system" -> 10)
		parts := strings.Split(filepath.Base(key), "-")
		if len(parts) > 0 {
			if order, err := strconv.Atoi(parts[0]); err == nil {
				keyOrders = append(keyOrders, keyOrder{key: key, order: order})
				continue
			}
		}
		// Fallback: order = 0 if we can't parse
		keyOrders = append(keyOrders, keyOrder{key: key, order: 0})
	}

	// Sort by order, then by key alphabetically for stable sorting
	sort.Slice(keyOrders, func(i, j int) bool {
		if keyOrders[i].order != keyOrders[j].order {
			return keyOrders[i].order < keyOrders[j].order
		}
		return keyOrders[i].key < keyOrders[j].key
	})

	// Concatenate fragments
	var combined strings.Builder
	for i, ko := range keyOrders {
		if i > 0 {
			combined.WriteString("\n\n")
		}
		combined.WriteString(fragments[ko.key])
	}

	// Process template
	tmpl, err := template.New("prompt").Parse(combined.String())
	if err != nil {
		return "", fmt.Errorf("GetPrompt() [prompt_impl.go]: failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, state); err != nil {
		return "", fmt.Errorf("GetPrompt() [prompt_impl.go]: failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// GetPrompt generates a prompt for the given tags, role and state.
// It retrieves prompt fragments from the role's PromptFragments field in ConfigStore,
// applies the same filtering and merging logic as FSPromptGenerator.
func (g *ConfPromptGenerator) GetPrompt(tags []string, role *conf.AgentRoleConfig, state *AgentState) (string, error) {
	if role == nil {
		return "", fmt.Errorf("GetPrompt() [prompt.go]: role cannot be nil")
	}

	// Get all role configs from store to access both "all" and role-specific fragments
	roleConfigs, err := g.store.GetAgentRoleConfigs()
	if err != nil {
		return "", fmt.Errorf("GetPrompt() [prompt.go]: failed to get role configs: %w", err)
	}

	// Collect fragments from "all" role and the specific role
	var allFragments []promptFragment

	// Process "all" role first
	if allRole, ok := roleConfigs["all"]; ok && allRole.PromptFragments != nil {
		for filename, content := range allRole.PromptFragments {
			fragment := parseFragmentFromKey(filename, content, true)
			if fragment == nil {
				continue
			}

			// Check if tag matches
			if fragment.tag != "all" && !contains(tags, fragment.tag) {
				continue
			}

			// For tools fragments, check if tool is enabled
			if fragment.kind == "tools" {
				if role.ToolsAccess == nil {
					continue
				}
				access, ok := role.ToolsAccess[fragment.toolName]
				if !ok || access == "" {
					continue
				}
			}

			allFragments = append(allFragments, *fragment)
		}
	}

	// Process role-specific fragments
	if role.PromptFragments != nil {
		for filename, content := range role.PromptFragments {
			fragment := parseFragmentFromKey(filename, content, false)
			if fragment == nil {
				continue
			}

			// Check if tag matches
			if fragment.tag != "all" && !contains(tags, fragment.tag) {
				continue
			}

			// For tools fragments, check if tool is enabled
			if fragment.kind == "tools" {
				if role.ToolsAccess == nil {
					continue
				}
				access, ok := role.ToolsAccess[fragment.toolName]
				if !ok || access == "" {
					continue
				}
			}

			allFragments = append(allFragments, *fragment)
		}
	}

	// Filter out duplicates (role-specific overrides "all")
	allFragments = filterDuplicates(allFragments)

	// Sort by order
	sort.Slice(allFragments, func(i, j int) bool {
		return allFragments[i].order < allFragments[j].order
	})

	// Build result map (using same format as FSPromptGenerator)
	fragments := make(map[string]string)
	for _, f := range allFragments {
		// Key format: dir/file_prefix (without extension)
		dir := "all"
		if !f.isAll {
			dir = role.Name
		}

		// Build key
		var key string
		if f.kind == "system" {
			if f.tag == "all" {
				key = fmt.Sprintf("%s/%d-system", dir, f.order)
			} else {
				key = fmt.Sprintf("%s/%d-system-%s", dir, f.order, f.tag)
			}
		} else {
			if f.tag == "all" {
				key = fmt.Sprintf("%s/%d-tools-%s", dir, f.order, f.toolName)
			} else {
				key = fmt.Sprintf("%s/%d-tools-%s-%s", dir, f.order, f.toolName, f.tag)
			}
		}
		fragments[key] = f.content
	}

	// Sort fragment keys by extracting the order number (same as FSPromptGenerator)
	type keyOrder struct {
		key   string
		order int
	}

	keyOrders := make([]keyOrder, 0, len(fragments))
	for key := range fragments {
		// Extract order from key (e.g., "all/10-system" -> 10)
		parts := strings.Split(filepath.Base(key), "-")
		if len(parts) > 0 {
			if order, err := strconv.Atoi(parts[0]); err == nil {
				keyOrders = append(keyOrders, keyOrder{key: key, order: order})
				continue
			}
		}
		// Fallback: order = 0 if we can't parse
		keyOrders = append(keyOrders, keyOrder{key: key, order: 0})
	}

	// Sort by order, then by key alphabetically for stable sorting
	sort.Slice(keyOrders, func(i, j int) bool {
		if keyOrders[i].order != keyOrders[j].order {
			return keyOrders[i].order < keyOrders[j].order
		}
		return keyOrders[i].key < keyOrders[j].key
	})

	// Concatenate fragments
	var combined strings.Builder
	for i, ko := range keyOrders {
		if i > 0 {
			combined.WriteString("\n\n")
		}
		combined.WriteString(fragments[ko.key])
	}

	// Process template
	tmpl, err := template.New("prompt").Parse(combined.String())
	if err != nil {
		return "", fmt.Errorf("GetPrompt() [prompt.go]: failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, state); err != nil {
		return "", fmt.Errorf("GetPrompt() [prompt.go]: failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// parseFragmentFromKey parses a fragment key (filename without extension) and returns a promptFragment.
// Key format: "<num>-system" or "<num>-system-<tag>" or "<num>-tools-<toolname>" or "<num>-tools-<toolname>-<tag>"
// Returns nil if the key is invalid.
func parseFragmentFromKey(key string, content string, isAll bool) *promptFragment {
	// Parse filename part (same format as parseFilename but without .md extension)
	parts := strings.Split(key, "-")
	if len(parts) < 2 {
		return nil
	}

	// Parse order
	order, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil
	}

	kind := parts[1]

	switch kind {
	case "system":
		// Format: <num>-system-<tag> or <num>-system
		tag := "all"
		if len(parts) > 2 {
			tag = strings.Join(parts[2:], "-")
		}
		return &promptFragment{
			order:    order,
			kind:     kind,
			toolName: "",
			tag:      tag,
			filename: key,
			content:  content,
			isAll:    isAll,
		}

	case "tools":
		// Format: <num>-tools-<toolname>-<tag> or <num>-tools-<toolname>
		if len(parts) < 3 {
			return nil
		}
		toolName := parts[2]
		tag := "all"
		if len(parts) > 3 {
			tag = strings.Join(parts[3:], "-")
		}
		return &promptFragment{
			order:    order,
			kind:     kind,
			toolName: toolName,
			tag:      tag,
			filename: key,
			content:  content,
			isAll:    isAll,
		}

	default:
		return nil
	}
}

// contains checks if a string is in a slice.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
