package main

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/mcp"
	"github.com/spf13/cobra"
)

type mcpClient interface {
	Start() error
	Close() error
	ListTools() ([]mcp.RemoteTool, error)
}

var mcpNewClientFactory = func(name string, cfg *conf.MCPServerConfig) (mcpClient, error) {
	return mcp.NewClient(name, cfg)
}

// McpCommand creates diagnostics commands for configured MCP servers.
func McpCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server diagnostics",
		Long:  "List MCP servers and inspect tools exposed by MCP servers",
	}

	cmd.AddCommand(mcpListCommand())
	cmd.AddCommand(mcpToolCommand())

	return cmd
}

func mcpListCommand() *cobra.Command {
	var withStatus bool

	cmd := &cobra.Command{
		Use:   "list [--status]",
		Short: "List configured MCP servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := GetCompositeConfigStore()
			if err != nil {
				return err
			}

			configs, err := store.GetMCPServerConfigs()
			if err != nil {
				return fmt.Errorf("mcpListCommand() [mcp.go]: failed to load mcp server configs: %w", err)
			}

			return outputMCPServerList(cmd.OutOrStdout(), configs, withStatus)
		},
	}

	cmd.Flags().BoolVar(&withStatus, "status", false, "Probe server availability and include status column")

	return cmd
}

func mcpToolCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool",
		Short: "Inspect MCP server tools",
	}

	cmd.AddCommand(mcpToolListCommand())
	cmd.AddCommand(mcpToolInfoCommand())

	return cmd
}

func mcpToolListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list <server-name>",
		Short: "List tools available on MCP server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			serverName := strings.TrimSpace(args[0])
			store, err := GetCompositeConfigStore()
			if err != nil {
				return err
			}

			cfg, err := getMCPServerConfigByName(store, serverName)
			if err != nil {
				return err
			}

			remoteTools, err := fetchMCPServerTools(serverName, cfg)
			if err != nil {
				return err
			}

			matchers, err := compileMCPToolMatchers(cfg.Tools)
			if err != nil {
				return fmt.Errorf("mcpToolListCommand() [mcp.go]: invalid tool filters for %s: %w", serverName, err)
			}

			return outputMCPToolList(cmd.OutOrStdout(), remoteTools, cfg.Tools, matchers)
		},
	}
}

func mcpToolInfoCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "info <server-name> <tool-name>",
		Short: "Show detailed information about a tool on MCP server",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			serverName := strings.TrimSpace(args[0])
			toolName := strings.TrimSpace(args[1])

			store, err := GetCompositeConfigStore()
			if err != nil {
				return err
			}

			cfg, err := getMCPServerConfigByName(store, serverName)
			if err != nil {
				return err
			}

			remoteTools, err := fetchMCPServerTools(serverName, cfg)
			if err != nil {
				return err
			}

			remoteTool, found := findMCPRemoteToolByName(remoteTools, toolName)
			if !found {
				return fmt.Errorf("mcpToolInfoCommand() [mcp.go]: tool %q not found on server %q", toolName, serverName)
			}

			return outputMCPToolInfo(cmd.OutOrStdout(), serverName, remoteTool)
		},
	}
}

func outputMCPServerList(output io.Writer, configs map[string]*conf.MCPServerConfig, withStatus bool) error {
	w := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	defer w.Flush()

	if withStatus {
		fmt.Fprintln(w, "NAME\tDESCRIPTION\tTRANSPORT\tSTATUS")
	} else {
		fmt.Fprintln(w, "NAME\tDESCRIPTION\tTRANSPORT")
	}

	names := make([]string, 0, len(configs))
	for name := range configs {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		cfg := configs[name]
		if cfg == nil {
			cfg = &conf.MCPServerConfig{}
		}
		transport := resolveConfiguredMCPTransport(cfg)
		description := strings.TrimSpace(cfg.Description)

		if !withStatus {
			fmt.Fprintf(w, "%s\t%s\t%s\n", name, description, transport)
			continue
		}

		status := probeMCPServerStatus(name, cfg)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, description, transport, status)
	}

	return nil
}

