package main

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	modelName      string
	configPath     string
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
			// Resolve working directory from argument or use current directory
			var dirPath string
			if len(args) > 0 {
				dirPath = args[0]
			}
			workDir, err := ResolveWorkDir(dirPath)
			if err != nil {
				return err
			}

			return runTUI(workDir, configPath, modelName, roleName, lspServer, saveSessionTo, saveSession, logLLMRequests)
		},
	}

	// Define flags
	rootCmd.Flags().StringVar(&modelName, "model", "", "Model name in provider/model format (if not set, uses default provider)")
	rootCmd.Flags().StringVar(&configPath, "config-path", "", "Colon-separated list of config directories (optional, added to default hierarchy)")
	rootCmd.Flags().StringVar(&roleName, "role", "developer", "Agent role name")
	rootCmd.Flags().StringVar(&lspServer, "lsp-server", "gopls", "Path to LSP server binary (empty to disable LSP)")
	rootCmd.Flags().StringVar(&saveSessionTo, "save-session-to", "", "Save session conversation to specified markdown file")
	rootCmd.Flags().BoolVar(&saveSession, "save-session", false, "Save session conversation")
	rootCmd.Flags().BoolVar(&logLLMRequests, "log-llm-requests", false, "Log LLM requests and responses")

	// Add subcommands
	rootCmd.AddCommand(ConfCommand())
	rootCmd.AddCommand(CliCommand())
	rootCmd.AddCommand(CleanCommand())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
