package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// toolListEntry represents a tool entry for listing
type toolListEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ToolCommand creates the tool command with all subcommands.
func ToolCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool",
		Short: "Manage tool configurations",
		Long:  "List and view tool configurations and descriptions",
	}

	// Add subcommands
	cmd.AddCommand(toolListCommand())
	cmd.AddCommand(toolInfoCommand())
	cmd.AddCommand(toolDescCommand())

	return cmd
}

func toolListCommand() *cobra.Command {
	var useJSON bool
	var roleName string

	cmd := &cobra.Command{
		Use:   "list [--role <role>] [--json]",
		Short: "List all available tools",
		Long:  "Lists all tools available for given role (or all tools if role is not specified)",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := GetCompositeConfigStore()
			if err != nil {
				return err
			}

			// Get role configs to access tool fragments
			roleConfigs, err := store.GetAgentRoleConfigs()
			if err != nil {
				return fmt.Errorf("toolListCommand() [tool.go]: failed to get role configs: %w", err)
			}

			var toolFragments map[string]string
			var roleConfig *conf.AgentRoleConfig

			if roleName != "" {
				// Get specific role's tool fragments
				rc, exists := findRoleConfigByName(roleConfigs, roleName)
				if !exists || rc == nil {
					return fmt.Errorf("toolListCommand() [tool.go]: role not found: %s", roleName)
				}
				roleConfig = rc
				toolFragments = rc.ToolFragments

				// If role doesn't have tool fragments, fall back to "all" role
				if toolFragments == nil {
					allRole, exists := roleConfigs["all"]
					if exists && allRole != nil {
						toolFragments = allRole.ToolFragments
					}
				}
			} else {
				// Get all tools from "all" role
				allRole, exists := roleConfigs["all"]
				if exists && allRole != nil {
					toolFragments = allRole.ToolFragments
				}
			}

			if toolFragments == nil {
				return fmt.Errorf("toolListCommand() [tool.go]: no tool fragments available")
			}

			// Extract tool names and descriptions from tool fragments
			tools := make(map[string]string)
			for key := range toolFragments {
				parts := strings.Split(key, "/")
				if len(parts) != 2 {
					continue
				}
				toolName := parts[0]
				fileName := parts[1]
				if toolName == "" || fileName == "" || strings.HasPrefix(fileName, ".") {
					continue
				}

				if !(fileName == toolName+".schema.json" || fileName == toolName+".md" || fileName == toolName+".json" || fileName == toolName+".yaml" || fileName == toolName+".yml") {
					continue
				}

				if roleConfig != nil {
					access, hasAccess := roleConfig.ToolsAccess[toolName]
					if !hasAccess {
						access, hasAccess = roleConfig.ToolsAccess["**"]
					}
					if !hasAccess || access == conf.AccessDeny {
						continue
					}
				}

				descKey := toolName + "/" + toolName + ".md"
				if desc, ok := toolFragments[descKey]; ok {
					toolInfo := tool.ToolInfo{Description: desc}
					tools[toolName] = toolInfo.ShortDescription()
				} else if _, exists := tools[toolName]; !exists {
					tools[toolName] = ""
				}
			}

			if useJSON {
				return outputToolListJSON(tools)
			}

			return outputToolListTable(tools)
		},
	}

	cmd.Flags().BoolVar(&useJSON, "json", false, "Output in JSON format")
	cmd.Flags().StringVar(&roleName, "role", "", "Filter tools by role")

	return cmd
}

