package conf

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
