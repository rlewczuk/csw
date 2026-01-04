Simplify function `FromConfig` in `pkg/models/config.go` to skip using `providerRegistry` and instead call the factory function directly.
Remove all `providerRegistry` related code, including `init()` functions in provider implementation files.
