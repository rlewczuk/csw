package models

import (
	"fmt"
	"strings"

	"github.com/rlewczuk/csw/pkg/conf"
)

// NormalizeModelAliasMap converts config alias map into model-chain resolver input.
func NormalizeModelAliasMap(values map[string]conf.ModelAliasValue) (map[string][]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	result := make(map[string][]string, len(values))
	for alias, value := range values {
		trimmedAlias := strings.TrimSpace(alias)
		if trimmedAlias == "" {
			return nil, fmt.Errorf("NormalizeModelAliasMap() [model_aliases.go]: alias key cannot be empty")
		}
		if len(value.Values) == 0 {
			return nil, fmt.Errorf("NormalizeModelAliasMap() [model_aliases.go]: alias %q has no targets", trimmedAlias)
		}

		targets := make([]string, 0, len(value.Values))
		for _, target := range value.Values {
			trimmedTarget := strings.TrimSpace(target)
			if trimmedTarget == "" {
				return nil, fmt.Errorf("NormalizeModelAliasMap() [model_aliases.go]: alias %q contains empty target", trimmedAlias)
			}
			targets = append(targets, trimmedTarget)
		}

		result[trimmedAlias] = targets
	}

	return result, nil
}
