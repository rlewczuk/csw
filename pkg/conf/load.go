package conf

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// CswConfigLoad loads consolidated config from a colon-separated source path.
func CswConfigLoad(path string) (*CswConfig, error) {
	sources, err := resolveConfigLoadSources(path)
	if err != nil {
		return nil, err
	}

	mergedGlobalConfig := &GlobalConfig{}
	mergedModelProviderConfigs := make(map[string]*ModelProviderConfig)
	mergedModelAliases := make(map[string]ModelAliasValue)
	mergedAgentRoleConfigs := make(map[string]*AgentRoleConfig)
	mergedAgentConfigFiles := make(map[string]map[string]string)
	for sourceIndex := range sources {
		sourceGlobalConfig, loadErr := loadGlobalConfigFromDir(sources[sourceIndex])
		if loadErr != nil {
			return nil, fmt.Errorf("CswConfigLoad() [load.go]: failed to load global config from source %q: %w", sources[sourceIndex], loadErr)
		}
		mergedGlobalConfig.Merge(sourceGlobalConfig)

		sourceModelProviderConfigs, loadErr := loadModelProviderConfigsFromDir(sources[sourceIndex])
		if loadErr != nil {
			return nil, fmt.Errorf("CswConfigLoad() [load.go]: failed to load model providers from source %q: %w", sources[sourceIndex], loadErr)
		}
		for providerName, providerConfig := range sourceModelProviderConfigs {
			mergedModelProviderConfigs[providerName] = providerConfig.Clone()
		}

		sourceModelAliases, loadErr := loadModelAliasesFromDir(sources[sourceIndex])
		if loadErr != nil {
			return nil, fmt.Errorf("CswConfigLoad() [load.go]: failed to load model aliases from source %q: %w", sources[sourceIndex], loadErr)
		}
		for aliasName, aliasValue := range sourceModelAliases {
			mergedModelAliases[aliasName] = ModelAliasValue{Values: append([]string(nil), aliasValue.Values...)}
		}

		sourceAgentRoleConfigs, loadErr := loadAgentRoleConfigsFromDir(sources[sourceIndex])
		if loadErr != nil {
			return nil, fmt.Errorf("CswConfigLoad() [load.go]: failed to load agent roles from source %q: %w", sources[sourceIndex], loadErr)
		}
		for roleName, roleConfig := range sourceAgentRoleConfigs {
			existingRoleConfig, exists := mergedAgentRoleConfigs[roleName]
			if !exists {
				mergedAgentRoleConfigs[roleName] = roleConfig.Clone()
				continue
			}

			existingRoleConfig.Merge(roleConfig)
		}

		sourceAgentConfigFiles, loadErr := loadAgentConfigFilesFromDir(sources[sourceIndex])
		if loadErr != nil {
			return nil, fmt.Errorf("CswConfigLoad() [load.go]: failed to load agent config files from source %q: %w", sources[sourceIndex], loadErr)
		}
		for subdir, files := range sourceAgentConfigFiles {
			existingFiles, exists := mergedAgentConfigFiles[subdir]
			if !exists {
				existingFiles = make(map[string]string)
				mergedAgentConfigFiles[subdir] = existingFiles
			}

			for filename, content := range files {
				existingFiles[filename] = content
			}
		}
	}

	return &CswConfig{
		GlobalConfig:          mergedGlobalConfig,
		AgentRoleConfigs:      mergedAgentRoleConfigs,
		ModelProviderConfigs: mergedModelProviderConfigs,
		AgentConfigFiles:      mergedAgentConfigFiles,
		ModelAliases:         mergedModelAliases,
	}, nil
}

func parseConfigLoadPath(path string) []string {
	if strings.TrimSpace(path) == "" {
		return []string{"@DEFAULTS"}
	}

	parts := strings.Split(path, ":")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}

	return result
}

func resolveConfigLoadSources(path string) ([]string, error) {
	configPathParts := parseConfigLoadPath(path)
	sources := make([]string, 0, len(configPathParts))

	for _, rawPathPart := range configPathParts {
		resolvedPathPart, err := resolveConfigPathPart(rawPathPart)
		if err != nil {
			return nil, err
		}

		pathInfo, statErr := os.Stat(resolvedPathPart)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			return nil, fmt.Errorf("resolveConfigLoadSources() [load.go]: failed to read path %q: %w", resolvedPathPart, statErr)
		}

		if !pathInfo.IsDir() {
			return nil, fmt.Errorf("resolveConfigLoadSources() [load.go]: path is not a directory: %q", resolvedPathPart)
		}

		sources = append(sources, resolvedPathPart)
	}

	return sources, nil
}

