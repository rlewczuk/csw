package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	modelName  string
	configPath string
	roleName   string
	lspServer  string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "csw [directory]",
		Short: "Codesnort SWE - AI-powered software engineering assistant",
		Long:  `Codesnort SWE is an AI-powered software engineering assistant that helps you write, review, and maintain code.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve working directory from argument or use current directory
			var workDir string
			if len(args) > 0 {
				// Directory provided as argument
				dirPath := args[0]
				absPath, err := filepath.Abs(dirPath)
				if err != nil {
					return fmt.Errorf("main() [main.go]: failed to resolve directory path: %w", err)
				}
				info, err := os.Stat(absPath)
				if err != nil {
					return fmt.Errorf("main() [main.go]: failed to access directory: %w", err)
				}
				if !info.IsDir() {
					return fmt.Errorf("main() [main.go]: path is not a directory: %s", dirPath)
				}
				workDir = absPath
			} else {
				// Use current directory
				wd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("main() [main.go]: failed to get current working directory: %w", err)
				}
				workDir = wd
			}

			return runTUI(workDir, configPath, modelName, roleName, lspServer)
		},
	}

	// Define flags
	rootCmd.Flags().StringVar(&modelName, "model", "", "Model name in provider/model format (if not set, uses default provider)")
	rootCmd.Flags().StringVar(&configPath, "config-path", "", "Colon-separated list of config directories (optional, added to default hierarchy)")
	rootCmd.Flags().StringVar(&roleName, "role", "developer", "Agent role name")
	rootCmd.Flags().StringVar(&lspServer, "lsp-server", "gopls", "Path to LSP server binary (empty to disable LSP)")

	// Add subcommands
	rootCmd.AddCommand(ConfCommand())
	rootCmd.AddCommand(CliCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
