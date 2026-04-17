package conf

import "strings"

// Clone returns a deep copy of ToolSelectionConfig.
func (c ToolSelectionConfig) Clone() ToolSelectionConfig {
	cloned := ToolSelectionConfig{
		Default: make(map[string]bool, len(c.Default)),
		Tags:    make(map[string]map[string]bool, len(c.Tags)),
	}

	for toolName, enabled := range c.Default {
		cloned.Default[toolName] = enabled
	}

	for tag, tools := range c.Tags {
		clonedTools := make(map[string]bool, len(tools))
		for toolName, enabled := range tools {
			clonedTools[toolName] = enabled
		}
		cloned.Tags[tag] = clonedTools
	}

	return cloned
}

// Merge merges overrides into ToolSelectionConfig.
//
// Default and tag-specific tool values are merged per key, and keys from override
// replace values from the receiver.
func (c *ToolSelectionConfig) Merge(override ToolSelectionConfig) {
	if c.Default == nil {
		c.Default = make(map[string]bool)
	}
	for toolName, enabled := range override.Default {
		c.Default[toolName] = enabled
	}

	if c.Tags == nil {
		c.Tags = make(map[string]map[string]bool)
	}
	for tag, tools := range override.Tags {
		if c.Tags[tag] == nil {
			c.Tags[tag] = make(map[string]bool)
		}
		for toolName, enabled := range tools {
			c.Tags[tag][toolName] = enabled
		}
	}
}

// Clone returns a deep copy of ContainerConfig.
func (c ContainerConfig) Clone() ContainerConfig {
	cloned := ContainerConfig{
		Mounts:  make([]string, len(c.Mounts)),
		Env:     make([]string, len(c.Env)),
		Image:   c.Image,
		Enabled: c.Enabled,
	}
	copy(cloned.Mounts, c.Mounts)
	copy(cloned.Env, c.Env)

	return cloned
}

// Merge merges overrides into ContainerConfig.
func (c *ContainerConfig) Merge(override ContainerConfig) {
	if len(override.Mounts) > 0 {
		c.Mounts = append([]string(nil), override.Mounts...)
	}
	if len(override.Env) > 0 {
		c.Env = append([]string(nil), override.Env...)
	}
	if override.Image != "" {
		c.Image = override.Image
	}
	if override.Enabled {
		c.Enabled = true
	}
}

// MergeFrom merges overrides into RunDefaultsConfig.
func (c *RunDefaultsConfig) MergeFrom(override RunDefaultsConfig) {
	if override.DefaultProvider != "" {
		c.DefaultProvider = override.DefaultProvider
	}
	if override.DefaultRole != "" {
		c.DefaultRole = override.DefaultRole
	}
	if override.Container != nil {
		overrideHasValue := override.Container.Enabled || override.Container.Image != "" || len(override.Container.Mounts) > 0 || len(override.Container.Env) > 0
		if c.Container == nil {
			if !overrideHasValue {
				goto mergeOtherFields
			}
			containerCopy := override.Container.Clone()
			c.Container = &containerCopy
		} else {
			c.Container.Merge(*override.Container)
		}
	}

mergeOtherFields:
	if override.Model != "" {
		c.Model = override.Model
	}
	if override.Worktree != "" {
		c.Worktree = override.Worktree
	}
	if override.Merge {
		c.Merge = true
	}
	if override.LogLLMRequests {
		c.LogLLMRequests = true
	}
	if override.LogLLMRequestsRaw {
		c.LogLLMRequestsRaw = true
	}
	if override.Thinking != "" {
		c.Thinking = override.Thinking
	}
	if override.LSPServer != "" {
		c.LSPServer = override.LSPServer
	}
	if override.GitUserName != "" {
		c.GitUserName = override.GitUserName
	}
	if override.GitUserEmail != "" {
		c.GitUserEmail = override.GitUserEmail
	}
	if override.MaxThreads > 0 {
		c.MaxThreads = override.MaxThreads
	}
	if override.TaskDir != "" {
		c.TaskDir = override.TaskDir
	}
	if override.ShadowDir != "" {
		c.ShadowDir = override.ShadowDir
	}
	if override.AllowAllPermissions {
		c.AllowAllPermissions = true
	}
	if len(override.VFSAllow) > 0 {
		c.VFSAllow = append([]string(nil), override.VFSAllow...)
	}
}

