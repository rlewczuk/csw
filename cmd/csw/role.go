package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/spf13/cobra"
)

// RoleCommand creates the role command with all subcommands.
func RoleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "role",
		Short: "Manage agent roles",
		Long:  "List and manage agent role configurations",
	}

	// Add subcommands
	cmd.AddCommand(roleListCommand())
	cmd.AddCommand(roleShowCommand())
	cmd.AddCommand(roleSetDefaultCommand())
	cmd.AddCommand(roleGetDefaultCommand())

	return cmd
}

func roleListCommand() *cobra.Command {
	var useJSON bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all available roles from all configuration paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := GetCompositeConfigStore()
			if err != nil {
				return err
			}

			configs, err := store.GetAgentRoleConfigs()
			if err != nil {
				return fmt.Errorf("roleListCommand() [role.go]: failed to get role configs: %w", err)
			}

			if useJSON {
				return outputJSON(configs)
			}

			return outputRoleList(configs)
		},
	}

	cmd.Flags().BoolVar(&useJSON, "json", false, "Output in JSON format")

	return cmd
}

func roleShowCommand() *cobra.Command {
	var useJSON bool
	var showSystemPrompt bool
	var modelName string

	cmd := &cobra.Command{
		Use:   "show <role>",
		Short: "Show details of a role from composite configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			roleName := args[0]

			store, err := GetCompositeConfigStore()
			if err != nil {
				return err
			}

			configs, err := store.GetAgentRoleConfigs()
			if err != nil {
				return fmt.Errorf("roleShowCommand() [role.go]: failed to get role configs: %w", err)
			}

			config, exists := findRoleConfigByName(configs, roleName)
			if !exists {
				return fmt.Errorf("roleShowCommand() [role.go]: role not found: %s", roleName)
			}

			// If --system-prompt is specified, render and output the prompt
			if showSystemPrompt {
				return outputSystemPrompt(store, config, modelName, useJSON)
			}

			if useJSON {
				return outputJSON(config)
			}

			return outputRoleDetails(config)
		},
	}

	cmd.Flags().BoolVar(&useJSON, "json", false, "Output in JSON format")
	cmd.Flags().BoolVar(&showSystemPrompt, "system-prompt", false, "Render and output system prompt")
	cmd.Flags().StringVar(&modelName, "model", "", "Model name to use for prompt rendering (provider/model format; fallback lists not supported)")

	return cmd
}

func roleSetDefaultCommand() *cobra.Command {
	var useGlobal bool
	var useLocal bool
	var toPath string
	var scope ConfigScope = ConfigScopeLocal

	cmd := &cobra.Command{
		Use:   "set-default <role>",
		Short: "Set default role",
		Args:  cobra.ExactArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Handle --to, --global and --local flags
			if toPath != "" {
				if useGlobal || useLocal {
					return fmt.Errorf("cannot specify --to with --global or --local")
				}
				scope = ConfigScope(toPath)
				return nil
			}

			if useGlobal && useLocal {
				return fmt.Errorf("cannot specify both --global and --local")
			}
			if useGlobal {
				scope = ConfigScopeGlobal
			} else if useLocal {
				scope = ConfigScopeLocal
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			roleName := args[0]

			store, err := GetConfigStore(scope)
			if err != nil {
				return err
			}
			if closer, ok := store.(interface{ Close() error }); ok {
				defer closer.Close()
			}

			// Check if role exists in composite config
			compositeStore, err := GetCompositeConfigStore()
			if err != nil {
				return err
			}

			configs, err := compositeStore.GetAgentRoleConfigs()
			if err != nil {
				return fmt.Errorf("roleSetDefaultCommand() [role.go]: failed to get role configs: %w", err)
			}

			resolvedRole, exists := findRoleConfigByName(configs, roleName)
			if !exists {
				return fmt.Errorf("roleSetDefaultCommand() [role.go]: role not found: %s", roleName)
			}

			// Load global config
			globalConfig, err := store.GetGlobalConfig()
			if err != nil {
				return fmt.Errorf("roleSetDefaultCommand() [role.go]: failed to get global config: %w", err)
			}

			// Update default role
			globalConfig.DefaultRole = resolvedRole.Name

			// Save global config
			if err := store.SaveGlobalConfig(globalConfig); err != nil {
				return fmt.Errorf("roleSetDefaultCommand() [role.go]: failed to save global config: %w", err)
			}

			fmt.Printf("Default role set to '%s'\n", resolvedRole.Name)
			return nil
		},
	}

	cmd.Flags().BoolVar(&useGlobal, "global", false, "Use global configuration")
	cmd.Flags().BoolVar(&useLocal, "local", false, "Use local configuration (default)")
	cmd.Flags().StringVar(&toPath, "to", "", "Custom path to configuration directory")

	return cmd
}

func roleGetDefaultCommand() *cobra.Command {
	var useJSON bool

	cmd := &cobra.Command{
		Use:   "get-default",
		Short: "Get default role",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := GetCompositeConfigStore()
			if err != nil {
				return err
			}

			globalConfig, err := store.GetGlobalConfig()
			if err != nil {
				return fmt.Errorf("roleGetDefaultCommand() [role.go]: failed to get global config: %w", err)
			}

			if useJSON {
				result := map[string]string{"default_role": globalConfig.DefaultRole}
				return outputJSON(result)
			}

			if globalConfig.DefaultRole == "" {
				fmt.Println("No default role set")
			} else {
				fmt.Printf("Default role: %s\n", globalConfig.DefaultRole)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&useJSON, "json", false, "Output in JSON format")

	return cmd
}

// Helper functions

func outputRoleList(configs map[string]*conf.AgentRoleConfig) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "NAME\tDESCRIPTION")

	// Sort roles by name for consistent output
	names := make([]string, 0, len(configs))
	for name := range configs {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		config := configs[name]
		description := config.Description
		if description == "" {
			description = "-"
		}
		fmt.Fprintf(w, "%s\t%s\n", name, description)
	}

	return nil
}

