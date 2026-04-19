package main

import (
	"context"
	"fmt"
	"os"

	"github.com/rlewczuk/csw/pkg/models"
	"github.com/spf13/cobra"
)

func providerTestCommand(scope *ConfigScope) *cobra.Command {
	var useStreaming bool
	var verbose bool

	cmd := &cobra.Command{
		Use:   "test <provider-name> <model-name>",
		Short: "Test a provider",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			providerName := args[0]
			modelName := args[1]

			store, err := GetCompositeConfigStore()
			if err != nil {
				return err
			}

			configs, err := store.GetModelProviderConfigs()
			if err != nil {
				return fmt.Errorf("providerTestCommand() [provider_test_cmd.go]: failed to get provider configs: %w", err)
			}

			config, exists := configs[providerName]
			if !exists {
				return fmt.Errorf("providerTestCommand() [provider_test_cmd.go]: provider not found: %s", providerName)
			}

			provider, err := models.ModelFromConfig(config)
			if err != nil {
				return fmt.Errorf("providerTestCommand() [provider_test_cmd.go]: failed to create provider: %w", err)
			}

			if updaterTarget, ok := provider.(interface{ SetConfigUpdater(models.ConfigUpdater) }); ok {
				store, closeFn, err := findWritableStoreForProvider(providerName)
				if err != nil {
					return err
				}
				if store != nil {
					if closeFn != nil {
						defer closeFn()
					}
					updater := models.NewConfigUpdater(providerName)
					updaterTarget.SetConfigUpdater(updater.Update())
				}
			}

			options := &models.ChatOptions{Verbose: verbose}
			chatModel := provider.ChatModel(modelName, options)

			fmt.Printf("Testing provider '%s' with model '%s'...\n\n", providerName, modelName)
			message := models.NewTextMessage(models.ChatRoleUser, "Please introduce yourself in one sentence.")

			if useStreaming {
				fmt.Print("Response: ")
				stream := chatModel.ChatStream(context.Background(), []*models.ChatMessage{message}, nil, nil)
				hasContent := false
				for fragment := range stream {
					fmt.Print(fragment.GetText())
					hasContent = true
				}
				fmt.Println()

				if !hasContent {
					fmt.Fprintf(os.Stderr, "\nNo response received from the model. This usually indicates an error occurred.\n")
					fmt.Fprintf(os.Stderr, "Common causes:\n")
					fmt.Fprintf(os.Stderr, "  - Invalid API key or authentication failure\n")
					fmt.Fprintf(os.Stderr, "  - Incorrect API endpoint URL\n")
					fmt.Fprintf(os.Stderr, "  - Model name not supported by the provider\n")
					fmt.Fprintf(os.Stderr, "  - Network connectivity issues\n")
					fmt.Fprintf(os.Stderr, "\nCheck the error messages above for more details.\n")
					return fmt.Errorf("providerTestCommand() [provider_test_cmd.go]: no response received from model")
				}
			} else {
				response, err := chatModel.Chat(context.Background(), []*models.ChatMessage{message}, nil, nil)
				if err != nil {
					return fmt.Errorf("providerTestCommand() [provider_test_cmd.go]: chat request failed: %w", err)
				}

				responseText := response.GetText()
				if responseText == "" {
					fmt.Fprintf(os.Stderr, "\nNo response received from the model. This usually indicates an error occurred.\n")
					fmt.Fprintf(os.Stderr, "Common causes:\n")
					fmt.Fprintf(os.Stderr, "  - Invalid API key or authentication failure\n")
					fmt.Fprintf(os.Stderr, "  - Incorrect API endpoint URL\n")
					fmt.Fprintf(os.Stderr, "  - Model name not supported by the provider\n")
					fmt.Fprintf(os.Stderr, "  - Network connectivity issues\n")
					fmt.Fprintf(os.Stderr, "\nCheck the error messages above for more details.\n")
					return fmt.Errorf("providerTestCommand() [provider_test_cmd.go]: no response received from model")
				}

				fmt.Printf("Response: %s\n", responseText)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&useStreaming, "streaming", false, "Use streaming mode (ChatStream)")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Print raw response and headers")

	return cmd
}