// Clone returns a deep copy of GlobalConfig.
func (c *GlobalConfig) Clone() *GlobalConfig {
	if c == nil {
		return nil
	}

	cloned := &GlobalConfig{
		ModelTags:                  make([]ModelTagMapping, len(c.ModelTags)),
		ToolSelection:              c.ToolSelection.Clone(),
		ContextCompactionThreshold: c.ContextCompactionThreshold,
		LLMRetryMaxAttempts:        c.LLMRetryMaxAttempts,
		LLMRetryMaxBackoffSeconds:  c.LLMRetryMaxBackoffSeconds,
		Defaults:                   c.Defaults,
		ShadowPaths:                append([]string(nil), c.ShadowPaths...),
	}
	if c.Defaults.Container != nil {
		containerCopy := c.Defaults.Container.Clone()
		cloned.Defaults.Container = &containerCopy
	}
	if len(c.Defaults.VFSAllow) > 0 {
		cloned.Defaults.VFSAllow = append([]string(nil), c.Defaults.VFSAllow...)
	}
	copy(cloned.ModelTags, c.ModelTags)

	return cloned
}

// Merge merges overrides into GlobalConfig.
func (c *GlobalConfig) Merge(override *GlobalConfig) {
	if override == nil {
		return
	}

	c.ModelTags = append(c.ModelTags, override.ModelTags...)
	c.ToolSelection.Merge(override.ToolSelection)

	if override.ContextCompactionThreshold > 0 {
		c.ContextCompactionThreshold = override.ContextCompactionThreshold
	}
	if override.LLMRetryMaxAttempts > 0 {
		c.LLMRetryMaxAttempts = override.LLMRetryMaxAttempts
	}
	if override.LLMRetryMaxBackoffSeconds > 0 {
		c.LLMRetryMaxBackoffSeconds = override.LLMRetryMaxBackoffSeconds
	}

	c.Defaults.MergeFrom(override.Defaults)
	if len(override.ShadowPaths) > 0 {
		c.ShadowPaths = append([]string(nil), override.ShadowPaths...)
	}

}

// Clone returns a deep copy of ModelProviderConfig.
func (c *ModelProviderConfig) Clone() *ModelProviderConfig {
	if c == nil {
		return nil
	}

	cloned := *c
	cloned.ModelTags = append([]ModelTagMapping(nil), c.ModelTags...)
	cloned.Cost = append([]ModelProviderCost(nil), c.Cost...)

	if c.Reasoning != nil {
		cloned.Reasoning = make(map[string]string, len(c.Reasoning))
		for key, value := range c.Reasoning {
			cloned.Reasoning[key] = value
		}
	}

	if c.Streaming != nil {
		streaming := *c.Streaming
		cloned.Streaming = &streaming
	}

	if c.Temperature != nil {
		temperature := *c.Temperature
		cloned.Temperature = &temperature
	}

	if c.ToolCall != nil {
		toolCall := *c.ToolCall
		cloned.ToolCall = &toolCall
	}

	if c.Experimental != nil {
		experimental := *c.Experimental
		cloned.Experimental = &experimental
	}

	if c.Headers != nil {
		cloned.Headers = make(map[string]string, len(c.Headers))
		for key, value := range c.Headers {
			cloned.Headers[key] = value
		}
	}

	if c.QueryParams != nil {
		cloned.QueryParams = make(map[string]string, len(c.QueryParams))
		for key, value := range c.QueryParams {
			cloned.QueryParams[key] = value
		}
	}

	if c.Options != nil {
		cloned.Options = make(map[string]any, len(c.Options))
		for key, value := range c.Options {
			cloned.Options[key] = value
		}
	}

	if c.Modalities != nil {
		cloned.Modalities = &ModelProviderModalities{
			Input:  append([]string(nil), c.Modalities.Input...),
			Output: append([]string(nil), c.Modalities.Output...),
		}
	}

	return &cloned
}

