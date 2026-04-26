package system

import (
	"fmt"
	"strings"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
)

// ResolveModelName determines the model name to use.
func ResolveModelName(modelName string, configStore *conf.CswConfig, providerRegistry *models.ProviderRegistry) (string, error) {
	if modelName != "" {
		return ResolveModelSpec(modelName, configStore)
	}

	if configStore != nil && configStore.GlobalConfig != nil && configStore.GlobalConfig.Defaults.DefaultProvider != "" {
		globalConfig := configStore.GlobalConfig
		return globalConfig.Defaults.DefaultProvider + "/default", nil
	}

	providers := providerRegistry.List()
	if len(providers) > 0 {
		return providers[0] + "/default", nil
	}

	return "", fmt.Errorf("ResolveModelName() [bootstrap_models.go]: no default provider configured and no providers available")
}

// ResolveModelSpec resolves model alias or provider/model chain to normalized provider/model chain.
func ResolveModelSpec(modelSpec string, configStore *conf.CswConfig) (string, error) {
	trimmedModelSpec := strings.TrimSpace(modelSpec)
	if trimmedModelSpec == "" {
		return "", fmt.Errorf("ResolveModelSpec() [bootstrap_models.go]: model spec cannot be empty")
	}

	if configStore == nil {
		refs, err := models.ParseProviderModelChain(trimmedModelSpec)
		if err != nil {
			return "", fmt.Errorf("ResolveModelSpec() [bootstrap_models.go]: %w", err)
		}
		return models.ComposeProviderModelSpec(refs), nil
	}

	aliases, err := models.NormalizeModelAliasMap(configStore.ModelAliases)
	if err != nil {
		return "", fmt.Errorf("ResolveModelSpec() [bootstrap_models.go]: failed to normalize model aliases: %w", err)
	}

	refs, err := models.ExpandProviderModelChain(trimmedModelSpec, aliases)
	if err != nil {
		return "", fmt.Errorf("ResolveModelSpec() [bootstrap_models.go]: %w", err)
	}

	return models.ComposeProviderModelSpec(refs), nil
}

// CreateProviderMap creates a map of provider names to ModelProvider instances from a registry.
func CreateProviderMap(providerRegistry *models.ProviderRegistry) (map[string]models.ModelProvider, error) {
	modelProviders := make(map[string]models.ModelProvider)
	configStore := providerRegistry.ConfigStore()
	providerConfigPresence := make(map[string]struct{})
	if configStore != nil {
		var err error
		providerConfigPresence, err = resolveProviderConfigPresence(configStore)
		if err != nil {
			return nil, err
		}
	}

	for _, name := range providerRegistry.List() {
		provider, err := providerRegistry.Get(name)
		if err != nil {
			return nil, fmt.Errorf("CreateProviderMap() [bootstrap_models.go]: failed to get provider %s: %w", name, err)
		}

		if updaterTarget, ok := provider.(interface{ SetConfigUpdater(models.ConfigUpdater) }); ok {
			if _, exists := providerConfigPresence[name]; exists {
				updater := createConfigUpdaterFunc(name)
				updaterTarget.SetConfigUpdater(updater.Update())
			}
		}
		modelProviders[name] = provider
	}
	return modelProviders, nil
}

// applyDisableRefreshToProviders enables DisableRefresh flag on provider configs.
func applyDisableRefreshToProviders(modelProviders map[string]models.ModelProvider) {
	for _, provider := range modelProviders {
		providerConfigAccessor, ok := provider.(interface {
			GetConfig() *conf.ModelProviderConfig
		})
		if !ok {
			continue
		}
		providerConfig := providerConfigAccessor.GetConfig()
		if providerConfig == nil {
			continue
		}
		providerConfig.DisableRefresh = true
	}
}

func resolveProviderConfigPresence(configStore *conf.CswConfig) (map[string]struct{}, error) {
	resolved := make(map[string]struct{})
	for providerName := range configStore.ModelProviderConfigs {
		resolved[providerName] = struct{}{}
	}

	return resolved, nil
}

// CreateModelTagRegistry creates and populates a model tag registry from config store.
func CreateModelTagRegistry(configStore *conf.CswConfig, providerRegistry *models.ProviderRegistry) (*models.ModelTagRegistry, error) {
	modelTagRegistry := models.NewModelTagRegistry()

	if configStore != nil && configStore.GlobalConfig != nil && len(configStore.GlobalConfig.ModelTags) > 0 {
		if err := modelTagRegistry.SetGlobalMappings(configStore.GlobalConfig.ModelTags); err != nil {
			return nil, fmt.Errorf("CreateModelTagRegistry() [bootstrap_models.go]: failed to set global model tags: %w", err)
		}
	}

	for _, providerName := range providerRegistry.List() {
		provider, err := providerRegistry.Get(providerName)
		if err != nil {
			continue
		}
		if chatProvider, ok := provider.(interface{ GetConfig() interface{} }); ok {
			config := chatProvider.GetConfig()
			if providerConfig, ok := config.(*conf.ModelProviderConfig); ok && len(providerConfig.ModelTags) > 0 {
				if err := modelTagRegistry.SetProviderMappings(providerName, providerConfig.ModelTags); err != nil {
					return nil, fmt.Errorf("CreateModelTagRegistry() [bootstrap_models.go]: failed to set provider model tags: %w", err)
				}
			}
		}
	}

	return modelTagRegistry, nil
}