func resolveConfigPathPart(pathPart string) (string, error) {
	if pathPart == "@DEFAULTS" {
		return defaultConfigPath()
	}

	if strings.HasPrefix(pathPart, "@PROJ/") {
		workingDir, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolveConfigPathPart() [load.go]: failed to get current working directory: %w", err)
		}
		return filepath.Join(workingDir, strings.TrimPrefix(pathPart, "@PROJ/")), nil
	}

	if strings.HasPrefix(pathPart, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolveConfigPathPart() [load.go]: failed to get user home directory: %w", err)
		}
		return filepath.Join(homeDir, strings.TrimPrefix(pathPart, "~/")), nil
	}

	if strings.HasPrefix(pathPart, "./") {
		workingDir, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolveConfigPathPart() [load.go]: failed to get current working directory: %w", err)
		}
		return filepath.Join(workingDir, strings.TrimPrefix(pathPart, "./")), nil
	}

	return pathPart, nil
}

func defaultConfigPath() (string, error) {
	_, thisFilePath, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("defaultConfigPath() [load.go]: failed to resolve caller file path")
	}

	return filepath.Join(filepath.Dir(thisFilePath), "impl", "conf"), nil
}

func loadGlobalConfigFromDir(configDir string) (*GlobalConfig, error) {
	globalConfigPath := filepath.Join(configDir, "global.json")
	data, err := os.ReadFile(globalConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &GlobalConfig{}, nil
		}
		return nil, fmt.Errorf("loadGlobalConfigFromDir() [load.go]: failed to read %q: %w", globalConfigPath, err)
	}

	var globalConfig GlobalConfig
	if err := json.Unmarshal(data, &globalConfig); err != nil {
		return nil, fmt.Errorf("loadGlobalConfigFromDir() [load.go]: failed to parse %q: %w", globalConfigPath, err)
	}

	return &globalConfig, nil
}

func loadModelProviderConfigsFromDir(configDir string) (map[string]*ModelProviderConfig, error) {
	modelConfigsPath := filepath.Join(configDir, "models")
	entries, err := os.ReadDir(modelConfigsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]*ModelProviderConfig{}, nil
		}
		return nil, fmt.Errorf("loadModelProviderConfigsFromDir() [load.go]: failed to read %q: %w", modelConfigsPath, err)
	}

	modelProviderConfigs := make(map[string]*ModelProviderConfig)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		extension := strings.ToLower(filepath.Ext(entry.Name()))
		if extension != ".json" {
			continue
		}

		providerName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		providerPath := filepath.Join(modelConfigsPath, entry.Name())
		data, readErr := os.ReadFile(providerPath)
		if readErr != nil {
			return nil, fmt.Errorf("loadModelProviderConfigsFromDir() [load.go]: failed to read %q: %w", providerPath, readErr)
		}

		var providerConfig ModelProviderConfig
		if unmarshalErr := json.Unmarshal(data, &providerConfig); unmarshalErr != nil {
			return nil, fmt.Errorf("loadModelProviderConfigsFromDir() [load.go]: failed to parse %q: %w", providerPath, unmarshalErr)
		}

		providerConfig.Name = providerName
		modelProviderConfigs[providerName] = &providerConfig
	}

	return modelProviderConfigs, nil
}

func loadModelAliasesFromDir(configDir string) (map[string]ModelAliasValue, error) {
	aliasesPath := filepath.Join(configDir, "model_aliases.jsonl")
	data, err := os.ReadFile(aliasesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]ModelAliasValue{}, nil
		}
		return nil, fmt.Errorf("loadModelAliasesFromDir() [load.go]: failed to read %q: %w", aliasesPath, err)
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return map[string]ModelAliasValue{}, nil
	}

	aliases := make(map[string]ModelAliasValue)
	if strings.HasPrefix(trimmed, "{") {
		if err := json.Unmarshal([]byte(trimmed), &aliases); err != nil {
			return nil, fmt.Errorf("loadModelAliasesFromDir() [load.go]: failed to parse %q: %w", aliasesPath, err)
		}
		return aliases, nil
	}

	lines := strings.Split(trimmed, "\n")
	for lineIndex, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}

		entry := make(map[string]ModelAliasValue)
		if err := json.Unmarshal([]byte(trimmedLine), &entry); err != nil {
			return nil, fmt.Errorf("loadModelAliasesFromDir() [load.go]: failed to parse %q line %d: %w", aliasesPath, lineIndex+1, err)
		}

		for aliasName, aliasValue := range entry {
			aliases[aliasName] = aliasValue
		}
	}

	return aliases, nil
}