func outputRoleDetails(config *conf.AgentRoleConfig) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// General information
	fmt.Fprintln(w, "=== General Information ===")
	fmt.Fprintln(w, "PROPERTY\tVALUE")
	fmt.Fprintf(w, "Name\t%s\n", config.Name)
	if len(config.Aliases) > 0 {
		fmt.Fprintf(w, "Aliases\t%s\n", strings.Join(config.Aliases, ", "))
	}
	if config.Description != "" {
		fmt.Fprintf(w, "Description\t%s\n", config.Description)
	}
	fmt.Fprintln(w)

	// VFS Privileges
	if len(config.VFSPrivileges) > 0 {
		fmt.Fprintln(w, "=== VFS Privileges ===")
		fmt.Fprintln(w, "PATH PATTERN\tREAD\tWRITE\tDELETE\tLIST\tFIND\tMOVE")

		// Sort paths for consistent output
		paths := make([]string, 0, len(config.VFSPrivileges))
		for path := range config.VFSPrivileges {
			paths = append(paths, path)
		}
		sort.Strings(paths)

		for _, path := range paths {
			access := config.VFSPrivileges[path]
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				path,
				formatAccessFlag(access.Read),
				formatAccessFlag(access.Write),
				formatAccessFlag(access.Delete),
				formatAccessFlag(access.List),
				formatAccessFlag(access.Find),
				formatAccessFlag(access.Move),
			)
		}
		fmt.Fprintln(w)
	}

	// Tool Access
	if len(config.ToolsAccess) > 0 {
		fmt.Fprintln(w, "=== Tool Access ===")
		fmt.Fprintln(w, "TOOL\tACCESS")

		// Sort tools for consistent output
		tools := make([]string, 0, len(config.ToolsAccess))
		for tool := range config.ToolsAccess {
			tools = append(tools, tool)
		}
		sort.Strings(tools)

		for _, tool := range tools {
			access := config.ToolsAccess[tool]
			fmt.Fprintf(w, "%s\t%s\n", tool, formatAccessFlag(access))
		}
		fmt.Fprintln(w)
	}

	// Run Privileges
	if len(config.RunPrivileges) > 0 {
		fmt.Fprintln(w, "=== Run Privileges ===")
		fmt.Fprintln(w, "COMMAND PATTERN\tACCESS")

		// Sort patterns for consistent output
		patterns := make([]string, 0, len(config.RunPrivileges))
		for pattern := range config.RunPrivileges {
			patterns = append(patterns, pattern)
		}
		sort.Strings(patterns)

		for _, pattern := range patterns {
			access := config.RunPrivileges[pattern]
			fmt.Fprintf(w, "%s\t%s\n", pattern, formatAccessFlag(access))
		}
		fmt.Fprintln(w)
	}

	return nil
}

func findRoleConfigByName(configs map[string]*conf.AgentRoleConfig, roleName string) (*conf.AgentRoleConfig, bool) {
	trimmedRoleName := strings.TrimSpace(roleName)
	if trimmedRoleName == "" {
		return nil, false
	}

	if config, exists := configs[trimmedRoleName]; exists && config != nil {
		return config, true
	}

	for _, config := range configs {
		if config == nil {
			continue
		}
		if strings.EqualFold(config.Name, trimmedRoleName) {
			return config, true
		}
		for _, alias := range config.Aliases {
			if strings.EqualFold(strings.TrimSpace(alias), trimmedRoleName) {
				return config, true
			}
		}
	}

	return nil, false
}

func formatAccessFlag(flag conf.AccessFlag) string {
	switch flag {
	case conf.AccessAllow:
		return "allow"
	case conf.AccessDeny:
		return "deny"
	case conf.AccessAsk:
		return "ask"
	default:
		return string(flag)
	}
}

func outputSystemPrompt(store conf.ConfigStore, roleConfig *conf.AgentRoleConfig, modelName string, useJSON bool) error {
	// Create prompt generator
	promptGenerator, err := core.NewConfPromptGenerator(store, nil)
	if err != nil {
		return fmt.Errorf("outputSystemPrompt() [role.go]: failed to create prompt generator: %w", err)
	}

	// Determine tags based on model name
	var tags []string
	if modelName != "" {
		// Parse model name (provider/model format)
		parts := strings.SplitN(modelName, "/", 2)
		if len(parts) != 2 {
			return fmt.Errorf("outputSystemPrompt() [role.go]: invalid model format: %s, expected single provider/model", modelName)
		}
		providerName := parts[0]
		actualModelName := parts[1]

		providerRegistry := models.NewProviderRegistry(store)
		modelTagRegistry, err := CreateModelTagRegistry(store, providerRegistry)
		if err != nil {
			return fmt.Errorf("outputSystemPrompt() [role.go]: failed to build model tag registry: %w", err)
		}
		tags = modelTagRegistry.GetTagsForModel(providerName, actualModelName)
	} else {
		// No model specified, use empty tags (will include "all" fragments)
		tags = []string{}
	}

	// Create empty agent state for prompt generation
	state := &core.AgentState{}

	// Generate prompt
	prompt, err := promptGenerator.GetPrompt(tags, roleConfig, state)
	if err != nil {
		return fmt.Errorf("outputSystemPrompt() [role.go]: failed to generate prompt: %w", err)
	}

	if useJSON {
		// Output as JSON string
		result := map[string]string{"system_prompt": prompt}
		return outputJSON(result)
	}

	// Output prompt directly to stdout
	fmt.Print(prompt)
	return nil
}
