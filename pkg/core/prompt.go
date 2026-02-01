package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/tool"
)

// PromptGenerator generates a prompt for the given tags, role and state.
type PromptGenerator interface {
	// GetPrompt generates a prompt for the given tags, role and state.
	// Takes map of fragments from GetFragments, concatenates and processes using text/template it to create final prompt;
	// Also responsible for eventual result caching if any of files has changed or agent state data has changed;
	GetPrompt(tags []string, role *conf.AgentRoleConfig, state *AgentState) (string, error)

	// GetToolInfo returns information about a tool including its description and parameter schema.
	// Returns error if tool description is not found.
	GetToolInfo(tags []string, toolName string, role *conf.AgentRoleConfig, state *AgentState) (tool.ToolInfo, error)
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

			// Skip tools fragments - they are now handled separately by GetToolInfo
			if fragment.kind == "tools" {
				continue
			}

			// Check if tag matches
			if fragment.tag != "all" && !contains(tags, fragment.tag) {
				continue
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

			// Skip tools fragments - they are now handled separately by GetToolInfo
			if fragment.kind == "tools" {
				continue
			}

			// Check if tag matches
			if fragment.tag != "all" && !contains(tags, fragment.tag) {
				continue
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

// GetToolInfo returns information about a tool including its description and parameter schema.
// It looks up tool descriptions from the role's ToolFragments with tag-specific overrides.
// Returns error if tool description is not found.
func (g *ConfPromptGenerator) GetToolInfo(tags []string, toolName string, role *conf.AgentRoleConfig, state *AgentState) (tool.ToolInfo, error) {
	if role == nil {
		return tool.ToolInfo{}, fmt.Errorf("GetToolInfo() [prompt.go]: role cannot be nil")
	}

	// Get all role configs from store to access both "all" and role-specific fragments
	roleConfigs, err := g.store.GetAgentRoleConfigs()
	if err != nil {
		return tool.ToolInfo{}, fmt.Errorf("GetToolInfo() [prompt.go]: failed to get role configs: %w", err)
	}

	// Collect tool fragments from "all" role and role-specific
	var toolFragments map[string]string

	// Check role-specific fragments first
	if role.ToolFragments != nil {
		toolFragments = role.ToolFragments
	} else if allRole, ok := roleConfigs["all"]; ok && allRole.ToolFragments != nil {
		toolFragments = allRole.ToolFragments
	}

	if toolFragments == nil {
		return tool.ToolInfo{}, fmt.Errorf("GetToolInfo() [prompt.go]: no tool fragments available")
	}

	// Look for <toolname>.schema.json
	schemaKey := fmt.Sprintf("%s/%s.schema.json", toolName, toolName)
	schemaContent, hasSchema := toolFragments[schemaKey]
	if !hasSchema {
		return tool.ToolInfo{}, fmt.Errorf("GetToolInfo() [prompt.go]: %s.schema.json not found for tool: %s", toolName, toolName)
	}

	// Parse the schema (description is now in the schema file)
	return parseToolDescription(toolName, schemaContent)
}

// parseToolDescription parses a tool description from JSON Schema file.
// schemaContent is the JSON Schema for tool parameters, including the description field.
func parseToolDescription(toolName string, schemaContent string) (tool.ToolInfo, error) {
	// Parse JSON Schema
	var schemaData map[string]any
	if err := json.Unmarshal([]byte(schemaContent), &schemaData); err != nil {
		return tool.ToolInfo{}, fmt.Errorf("parseToolDescription() [prompt.go]: failed to parse JSON schema for %s: %w", toolName, err)
	}

	// Extract description from schema
	description := ""
	if desc, ok := schemaData["description"].(string); ok {
		description = strings.TrimSpace(desc)
	}

	// Convert JSON schema to ToolSchema
	schema, err := convertJSONSchemaToToolSchema(schemaData)
	if err != nil {
		return tool.ToolInfo{}, fmt.Errorf("parseToolDescription() [prompt.go]: failed to convert schema for %s: %w", toolName, err)
	}

	return tool.ToolInfo{
		Name:        toolName,
		Description: description,
		Schema:      schema,
	}, nil
}

// convertJSONSchemaToToolSchema converts a JSON schema object to ToolSchema.
func convertJSONSchemaToToolSchema(schemaData map[string]any) (tool.ToolSchema, error) {
	schema := tool.NewToolSchema()

	// Get properties
	properties, ok := schemaData["properties"].(map[string]any)
	if !ok {
		return schema, nil
	}

	// Get required fields
	var requiredFields []string
	if required, ok := schemaData["required"].([]any); ok {
		for _, r := range required {
			if rStr, ok := r.(string); ok {
				requiredFields = append(requiredFields, rStr)
			}
		}
	}

	// Process each property
	for propName, propData := range properties {
		propMap, ok := propData.(map[string]any)
		if !ok {
			continue
		}

		propSchema, err := convertPropertySchema(propMap)
		if err != nil {
			return schema, fmt.Errorf("convertJSONSchemaToToolSchema() [prompt.go]: failed to convert property %s: %w", propName, err)
		}

		isRequired := false
		for _, reqField := range requiredFields {
			if reqField == propName {
				isRequired = true
				break
			}
		}

		schema.AddProperty(propName, propSchema, isRequired)
	}

	return schema, nil
}

// convertPropertySchema converts a JSON schema property to PropertySchema.
func convertPropertySchema(propData map[string]any) (tool.PropertySchema, error) {
	propSchema := tool.PropertySchema{}

	// Type
	if typeVal, ok := propData["type"].(string); ok {
		propSchema.Type = tool.SchemaType(typeVal)
	}

	// Description
	if desc, ok := propData["description"].(string); ok {
		propSchema.Description = desc
	}

	// Enum
	if enumVal, ok := propData["enum"].([]any); ok {
		for _, e := range enumVal {
			if eStr, ok := e.(string); ok {
				propSchema.Enum = append(propSchema.Enum, eStr)
			}
		}
	}

	// Items (for array type)
	if itemsVal, ok := propData["items"].(map[string]any); ok {
		itemsSchema, err := convertPropertySchema(itemsVal)
		if err != nil {
			return propSchema, fmt.Errorf("convertPropertySchema() [prompt.go]: failed to convert items: %w", err)
		}
		propSchema.Items = &itemsSchema
	}

	// Properties (for object type)
	if propsVal, ok := propData["properties"].(map[string]any); ok {
		propSchema.Properties = make(map[string]tool.PropertySchema)
		for nestedPropName, nestedPropData := range propsVal {
			nestedPropMap, ok := nestedPropData.(map[string]any)
			if !ok {
				continue
			}
			nestedSchema, err := convertPropertySchema(nestedPropMap)
			if err != nil {
				return propSchema, fmt.Errorf("convertPropertySchema() [prompt.go]: failed to convert nested property %s: %w", nestedPropName, err)
			}
			propSchema.Properties[nestedPropName] = nestedSchema
		}
	}

	// Required (for object type)
	if reqVal, ok := propData["required"].([]any); ok {
		for _, r := range reqVal {
			if rStr, ok := r.(string); ok {
				propSchema.Required = append(propSchema.Required, rStr)
			}
		}
	}

	return propSchema, nil
}
