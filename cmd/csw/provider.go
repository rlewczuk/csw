package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/spf13/cobra"
)

// modelEntry represents a model from a specific provider
type modelEntry struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

var (
	providerAuthPort         = 1455
	providerAuthCallbackPath = "/auth/callback"
	providerAuthTimeout      = 5 * time.Minute
	providerAuthExtraParams  = map[string]string{
		"id_token_add_organizations": "true",
		"codex_cli_simplified_flow":  "true",
		"originator":                 "opencode",
	}
)

// ProviderCommand creates the provider command with all subcommands.
func ProviderCommand() *cobra.Command {
	var useJSON bool
	var useGlobal bool
	var useLocal bool
	var toPath string
	var scope ConfigScope = ConfigScopeLocal

	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Manage model providers",
		Long:  "List and manage model provider configurations",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Handle --to, --global and --local flags
			// --to has priority over --global and --local
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
	}

	// Add global flags
	cmd.PersistentFlags().BoolVar(&useJSON, "json", false, "Use JSON format for input/output")
	cmd.PersistentFlags().BoolVar(&useGlobal, "global", false, "Use global configuration")
	cmd.PersistentFlags().BoolVar(&useLocal, "local", false, "Use local configuration (default)")
	cmd.PersistentFlags().StringVar(&toPath, "to", "", "Custom path to configuration directory")

	// Add subcommands
	cmd.AddCommand(providerListCommand(&useJSON))
	cmd.AddCommand(providerShowCommand(&useJSON))
	cmd.AddCommand(providerSetDefaultCommand(&scope))
	cmd.AddCommand(providerTestCommand(&scope))
	cmd.AddCommand(providerAuthCommand())
	cmd.AddCommand(providerModelsCommand(&useJSON))

	return cmd
}

func providerListCommand(useJSON *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all providers from all configuration paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := GetCompositeConfigStore()
			if err != nil {
				return err
			}

			configs, err := store.GetModelProviderConfigs()
			if err != nil {
				return fmt.Errorf("providerListCommand() [provider.go]: failed to get provider configs: %w", err)
			}

			if *useJSON {
				return outputJSON(configs)
			}

			return outputProviderList(configs)
		},
	}
}

func providerShowCommand(useJSON *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "show <provider-name>",
		Short: "Show details of a provider from composite configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			providerName := args[0]

			store, err := GetCompositeConfigStore()
			if err != nil {
				return err
			}

			configs, err := store.GetModelProviderConfigs()
			if err != nil {
				return fmt.Errorf("providerShowCommand() [provider.go]: failed to get provider configs: %w", err)
			}

			config, exists := configs[providerName]
			if !exists {
				return fmt.Errorf("providerShowCommand() [provider.go]: provider not found: %s", providerName)
			}

			if *useJSON {
				return outputJSON(config)
			}

			return outputProviderDetails(config)
		},
	}
}

func providerSetDefaultCommand(scope *ConfigScope) *cobra.Command {
	return &cobra.Command{
		Use:   "set-default <provider-name>",
		Short: "Set default provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			providerName := args[0]

			store, err := GetConfigStore(*scope)
			if err != nil {
				return err
			}
			if closer, ok := store.(interface{ Close() error }); ok {
				defer closer.Close()
			}

			// Check if provider exists
			configs, err := store.GetModelProviderConfigs()
			if err != nil {
				return fmt.Errorf("providerSetDefaultCommand() [provider.go]: failed to get provider configs: %w", err)
			}

			if _, exists := configs[providerName]; !exists {
				return fmt.Errorf("providerSetDefaultCommand() [provider.go]: provider not found: %s", providerName)
			}

			// Load global config
			globalConfig, err := store.GetGlobalConfig()
			if err != nil {
				return fmt.Errorf("providerSetDefaultCommand() [provider.go]: failed to get global config: %w", err)
			}

			// Update default provider
			globalConfig.Defaults.DefaultProvider = providerName

			// Save global config
			if err := store.SaveGlobalConfig(globalConfig); err != nil {
				return fmt.Errorf("providerSetDefaultCommand() [provider.go]: failed to save global config: %w", err)
			}

			fmt.Printf("Default provider set to '%s'\n", providerName)
			return nil
		},
	}
}