// Merge merges overrides into ModelProviderConfig.
func (c *ModelProviderConfig) Merge(override *ModelProviderConfig) {
	if override == nil {
		return
	}

	if override.Type != "" {
		c.Type = override.Type
	}
	if override.Name != "" {
		c.Name = override.Name
	}
	if override.Description != "" {
		c.Description = override.Description
	}
	if override.Family != "" {
		c.Family = override.Family
	}
	if override.ReleaseDate != "" {
		c.ReleaseDate = override.ReleaseDate
	}
	if override.URL != "" {
		c.URL = override.URL
	}
	if override.APIKey != "" {
		c.APIKey = override.APIKey
	}
	if override.ConnectTimeout > 0 {
		c.ConnectTimeout = override.ConnectTimeout
	}
	if override.RequestTimeout > 0 {
		c.RequestTimeout = override.RequestTimeout
	}
	if override.DefaultTemperature != 0 {
		c.DefaultTemperature = override.DefaultTemperature
	}
	if override.DefaultTopP != 0 {
		c.DefaultTopP = override.DefaultTopP
	}
	if override.DefaultTopK != 0 {
		c.DefaultTopK = override.DefaultTopK
	}
	if override.ContextLengthLimit > 0 {
		c.ContextLengthLimit = override.ContextLengthLimit
	}
	if override.MaxTokens > 0 {
		c.MaxTokens = override.MaxTokens
	}
	if override.MaxInputTokens > 0 {
		c.MaxInputTokens = override.MaxInputTokens
	}
	if override.MaxOutputTokens > 0 {
		c.MaxOutputTokens = override.MaxOutputTokens
	}
	if override.ReasoningContent != "" {
		c.ReasoningContent = override.ReasoningContent
	}
	if override.Interleaved != "" {
		c.Interleaved = override.Interleaved
	}
	if override.Status != "" {
		c.Status = override.Status
	}
	if override.MaxRetries > 0 {
		c.MaxRetries = override.MaxRetries
	}
	if override.RateLimitBackoffScale > 0 {
		c.RateLimitBackoffScale = override.RateLimitBackoffScale
	}
	if override.AuthMode != "" {
		c.AuthMode = override.AuthMode
	}
	if override.AuthURL != "" {
		c.AuthURL = override.AuthURL
	}
	if override.TokenURL != "" {
		c.TokenURL = override.TokenURL
	}
	if override.ClientID != "" {
		c.ClientID = override.ClientID
	}
	if override.ClientSecret != "" {
		c.ClientSecret = override.ClientSecret
	}
	if override.RefreshToken != "" {
		c.RefreshToken = override.RefreshToken
	}
	if override.DisableRefresh {
		c.DisableRefresh = true
	}

	if override.Streaming != nil {
		streaming := *override.Streaming
		c.Streaming = &streaming
	}
	if override.Temperature != nil {
		temperature := *override.Temperature
		c.Temperature = &temperature
	}
	if override.ToolCall != nil {
		toolCall := *override.ToolCall
		c.ToolCall = &toolCall
	}
	if override.Experimental != nil {
		experimental := *override.Experimental
		c.Experimental = &experimental
	}

	c.ModelTags = mergeModelTagMappings(c.ModelTags, override.ModelTags)
	c.Cost = mergeModelProviderCosts(c.Cost, override.Cost)

	if c.Reasoning == nil {
		c.Reasoning = make(map[string]string)
	}
	for key, value := range override.Reasoning {
		c.Reasoning[key] = value
	}

	if c.Headers == nil {
		c.Headers = make(map[string]string)
	}
	for key, value := range override.Headers {
		c.Headers[key] = value
	}

	if c.QueryParams == nil {
		c.QueryParams = make(map[string]string)
	}
	for key, value := range override.QueryParams {
		c.QueryParams[key] = value
	}

	if c.Options == nil {
		c.Options = make(map[string]any)
	}
	for key, value := range override.Options {
		c.Options[key] = value
	}

	if override.Modalities != nil {
		if c.Modalities == nil {
			c.Modalities = &ModelProviderModalities{}
		}
		if len(override.Modalities.Input) > 0 {
			c.Modalities.Input = append([]string(nil), override.Modalities.Input...)
		}
		if len(override.Modalities.Output) > 0 {
			c.Modalities.Output = append([]string(nil), override.Modalities.Output...)
		}
	}
}