func toolInfoCommand() *cobra.Command {
	var useJSON bool

	cmd := &cobra.Command{
		Use:   "info <tool-name> [--json]",
		Short: "Show detailed information about a tool",
		Long:  "Prints information about given tool (the same that is given to LLM, i.e. schema and description)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			toolName := args[0]

			store, err := GetCompositeConfigStore()
			if err != nil {
				return err
			}

			// Create prompt generator to access GetToolInfo
			promptGenerator, err := core.NewConfPromptGenerator(store, nil)
			if err != nil {
				return fmt.Errorf("toolInfoCommand() [tool.go]: failed to create prompt generator: %w", err)
			}

			// Get role configs
			roleConfigs, err := store.GetAgentRoleConfigs()
			if err != nil {
				return fmt.Errorf("toolInfoCommand() [tool.go]: failed to get role configs: %w", err)
			}

			// Use "all" role to get tool info
			allRole, exists := roleConfigs["all"]
			if !exists || allRole == nil {
				return fmt.Errorf("toolInfoCommand() [tool.go]: 'all' role not found")
			}

			// Get tool info with empty tags (will use default <toolname>.md)
			toolInfo, err := promptGenerator.GetToolInfo([]string{}, toolName, allRole, &core.AgentState{})
			if err != nil {
				return fmt.Errorf("toolInfoCommand() [tool.go]: failed to get tool info: %w", err)
			}

			if useJSON {
				return outputJSON(toolInfo)
			}

			return outputToolInfo(toolInfo)
		},
	}

	cmd.Flags().BoolVar(&useJSON, "json", false, "Output in JSON format")

	return cmd
}

func toolDescCommand() *cobra.Command {
	var useJSON bool

	cmd := &cobra.Command{
		Use:   "desc <tool-name> [--json]",
		Short: "Show tool description",
		Long:  "Prints tool description in same format as 'conf tool info' but without schema",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			toolName := args[0]

			store, err := GetCompositeConfigStore()
			if err != nil {
				return err
			}

			// Create prompt generator to access GetToolInfo
			promptGenerator, err := core.NewConfPromptGenerator(store, nil)
			if err != nil {
				return fmt.Errorf("toolDescCommand() [tool.go]: failed to create prompt generator: %w", err)
			}

			// Get role configs
			roleConfigs, err := store.GetAgentRoleConfigs()
			if err != nil {
				return fmt.Errorf("toolDescCommand() [tool.go]: failed to get role configs: %w", err)
			}

			// Use "all" role to get tool info
			allRole, exists := roleConfigs["all"]
			if !exists || allRole == nil {
				return fmt.Errorf("toolDescCommand() [tool.go]: 'all' role not found")
			}

			// Get tool info with empty tags (will use default <toolname>.md)
			toolInfo, err := promptGenerator.GetToolInfo([]string{}, toolName, allRole, &core.AgentState{})
			if err != nil {
				return fmt.Errorf("toolDescCommand() [tool.go]: failed to get tool info: %w", err)
			}

			if useJSON {
				result := map[string]string{"description": toolInfo.Description}
				return outputJSON(result)
			}

			// Output description as is (markdown)
			fmt.Print(toolInfo.Description)
			return nil
		},
	}

	cmd.Flags().BoolVar(&useJSON, "json", false, "Output in JSON format")

	return cmd
}

// Helper functions

func outputToolListTable(tools map[string]string) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "NAME\tDESCRIPTION")

	// Sort tool names for consistent output
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		description := tools[name]
		if description == "" {
			description = "-"
		}
		// Truncate long descriptions for table display
		if len(description) > 80 {
			description = description[:77] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\n", name, description)
	}

	return nil
}

func outputToolListJSON(tools map[string]string) error {
	// Sort tool names for consistent output
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	sort.Strings(names)

	// Create array of tool entries
	entries := make([]toolListEntry, 0, len(tools))
	for _, name := range names {
		entries = append(entries, toolListEntry{
			Name:        name,
			Description: tools[name],
		})
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(entries)
}

func outputToolInfo(toolInfo tool.ToolInfo) error {
	// Output in YAML-like format with description at the end
	fmt.Printf("name: %s\n", toolInfo.Name)
	fmt.Printf("schema:\n")

	// Convert schema to YAML with proper indentation
	schemaYAML, err := yaml.Marshal(toolInfo.Schema)
	if err != nil {
		return fmt.Errorf("outputToolInfo() [tool.go]: failed to marshal schema: %w", err)
	}

	// Indent schema YAML by 2 spaces
	schemaLines := strings.Split(string(schemaYAML), "\n")
	for _, line := range schemaLines {
		if line != "" {
			fmt.Printf("  %s\n", line)
		}
	}

	fmt.Printf("description: |\n")
	// Indent description by 2 spaces
	descLines := strings.Split(toolInfo.Description, "\n")
	for _, line := range descLines {
		fmt.Printf("  %s\n", line)
	}

	return nil
}