func providerTestCommand(scope *ConfigScope) *cobra.Command {
	var useStreaming bool
	var verbose bool

	cmd := &cobra.Command{
		Use:   "test <provider-name> <model-name>",
		Short: "Test a provider",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Silence usage message on errors
			cmd.SilenceUsage = true

			providerName := args[0]
			modelName := args[1]

			// Use composite config to get provider configuration from all sources
			store, err := GetCompositeConfigStore()
			if err != nil {
				return err
			}

			configs, err := store.GetModelProviderConfigs()
			if err != nil {
				return fmt.Errorf("providerTestCommand() [provider.go]: failed to get provider configs: %w", err)
			}

			config, exists := configs[providerName]
			if !exists {
				return fmt.Errorf("providerTestCommand() [provider.go]: provider not found: %s", providerName)
			}

			// Create provider
			provider, err := models.ModelFromConfig(config)
			if err != nil {
				return fmt.Errorf("providerTestCommand() [provider.go]: failed to create provider: %w", err)
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
					updater := models.NewConfigUpdater(store, providerName)
					updaterTarget.SetConfigUpdater(updater.Update())
				}
			}

			// Create chat options with verbose flag
			options := &models.ChatOptions{
				Verbose: verbose,
			}

			// Create chat model
			chatModel := provider.ChatModel(modelName, options)

			// Send test message
			fmt.Printf("Testing provider '%s' with model '%s'...\n\n", providerName, modelName)

			message := models.NewTextMessage(models.ChatRoleUser, "Please introduce yourself in one sentence.")

			if useStreaming {
				// Use ChatStream for streaming response
				fmt.Print("Response: ")
				stream := chatModel.ChatStream(context.Background(), []*models.ChatMessage{message}, nil, nil)
				hasContent := false
				for fragment := range stream {
					fmt.Print(fragment.GetText())
					hasContent = true
				}
				fmt.Println()

				// If no content was received, it likely means there was an error
				if !hasContent {
					fmt.Fprintf(os.Stderr, "\nNo response received from the model. This usually indicates an error occurred.\n")
					fmt.Fprintf(os.Stderr, "Common causes:\n")
					fmt.Fprintf(os.Stderr, "  - Invalid API key or authentication failure\n")
					fmt.Fprintf(os.Stderr, "  - Incorrect API endpoint URL\n")
					fmt.Fprintf(os.Stderr, "  - Model name not supported by the provider\n")
					fmt.Fprintf(os.Stderr, "  - Network connectivity issues\n")
					fmt.Fprintf(os.Stderr, "\nCheck the error messages above for more details.\n")
					return fmt.Errorf("providerTestCommand() [provider.go]: no response received from model")
				}
			} else {
				// Use Chat for non-streaming response
				response, err := chatModel.Chat(context.Background(), []*models.ChatMessage{message}, nil, nil)
				if err != nil {
					return fmt.Errorf("providerTestCommand() [provider.go]: chat request failed: %w", err)
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
					return fmt.Errorf("providerTestCommand() [provider.go]: no response received from model")
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

func providerAuthCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "auth <provider-name>",
		Short: "Authenticate a provider via browser OAuth2 flow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			providerName := args[0]

			store, closeFn, err := findWritableStoreForProvider(providerName)
			if err != nil {
				return err
			}
			if store == nil {
				return fmt.Errorf("providerAuthCommand() [provider.go]: provider not found: %s", providerName)
			}
			if closeFn != nil {
				defer closeFn()
			}

			configs, err := store.GetModelProviderConfigs()
			if err != nil {
				return fmt.Errorf("providerAuthCommand() [provider.go]: failed to get provider configs: %w", err)
			}

			config, exists := configs[providerName]
			if !exists {
				return fmt.Errorf("providerAuthCommand() [provider.go]: provider not found: %s", providerName)
			}

			config.APIKey = ""
			config.RefreshToken = ""
			if err := store.SaveModelProviderConfig(config); err != nil {
				return fmt.Errorf("providerAuthCommand() [provider.go]: failed to clear previous provider auth data: %w", err)
			}

			pkce, err := models.GenerateOAuthPKCECodes()
			if err != nil {
				return fmt.Errorf("providerAuthCommand() [provider.go]: failed to generate PKCE codes: %w", err)
			}

			state, err := models.GenerateOAuthState()
			if err != nil {
				return fmt.Errorf("providerAuthCommand() [provider.go]: failed to generate OAuth state: %w", err)
			}

			redirectURI := providerAuthRedirectURI()
			authURL, err := models.BuildAuthorizationURL(
				config,
				redirectURI,
				state,
				pkce.Challenge,
				models.DefaultOAuthScope,
				providerAuthExtraParams,
			)
			if err != nil {
				return fmt.Errorf("providerAuthCommand() [provider.go]: failed to build authorization URL: %w", err)
			}

			fmt.Printf("Open this link in your browser to authenticate provider '%s':\n%s\n", providerName, authURL)
			fmt.Printf("Waiting for callback on %s ...\n", redirectURI)

			ctx, cancel := context.WithTimeout(context.Background(), providerAuthTimeout)
			defer cancel()

			callback, err := models.WaitForOAuthCallback(ctx, providerAuthListenAddress(), providerAuthCallbackPath)
			if err != nil {
				return fmt.Errorf("providerAuthCommand() [provider.go]: failed waiting for OAuth callback: %w", err)
			}

			if callback.State != state {
				return fmt.Errorf("providerAuthCommand() [provider.go]: invalid state returned by callback")
			}

			httpClient := &http.Client{Timeout: 30 * time.Second}
			if config.RequestTimeout > 0 {
				httpClient.Timeout = config.RequestTimeout
			}

			tokenResp, err := models.ExchangeAuthorizationCode(config, httpClient, callback.Code, redirectURI, pkce.Verifier)
			if err != nil {
				return fmt.Errorf("providerAuthCommand() [provider.go]: failed to exchange authorization code: %w", err)
			}

			config.AuthMode = conf.AuthModeOAuth2
			config.APIKey = tokenResp.AccessToken
			if tokenResp.RefreshToken != "" {
				config.RefreshToken = tokenResp.RefreshToken
			}

			if err := store.SaveModelProviderConfig(config); err != nil {
				return fmt.Errorf("providerAuthCommand() [provider.go]: failed to save provider config: %w", err)
			}

			fmt.Printf("Provider '%s' authenticated successfully\n", providerName)
			return nil
		},
	}
}