func outputMCPToolList(output io.Writer, remoteTools []mcp.RemoteTool, filters []string, matchers []mcpToolMatcher) error {
	w := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "NAME\tDESCRIPTION\tAVAILABLE")

	sortedTools := append([]mcp.RemoteTool(nil), remoteTools...)
	sort.Slice(sortedTools, func(i, j int) bool {
		return sortedTools[i].Name < sortedTools[j].Name
	})

	for _, remoteTool := range sortedTools {
		available := "no"
		if isMCPToolAvailable(remoteTool.Name, filters, matchers) {
			available = "yes"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", remoteTool.Name, strings.TrimSpace(remoteTool.Description), available)
	}

	return nil
}

func outputMCPToolInfo(output io.Writer, serverName string, remoteTool mcp.RemoteTool) error {
	fmt.Fprintf(output, "Server: %s\n", serverName)
	fmt.Fprintf(output, "Tool: %s\n", remoteTool.Name)
	fmt.Fprintf(output, "Description: %s\n", strings.TrimSpace(remoteTool.Description))

	params := extractMCPToolParameters(remoteTool.InputSchema)
	fmt.Fprintln(output, "")
	fmt.Fprintln(output, "Parameters:")
	if len(params) == 0 {
		fmt.Fprintln(output, "- none")
	} else {
		for _, param := range params {
			required := "no"
			if param.Required {
				required = "yes"
			}
			fmt.Fprintf(output, "- %s (type: %s, required: %s): %s\n", param.Name, param.Type, required, param.Description)
		}
	}

	schemaJSON, err := json.MarshalIndent(remoteTool.InputSchema, "", "  ")
	if err != nil {
		return fmt.Errorf("outputMCPToolInfo() [mcp.go]: failed to marshal input schema for %s: %w", remoteTool.Name, err)
	}

	fmt.Fprintln(output, "")
	fmt.Fprintln(output, "Input Schema:")
	fmt.Fprintln(output, string(schemaJSON))

	return nil
}

func getMCPServerConfigByName(store conf.ConfigStore, serverName string) (*conf.MCPServerConfig, error) {
	configs, err := store.GetMCPServerConfigs()
	if err != nil {
		return nil, fmt.Errorf("getMCPServerConfigByName() [mcp.go]: failed to load mcp server configs: %w", err)
	}

	cfg, ok := configs[serverName]
	if !ok || cfg == nil {
		return nil, fmt.Errorf("getMCPServerConfigByName() [mcp.go]: mcp server not found: %s", serverName)
	}

	return cfg, nil
}

func fetchMCPServerTools(serverName string, cfg *conf.MCPServerConfig) ([]mcp.RemoteTool, error) {
	client, err := mcpNewClientFactory(serverName, cfg)
	if err != nil {
		return nil, fmt.Errorf("fetchMCPServerTools() [mcp.go]: failed to create mcp client for %s: %w", serverName, err)
	}

	if err := client.Start(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("fetchMCPServerTools() [mcp.go]: failed to start mcp client for %s: %w", serverName, err)
	}
	defer client.Close()

	tools, err := client.ListTools()
	if err != nil {
		return nil, fmt.Errorf("fetchMCPServerTools() [mcp.go]: failed to list tools for %s: %w", serverName, err)
	}

	return tools, nil
}

func probeMCPServerStatus(serverName string, cfg *conf.MCPServerConfig) string {
	if cfg == nil {
		return "unavailable"
	}
	if !cfg.Enabled {
		return "disabled"
	}

	client, err := mcpNewClientFactory(serverName, cfg)
	if err != nil {
		return "unavailable"
	}

	if err := client.Start(); err != nil {
		_ = client.Close()
		return "unavailable"
	}
	defer client.Close()

	if _, err := client.ListTools(); err != nil {
		return "unavailable"
	}

	return "available"
}

