package main

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	modelName      string
	configPath     string
	projectConfig  string
	roleName       string
	lspServer      string
	saveSessionTo  string
	saveSession    bool
	logLLMRequests bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "csw [directory]",
		Short: "Codesnort SWE - AI-powered software engineering assistant",
		Long:  `Codesnort SWE is an AI-powered software engineering assistant that helps you write, review, and maintain code.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Define flags
	rootCmd.Flags().StringVar(&modelName, "model", "", "Model alias or model spec in provider/model format (single or comma-separated fallback list); if not set, uses defaults")
	rootCmd.Flags().StringVar(&configPath, "config-path", "", "Colon-separated list of config directories (optional, added to default hierarchy)")
	rootCmd.Flags().StringVar(&projectConfig, "project-config", "", "Custom project config directory (default: .csw/config)")
	rootCmd.Flags().StringVar(&roleName, "role", "developer", "Agent role name")
	rootCmd.Flags().StringVar(&lspServer, "lsp-server", "gopls", "Path to LSP server binary (empty to disable LSP)")
	rootCmd.Flags().StringVar(&saveSessionTo, "save-session-to", "", "Save session conversation to specified markdown file")
	rootCmd.Flags().BoolVar(&saveSession, "save-session", false, "Save session conversation")
	rootCmd.Flags().BoolVar(&logLLMRequests, "log-llm-requests", false, "Log LLM requests and responses")

	// Add subcommands
	rootCmd.AddCommand(ProviderCommand())
	rootCmd.AddCommand(RoleCommand())
	rootCmd.AddCommand(ToolCommand())
	rootCmd.AddCommand(CliCommand())
	rootCmd.AddCommand(CleanCommand())
	rootCmd.AddCommand(McpCommand())
	rootCmd.AddCommand(HookCommand())
	rootCmd.AddCommand(TaskCommand())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
