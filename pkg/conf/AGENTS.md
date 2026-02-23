# pkg/conf

`pkg/conf` defines the configuration domain for CSW and the config-store abstractions used by the rest of the system. It covers global settings, model providers, agent roles, tool and file access policies, and layered config loading across defaults and local/project scopes.

## Major files

- `conf.go`: Core public configuration API. Defines config structs and major interfaces like `ConfigStore` and `WritableConfigStore`.
- `impl/local.go`: Local filesystem-backed config store with read/write operations, file watching, and prompt fragment loading.
- `impl/embedded.go`: Read-only embedded defaults config store built from bundled `conf/**` resources.
- `impl/composite.go`: Multi-source config store that merges layered stores (`@DEFAULTS`, project, user, custom paths) with type-specific merge rules.
- `impl/mock.go`: Thread-safe config store test double with controllable data, errors, and timestamps.