func resolveConfiguredMCPTransport(cfg *conf.MCPServerConfig) conf.MCPTransportType {
	if cfg == nil {
		return conf.MCPTransportTypeStdio
	}
	transport := conf.MCPTransportType(strings.ToLower(strings.TrimSpace(string(cfg.Transport))))
	switch transport {
	case conf.MCPTransportTypeHTTP, conf.MCPTransportTypeHTTPS, conf.MCPTransportTypeStdio:
		return transport
	default:
		return conf.MCPTransportTypeStdio
	}
}

type mcpToolMatcher struct {
	exact string
	glob  string
}

func compileMCPToolMatchers(patterns []string) ([]mcpToolMatcher, error) {
	if len(patterns) == 0 {
		return nil, nil
	}

	matchers := make([]mcpToolMatcher, 0, len(patterns))
	for _, pattern := range patterns {
		trimmed := strings.TrimSpace(pattern)
		if trimmed == "" {
			continue
		}
		if isMCPGlobPattern(trimmed) {
			if _, err := filepath.Match(trimmed, ""); err != nil {
				return nil, fmt.Errorf("compileMCPToolMatchers() [mcp.go]: invalid glob %q: %w", trimmed, err)
			}
			matchers = append(matchers, mcpToolMatcher{glob: trimmed})
			continue
		}
		matchers = append(matchers, mcpToolMatcher{exact: trimmed})
	}

	return matchers, nil
}

func isMCPToolAvailable(toolName string, patterns []string, matchers []mcpToolMatcher) bool {
	if len(patterns) == 0 {
		return true
	}

	for _, matcher := range matchers {
		if matcher.exact != "" && matcher.exact == toolName {
			return true
		}
		if matcher.glob == "" {
			continue
		}
		if ok, _ := filepath.Match(matcher.glob, toolName); ok {
			return true
		}
	}

	return false
}

func isMCPGlobPattern(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}

func findMCPRemoteToolByName(tools []mcp.RemoteTool, toolName string) (mcp.RemoteTool, bool) {
	for _, remoteTool := range tools {
		if remoteTool.Name == toolName {
			return remoteTool, true
		}
	}

	return mcp.RemoteTool{}, false
}

type mcpToolParameter struct {
	Name        string
	Type        string
	Description string
	Required    bool
}

func extractMCPToolParameters(schema map[string]any) []mcpToolParameter {
	propertiesRaw, ok := schema["properties"]
	if !ok {
		return nil
	}

	properties, ok := propertiesRaw.(map[string]any)
	if !ok {
		return nil
	}

	requiredSet := make(map[string]bool)
	for _, required := range parseRequiredParameters(schema["required"]) {
		requiredSet[required] = true
	}

	params := make([]mcpToolParameter, 0, len(properties))
	for name, propertyRaw := range properties {
		param := mcpToolParameter{
			Name:     name,
			Type:     "any",
			Required: requiredSet[name],
		}
		propertyMap, ok := propertyRaw.(map[string]any)
		if ok {
			param.Type = parseSchemaType(propertyMap["type"])
			if description, ok := propertyMap["description"].(string); ok {
				param.Description = strings.TrimSpace(description)
			}
		}
		params = append(params, param)
	}

	sort.Slice(params, func(i, j int) bool {
		return params[i].Name < params[j].Name
	})

	return params
}

func parseRequiredParameters(value any) []string {
	required := make([]string, 0)

	list, ok := value.([]any)
	if ok {
		for _, item := range list {
			if strValue, ok := item.(string); ok {
				required = append(required, strValue)
			}
		}
		return required
	}

	stringList, ok := value.([]string)
	if ok {
		required = append(required, stringList...)
	}

	return required
}

func parseSchemaType(value any) string {
	if typeValue, ok := value.(string); ok {
		trimmed := strings.TrimSpace(typeValue)
		if trimmed != "" {
			return trimmed
		}
	}

	typeList, ok := value.([]any)
	if ok {
		parts := make([]string, 0, len(typeList))
		for _, item := range typeList {
			if strValue, ok := item.(string); ok {
				trimmed := strings.TrimSpace(strValue)
				if trimmed != "" {
					parts = append(parts, trimmed)
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "|")
		}
	}

	return "any"
}
