package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/system"
)

// ConfigScope represents the scope of configuration (global or local).
type ConfigScope string

const (
	ConfigScopeLocal  ConfigScope = "local"
	ConfigScopeGlobal ConfigScope = "global"
)

// GetConfigStore returns a local config store for the specified scope or custom path.
func GetConfigStore(scope ConfigScope) (*conf.CswConfig, error) {
	var configDir string
	configRoot, err := resolveConfigRootFromShadow()
	if err != nil {
		return nil, fmt.Errorf("GetConfigStore() [common.go]: failed to resolve config root: %w", err)
	}

	switch scope {
	case ConfigScopeLocal:
		configDir = filepath.Join(configRoot, ".csw", "config")
	case ConfigScopeGlobal:
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("GetConfigStore() [common.go]: failed to get home directory: %w", err)
		}
		configDir = filepath.Join(homeDir, ".config", "csw")
	default:
		configDir = string(scope)
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("GetConfigStore() [common.go]: failed to create config directory: %w", err)
	}

	store, err := conf.CswConfigLoad(configDir)
	if err != nil {
		return nil, fmt.Errorf("GetConfigStore() [common.go]: failed to load config: %w", err)
	}

	return store, nil
}

// GetCompositeConfigStore returns a composite config store that merges configurations.
func GetCompositeConfigStore() (*conf.CswConfig, error) {
	configRoot, err := resolveConfigRootFromShadow()
	if err != nil {
		return nil, fmt.Errorf("GetCompositeConfigStore() [common.go]: failed to resolve config root: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("GetCompositeConfigStore() [common.go]: failed to get home directory: %w", err)
	}

	configPathStr := "@DEFAULTS:" + filepath.Join(homeDir, ".config", "csw") + ":" + filepath.Join(configRoot, ".csw", "config")

	store, err := conf.CswConfigLoad(configPathStr)
	if err != nil {
		return nil, fmt.Errorf("GetCompositeConfigStore() [common.go]: failed to load config: %w", err)
	}

	return store, nil
}

func resolveConfigRootFromShadow() (string, error) {
	trimmedShadowDir := strings.TrimSpace(shadowDir)
	if trimmedShadowDir != "" {
		resolvedShadowDir, err := system.ResolveWorkDir(trimmedShadowDir)
		if err != nil {
			return "", fmt.Errorf("resolveConfigRootFromShadow() [common.go]: failed to resolve shadow directory: %w", err)
		}
		return resolvedShadowDir, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolveConfigRootFromShadow() [common.go]: failed to get current directory: %w", err)
	}

	return cwd, nil
}

// BuildConfigPath builds a config path hierarchy string.
func BuildConfigPath(projectConfig, customConfigPath string) (string, error) {
	return system.BuildConfigPath(projectConfig, customConfigPath)
}

// ValidateConfigPaths validates that all paths in a colon-separated string exist and are directories.
func ValidateConfigPaths(configPath string) error {
	return system.ValidateConfigPaths(configPath)
}

// ResolveWorkDir resolves working directory from an optional path argument.
func ResolveWorkDir(dirPath string) (string, error) {
	return system.ResolveWorkDir(dirPath)
}

// ResolveModelName determines model name to use.
func ResolveModelName(modelName string, configStore *conf.CswConfig, providerRegistry *models.ProviderRegistry) (string, error) {
	if modelName != "" {
		return ResolveModelSpec(modelName, configStore)
	}

	if configStore != nil && configStore.GlobalConfig != nil && configStore.GlobalConfig.Defaults.DefaultProvider != "" {
		return configStore.GlobalConfig.Defaults.DefaultProvider + "/default", nil
	}

	providers := providerRegistry.List()
	if len(providers) > 0 {
		return providers[0] + "/default", nil
	}

	return "", fmt.Errorf("ResolveModelName() [common.go]: no default provider configured and no providers available")
}

// ResolveModelSpec resolves model alias or provider/model chain to normalized provider/model chain.
func ResolveModelSpec(modelSpec string, configStore *conf.CswConfig) (string, error) {
	trimmedModelSpec := strings.TrimSpace(modelSpec)
	if trimmedModelSpec == "" {
		return "", fmt.Errorf("ResolveModelSpec() [common.go]: model spec cannot be empty")
	}

	if configStore == nil {
		refs, err := models.ParseProviderModelChain(trimmedModelSpec)
		if err != nil {
			return "", fmt.Errorf("ResolveModelSpec() [common.go]: %w", err)
		}
		return models.ComposeProviderModelSpec(refs), nil
	}

	aliases, err := models.NormalizeModelAliasMap(configStore.ModelAliases)
	if err != nil {
		return "", fmt.Errorf("ResolveModelSpec() [common.go]: failed to normalize model aliases: %w", err)
	}

	refs, err := models.ExpandProviderModelChain(trimmedModelSpec, aliases)
	if err != nil {
		return "", fmt.Errorf("ResolveModelSpec() [common.go]: %w", err)
	}

	return models.ComposeProviderModelSpec(refs), nil
}

// CreateProviderMap creates a map of provider names to ModelProvider instances from a registry.
func CreateProviderMap(providerRegistry *models.ProviderRegistry) (map[string]models.ModelProvider, error) {
	return system.CreateProviderMap(providerRegistry)
}

// CreateModelTagRegistry creates and populates model tag registry from config store.
func CreateModelTagRegistry(configStore *conf.CswConfig, providerRegistry *models.ProviderRegistry) (*models.ModelTagRegistry, error) {
	modelTagRegistry := models.NewModelTagRegistry()

	if configStore != nil && configStore.GlobalConfig != nil && len(configStore.GlobalConfig.ModelTags) > 0 {
		if err := modelTagRegistry.SetGlobalMappings(configStore.GlobalConfig.ModelTags); err != nil {
			return nil, fmt.Errorf("CreateModelTagRegistry() [common.go]: failed to set global model tags: %w", err)
		}
	}

	for _, providerName := range providerRegistry.List() {
		provider, err := providerRegistry.Get(providerName)
		if err != nil {
			continue
		}
		if chatProvider, ok := provider.(interface{ GetConfig() interface{} }); ok {
			providerConfig, ok := chatProvider.GetConfig().(*conf.ModelProviderConfig)
			if ok && len(providerConfig.ModelTags) > 0 {
				if err := modelTagRegistry.SetProviderMappings(providerName, providerConfig.ModelTags); err != nil {
					return nil, fmt.Errorf("CreateModelTagRegistry() [common.go]: failed to set provider model tags: %w", err)
				}
			}
		}
	}

	return modelTagRegistry, nil
}