func providerModelsCommand(useJSON *bool) *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:   "models [<provider>]",
		Short: "List models for a provider or all providers",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := GetCompositeConfigStore()
			if err != nil {
				return err
			}

			configs, err := store.GetModelProviderConfigs()
			if err != nil {
				return fmt.Errorf("providerModelsCommand() [provider.go]: failed to get provider configs: %w", err)
			}

			// Check if specific provider is requested
			var targetProviders []string
			if len(args) == 1 {
				providerName := args[0]
				if _, exists := configs[providerName]; !exists {
					fmt.Fprintf(os.Stderr, "Error: provider not found: %s\n", providerName)
					os.Exit(1)
				}
				targetProviders = []string{providerName}
			} else {
				// List all providers
				for name := range configs {
					targetProviders = append(targetProviders, name)
				}
			}

			// Collect models from target providers
			var modelsList []modelEntry

			for _, providerName := range targetProviders {
				config := configs[providerName]
				provider, err := models.ModelFromConfig(config)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to create provider %s: %v\n", providerName, err)
					continue
				}

				// Enable verbose logging if requested
				provider.SetVerbose(verbose)

				modelInfos, err := provider.ListModels()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to list models for provider %s: %v\n", providerName, err)
					continue
				}

				for _, modelInfo := range modelInfos {
					modelsList = append(modelsList, modelEntry{
						Provider: providerName,
						Model:    modelInfo.Name,
					})
				}
			}

			// Output results
			if *useJSON {
				return outputJSON(modelsList)
			}

			return outputModelsList(modelsList)
		},
	}

	cmd.Flags().BoolVar(&verbose, "verbose", false, "Print raw request and response headers")

	return cmd
}

// Helper functions

func outputProviderList(configs map[string]*conf.ModelProviderConfig) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "NAME\tTYPE\tDESCRIPTION")
	for name, config := range configs {
		description := config.Description
		if description == "" {
			description = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", name, config.Type, description)
	}

	return nil
}

func outputProviderDetails(config *conf.ModelProviderConfig) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "PROPERTY\tVALUE")
	fmt.Fprintf(w, "Name\t%s\n", config.Name)
	fmt.Fprintf(w, "Type\t%s\n", config.Type)
	if config.Description != "" {
		fmt.Fprintf(w, "Description\t%s\n", config.Description)
	}
	fmt.Fprintf(w, "URL\t%s\n", config.URL)
	if config.AuthURL != "" {
		fmt.Fprintf(w, "Auth URL\t%s\n", config.AuthURL)
	}
	if config.APIKey != "" {
		fmt.Fprintf(w, "API Key\t%s\n", maskAPIKey(config.APIKey))
	}
	if config.ConnectTimeout > 0 {
		fmt.Fprintf(w, "Connect Timeout\t%s\n", config.ConnectTimeout)
	}
	if config.RequestTimeout > 0 {
		fmt.Fprintf(w, "Request Timeout\t%s\n", config.RequestTimeout)
	}
	if len(config.Headers) > 0 {
		for name, value := range config.Headers {
			fmt.Fprintf(w, "Header %s\t%s\n", name, maskHeaderValue(name, value))
		}
	}

	return nil
}

func outputJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func promptProviderConfig(name, providerType, url, description, apiKey string) (*conf.ModelProviderConfig, error) {
	reader := bufio.NewReader(os.Stdin)

	config := &conf.ModelProviderConfig{
		Name: name,
	}

	// Prompt for type if not provided
	if providerType == "" {
		fmt.Print("Provider type (openai/ollama/anthropic/responses/jetbrains) [openai]: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("promptProviderConfig() [provider.go]: failed to read type: %w", err)
		}
		input = strings.TrimSpace(input)
		if input == "" {
			config.Type = "openai"
		} else {
			config.Type = input
		}
	} else {
		config.Type = providerType
	}

	// Prompt for URL if not provided
	if url == "" {
		var defaultURL string
		switch config.Type {
		case "openai":
			defaultURL = "https://api.openai.com/v1"
		case "ollama":
			defaultURL = "http://localhost:11434"
		case "anthropic":
			defaultURL = "https://api.anthropic.com/v1"
		case "responses":
			defaultURL = "https://api.openai.com/v1"
		case "jetbrains":
			defaultURL = "https://api.jetbrains.ai"
		}

		fmt.Printf("Provider URL [%s]: ", defaultURL)
		input, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("promptProviderConfig() [provider.go]: failed to read URL: %w", err)
		}
		input = strings.TrimSpace(input)
		if input == "" {
			config.URL = defaultURL
		} else {
			config.URL = input
		}
	} else {
		config.URL = url
	}

	// Prompt for description if not provided
	if description == "" {
		fmt.Print("Description (optional): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("promptProviderConfig() [provider.go]: failed to read description: %w", err)
		}
		config.Description = strings.TrimSpace(input)
	} else {
		config.Description = description
	}

	// Prompt for API key if not provided
	if apiKey == "" {
		fmt.Print("API key (optional): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("promptProviderConfig() [provider.go]: failed to read API key: %w", err)
		}
		config.APIKey = strings.TrimSpace(input)
	} else {
		config.APIKey = apiKey
	}

	return config, nil
}

func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "********"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

func maskHeaderValue(name, value string) string {
	if value == "" {
		return value
	}
	nameLower := strings.ToLower(name)
	if strings.Contains(nameLower, "authorization") || strings.Contains(nameLower, "api-key") {
		return maskAPIKey(value)
	}
	return value
}

func parseHeadersFlag(headers []string) (map[string]string, error) {
	if len(headers) == 0 {
		return nil, nil
	}

	parsed := make(map[string]string, len(headers))
	for _, header := range headers {
		parts := strings.SplitN(header, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("parseHeadersFlag() [provider.go]: invalid header %q, expected key=value", header)
		}
		name := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if name == "" || value == "" {
			return nil, fmt.Errorf("parseHeadersFlag() [provider.go]: invalid header %q, key and value required", header)
		}
		parsed[name] = value
	}

	return parsed, nil
}

func outputModelsList(modelsList []modelEntry) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "PROVIDER\tMODEL")
	for _, entry := range modelsList {
		fmt.Fprintf(w, "%s\t%s\n", entry.Provider, entry.Model)
	}

	return nil
}

func providerAuthListenAddress() string {
	return fmt.Sprintf("127.0.0.1:%d", providerAuthPort)
}

func providerAuthRedirectURI() string {
	return fmt.Sprintf("http://localhost:%d%s", providerAuthPort, providerAuthCallbackPath)
}

func findWritableStoreForProvider(providerName string) (conf.WritableConfigStore, func() error, error) {
	localStore, err := GetConfigStore(ConfigScopeLocal)
	if err != nil {
		return nil, nil, fmt.Errorf("findWritableStoreForProvider() [provider.go]: failed to open local config store: %w", err)
	}
	localConfigs, err := localStore.GetModelProviderConfigs()
	if err != nil {
		if closer, ok := localStore.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		return nil, nil, fmt.Errorf("findWritableStoreForProvider() [provider.go]: failed to load local provider configs: %w", err)
	}
	if _, exists := localConfigs[providerName]; exists {
		return localStore, func() error {
			if closer, ok := localStore.(interface{ Close() error }); ok {
				return closer.Close()
			}
			return nil
		}, nil
	}
	if closer, ok := localStore.(interface{ Close() error }); ok {
		_ = closer.Close()
	}

	globalStore, err := GetConfigStore(ConfigScopeGlobal)
	if err != nil {
		return nil, nil, fmt.Errorf("findWritableStoreForProvider() [provider.go]: failed to open global config store: %w", err)
	}
	globalConfigs, err := globalStore.GetModelProviderConfigs()
	if err != nil {
		if closer, ok := globalStore.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		return nil, nil, fmt.Errorf("findWritableStoreForProvider() [provider.go]: failed to load global provider configs: %w", err)
	}
	if _, exists := globalConfigs[providerName]; exists {
		return globalStore, func() error {
			if closer, ok := globalStore.(interface{ Close() error }); ok {
				return closer.Close()
			}
			return nil
		}, nil
	}
	if closer, ok := globalStore.(interface{ Close() error }); ok {
		_ = closer.Close()
	}

	return nil, nil, nil
}