// Clone returns a deep copy of ModelVendorFamilyTemplateOverride.
func (c *ModelVendorFamilyTemplateOverride) Clone() *ModelVendorFamilyTemplateOverride {
	if c == nil {
		return nil
	}

	cloned := *c
	cloned.Vendor = *c.Vendor.Clone()
	if c.Families != nil {
		cloned.Families = make(map[string]ModelProviderConfig, len(c.Families))
		for key, value := range c.Families {
			cloned.Families[key] = *value.Clone()
		}
	}

	return &cloned
}

// Merge merges overrides into ModelVendorFamilyTemplateOverride.
func (c *ModelVendorFamilyTemplateOverride) Merge(override *ModelVendorFamilyTemplateOverride) {
	if override == nil {
		return
	}

	c.Vendor.Merge(&override.Vendor)
	if c.Families == nil {
		c.Families = make(map[string]ModelProviderConfig)
	}
	for key, value := range override.Families {
		if existing, ok := c.Families[key]; ok {
			existing.Merge(&value)
			c.Families[key] = existing
			continue
		}
		c.Families[key] = *value.Clone()
	}
}

func mergeModelTagMappings(base []ModelTagMapping, override []ModelTagMapping) []ModelTagMapping {
	result := append([]ModelTagMapping(nil), base...)
	for _, item := range override {
		replaced := false
		for idx := range result {
			if result[idx].Model == item.Model {
				result[idx] = item
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, item)
		}
	}
	return result
}

