Derive common configuration data for different model providers: ollama, openai, anthropic:
* in `pkg/models/config.go` create structure `ModelProviderConfig` with fields:
  * `Type` - type of the provider (e.g. "ollama", "openai", "anthropic")
  * `Name` - name of the provider (e.g. "ollama_local", "openai_cloud", "anthropic")
  * `URL` - base URL for the provider
  * `APIKey` - API key for the provider (if any)
  * `ConnectTimeout` - connection timeout
  * `RequestTimeout` - request timeout
  * `DefaultTemperature` - default temperature
  * `DefaultTopP` - default top_p
  * `DefaultTopK` - default top_k
  * `ContextLengthLimit` - maximum context length (in tokens)
* implement function `FromConfig()` which will create a new instance of the provider from the configuration, automatically selecting the right implementation based on `Type` field;
* modify `pkg/models/ollama/ollama_client.go` to use `ModelProviderConfig` and `FromConfig` function instead of custom structure;
* modify `pkg/models/openai/openai_client.go` to use `ModelProviderConfig` and `FromConfig` function instead of custom structure;
* modify `pkg/models/anthropic/anthropic_client.go` to use `ModelProviderConfig` and `FromConfig` function instead of custom structure;
* adapt unit tests for the above changes;
* make sure tests are running properly in integration mode (`_integ/all.enabled` etc.)
