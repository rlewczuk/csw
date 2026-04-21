package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/spf13/cobra"
)

// modelEntry represents a model from a specific provider
type modelEntry struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

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
	cmd.AddCommand(providerTestCommand(&scope))
	cmd.AddCommand(providerAuthCommand())
	cmd.AddCommand(providerModelsCommand(&useJSON))

	return cmd
}

// Helper functions

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

func findWritableStoreForProvider(providerName string) (*conf.CswConfig, func() error, error) {
	localStore, err := GetConfigStore(ConfigScopeLocal)
	if err != nil {
		return nil, nil, fmt.Errorf("findWritableStoreForProvider() [provider.go]: failed to open local config store: %w", err)
	}
	localConfigs := localStore.ModelProviderConfigs
	if _, exists := localConfigs[providerName]; exists {
		return localStore, nil, nil
	}

	globalStore, err := GetConfigStore(ConfigScopeGlobal)
	if err != nil {
		return nil, nil, fmt.Errorf("findWritableStoreForProvider() [provider.go]: failed to open global config store: %w", err)
	}
	globalConfigs := globalStore.ModelProviderConfigs
	if _, exists := globalConfigs[providerName]; exists {
		return globalStore, nil, nil
	}

	return nil, nil, nil
}