func loadAgentRoleConfigsFromDir(configDir string) (map[string]*AgentRoleConfig, error) {
	rolesDir := filepath.Join(configDir, "roles")
	entries, err := os.ReadDir(rolesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]*AgentRoleConfig{}, nil
		}
		return nil, fmt.Errorf("loadAgentRoleConfigsFromDir() [load.go]: failed to read %q: %w", rolesDir, err)
	}

	agentRoleConfigs := make(map[string]*AgentRoleConfig)
	toolFragments, err := loadToolFragmentsFromDir(configDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		roleName := entry.Name()
		roleDir := filepath.Join(rolesDir, roleName)
		roleConfigPath := filepath.Join(roleDir, "config.json")
		data, readErr := os.ReadFile(roleConfigPath)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				if roleName == "all" {
					promptFragments, loadErr := loadPromptFragmentsFromDir(roleDir)
					if loadErr != nil {
						return nil, loadErr
					}
					agentRoleConfigs["all"] = &AgentRoleConfig{
						Name:            "all",
						PromptFragments: promptFragments,
						ToolFragments:   cloneStringMapLoad(toolFragments),
					}
					continue
				}
				return nil, fmt.Errorf("loadAgentRoleConfigsFromDir() [load.go]: role %q missing config.json", roleName)
			}
			return nil, fmt.Errorf("loadAgentRoleConfigsFromDir() [load.go]: failed to read %q: %w", roleConfigPath, readErr)
		}

		var roleConfig AgentRoleConfig
		if unmarshalErr := json.Unmarshal(data, &roleConfig); unmarshalErr != nil {
			return nil, fmt.Errorf("loadAgentRoleConfigsFromDir() [load.go]: failed to parse %q: %w", roleConfigPath, unmarshalErr)
		}

		if roleConfig.Name == "" {
			roleConfig.Name = roleName
		}

		promptFragments, loadErr := loadPromptFragmentsFromDir(roleDir)
		if loadErr != nil {
			return nil, loadErr
		}
		roleConfig.PromptFragments = promptFragments
		roleConfig.ToolFragments = cloneStringMapLoad(toolFragments)

		agentRoleConfigs[roleConfig.Name] = &roleConfig
	}

	return agentRoleConfigs, nil
}

func loadPromptFragmentsFromDir(roleDir string) (map[string]string, error) {
	entries, err := os.ReadDir(roleDir)
	if err != nil {
		return nil, fmt.Errorf("loadPromptFragmentsFromDir() [load.go]: failed to read %q: %w", roleDir, err)
	}

	fragments := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if strings.ToLower(filepath.Ext(entry.Name())) != ".md" {
			continue
		}

		fragmentPath := filepath.Join(roleDir, entry.Name())
		data, readErr := os.ReadFile(fragmentPath)
		if readErr != nil {
			return nil, fmt.Errorf("loadPromptFragmentsFromDir() [load.go]: failed to read %q: %w", fragmentPath, readErr)
		}

		fragmentName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		fragments[fragmentName] = string(data)
	}

	return fragments, nil
}

func loadToolFragmentsFromDir(configDir string) (map[string]string, error) {
	toolsDir := filepath.Join(configDir, "tools")
	entries, err := os.ReadDir(toolsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("loadToolFragmentsFromDir() [load.go]: failed to read %q: %w", toolsDir, err)
	}

	fragments := make(map[string]string)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		toolName := entry.Name()
		toolDir := filepath.Join(toolsDir, toolName)
		fragments[fmt.Sprintf("%s/.tooldir", toolName)] = toolDir

		toolEntries, readDirErr := os.ReadDir(toolDir)
		if readDirErr != nil {
			return nil, fmt.Errorf("loadToolFragmentsFromDir() [load.go]: failed to read %q: %w", toolDir, readDirErr)
		}

		for _, toolEntry := range toolEntries {
			if toolEntry.IsDir() {
				continue
			}

			toolFilePath := filepath.Join(toolDir, toolEntry.Name())
			data, readErr := os.ReadFile(toolFilePath)
			if readErr != nil {
				return nil, fmt.Errorf("loadToolFragmentsFromDir() [load.go]: failed to read %q: %w", toolFilePath, readErr)
			}

			fragments[fmt.Sprintf("%s/%s", toolName, toolEntry.Name())] = string(data)
		}
	}

	return fragments, nil
}

func cloneStringMapLoad(input map[string]string) map[string]string {
	cloned := make(map[string]string, len(input))
	for key, value := range input {
		cloned[key] = value
	}

	return cloned
}

func loadAgentConfigFilesFromDir(configDir string) (map[string]map[string]string, error) {
	agentDir := filepath.Join(configDir, "agent")
	if _, err := os.Stat(agentDir); err != nil {
		if os.IsNotExist(err) {
			return map[string]map[string]string{}, nil
		}
		return nil, fmt.Errorf("loadAgentConfigFilesFromDir() [load.go]: failed to stat %q: %w", agentDir, err)
	}

	result := make(map[string]map[string]string)
	walkErr := filepath.WalkDir(agentDir, func(path string, dirEntry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("loadAgentConfigFilesFromDir() [load.go]: failed to walk %q: %w", path, walkErr)
		}

		if dirEntry.IsDir() {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("loadAgentConfigFilesFromDir() [load.go]: failed to read %q: %w", path, readErr)
		}

		relPath, relErr := filepath.Rel(agentDir, path)
		if relErr != nil {
			return fmt.Errorf("loadAgentConfigFilesFromDir() [load.go]: failed to resolve relative path for %q: %w", path, relErr)
		}

		subdir := filepath.Dir(relPath)
		if subdir == "." {
			subdir = ""
		}

		if result[subdir] == nil {
			result[subdir] = make(map[string]string)
		}

		filename := filepath.Base(relPath)
		result[subdir][filename] = string(data)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	return result, nil
}