func mergeModelProviderCosts(base []ModelProviderCost, override []ModelProviderCost) []ModelProviderCost {
	merged := make(map[int]ModelProviderCost, len(base)+len(override))
	for _, item := range base {
		merged[item.Context] = item
	}
	for _, item := range override {
		existing, ok := merged[item.Context]
		if !ok {
			merged[item.Context] = item
			continue
		}
		if item.Input != 0 {
			existing.Input = item.Input
		}
		if item.Output != 0 {
			existing.Output = item.Output
		}
		if item.CacheRead != 0 {
			existing.CacheRead = item.CacheRead
		}
		if item.CacheWrite != 0 {
			existing.CacheWrite = item.CacheWrite
		}
		if item.Context != 0 {
			existing.Context = item.Context
		}
		merged[item.Context] = existing
	}

	result := make([]ModelProviderCost, 0, len(merged))
	for _, item := range merged {
		result = append(result, item)
	}
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Context < result[i].Context {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

func cloneModelProviderMapValue(input map[string]ModelProviderConfig) map[string]ModelProviderConfig {
	if input == nil {
		return nil
	}
	cloned := make(map[string]ModelProviderConfig, len(input))
	for key, value := range input {
		cloned[key] = *value.Clone()
	}
	return cloned
}

func cloneModelTemplateGroups(input map[string]map[string]ModelProviderConfig) map[string]map[string]ModelProviderConfig {
	if input == nil {
		return nil
	}
	cloned := make(map[string]map[string]ModelProviderConfig, len(input))
	for group, templates := range input {
		cloned[group] = cloneModelProviderMapValue(templates)
	}
	return cloned
}

// Clone returns a deep copy of HookConfig.
func (c *HookConfig) Clone() *HookConfig {
	if c == nil {
		return nil
	}

	cloned := *c
	if c.EmbeddedFiles != nil {
		cloned.EmbeddedFiles = make(map[string][]byte, len(c.EmbeddedFiles))
		for key, value := range c.EmbeddedFiles {
			copied := make([]byte, len(value))
			copy(copied, value)
			cloned.EmbeddedFiles[key] = copied
		}
	}
	return &cloned
}

// Clone returns a deep copy of MCPServerConfig.
func (c *MCPServerConfig) Clone() *MCPServerConfig {
	if c == nil {
		return nil
	}

	cloned := &MCPServerConfig{
		Name:        c.Name,
		Description: c.Description,
		Transport:   c.Transport,
		URL:         c.URL,
		APIKey:      c.APIKey,
		Cmd:         c.Cmd,
		Enabled:     c.Enabled,
		Args:        append([]string(nil), c.Args...),
		Tools:       append([]string(nil), c.Tools...),
	}

	if c.Env != nil {
		cloned.Env = make(map[string]string, len(c.Env))
		for key, value := range c.Env {
			cloned.Env[key] = value
		}
	}

	return cloned
}

func cloneVendorFamilyOverrides(input map[string]ModelVendorFamilyTemplateOverride) map[string]ModelVendorFamilyTemplateOverride {
	if input == nil {
		return nil
	}
	cloned := make(map[string]ModelVendorFamilyTemplateOverride, len(input))
	for key, value := range input {
		cloned[key] = *value.Clone()
	}
	return cloned
}

// Clone returns a deep copy of AgentRoleConfig.
func (c *AgentRoleConfig) Clone() *AgentRoleConfig {
	if c == nil {
		return nil
	}

	cloned := *c

	if c.VFSPrivileges != nil {
		cloned.VFSPrivileges = make(map[string]FileAccess, len(c.VFSPrivileges))
		for key, value := range c.VFSPrivileges {
			cloned.VFSPrivileges[key] = value
		}
	}

	if c.ToolsAccess != nil {
		cloned.ToolsAccess = make(map[string]AccessFlag, len(c.ToolsAccess))
		for key, value := range c.ToolsAccess {
			cloned.ToolsAccess[key] = value
		}
	}

	if c.RunPrivileges != nil {
		cloned.RunPrivileges = make(map[string]AccessFlag, len(c.RunPrivileges))
		for key, value := range c.RunPrivileges {
			cloned.RunPrivileges[key] = value
		}
	}

	if c.PromptFragments != nil {
		cloned.PromptFragments = make(map[string]string, len(c.PromptFragments))
		for key, value := range c.PromptFragments {
			cloned.PromptFragments[key] = value
		}
	}

	if c.ToolFragments != nil {
		cloned.ToolFragments = make(map[string]string, len(c.ToolFragments))
		for key, value := range c.ToolFragments {
			cloned.ToolFragments[key] = value
		}
	}

	if c.HiddenPatterns != nil {
		cloned.HiddenPatterns = make([]string, len(c.HiddenPatterns))
		copy(cloned.HiddenPatterns, c.HiddenPatterns)
	}

	if c.Aliases != nil {
		cloned.Aliases = make([]string, len(c.Aliases))
		copy(cloned.Aliases, c.Aliases)
	}

	if c.MCPServers != nil {
		cloned.MCPServers = make([]string, len(c.MCPServers))
		copy(cloned.MCPServers, c.MCPServers)
	}

	return &cloned
}

// Merge merges overrides into AgentRoleConfig.
//
// Scalar fields and privilege maps are replaced by override values. PromptFragments
// and ToolFragments are merged by key; empty or whitespace-only content in override
// removes an existing fragment. HiddenPatterns are appended.
func (c *AgentRoleConfig) Merge(override *AgentRoleConfig) {
	if override == nil {
		return
	}

	existingPromptFragments := cloneStringMap(c.PromptFragments)
	existingToolFragments := cloneStringMap(c.ToolFragments)
	existingHiddenPatterns := append([]string(nil), c.HiddenPatterns...)

	clonedOverride := override.Clone()
	*c = *clonedOverride

	c.PromptFragments = existingPromptFragments
	mergeFragments(c.PromptFragments, override.PromptFragments)

	c.ToolFragments = existingToolFragments
	mergeFragments(c.ToolFragments, override.ToolFragments)

	c.HiddenPatterns = append(existingHiddenPatterns, override.HiddenPatterns...)
	if override.Aliases != nil {
		c.Aliases = append([]string(nil), override.Aliases...)
	}
	if len(override.MCPServers) > 0 {
		c.MCPServers = append([]string(nil), override.MCPServers...)
	}
}

func cloneStringMap(value map[string]string) map[string]string {
	if value == nil {
		return make(map[string]string)
	}

	cloned := make(map[string]string, len(value))
	for key, item := range value {
		cloned[key] = item
	}

	return cloned
}

func mergeFragments(target map[string]string, overrides map[string]string) {
	for filename, content := range overrides {
		if strings.TrimSpace(content) == "" {
			delete(target, filename)
			continue
		}

		target[filename] = content
	}
}
