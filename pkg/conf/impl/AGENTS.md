# Package `pkg/conf/impl` Overview

Package `pkg/conf/impl` provides concrete configuration store implementations for `pkg/conf` interfaces. It includes composite merging across multiple sources, embedded read-only defaults, local filesystem-based stores with auto-reload, and a test mock.

## Important files

* `composite.go` - Composite merged config store
* `embedded.go` - Embedded read-only config store
* `local.go` - Local filesystem config store
* `local_yaml_test.go` - YAML-specific local config store tests extracted from `local_test.go`
* `mock.go` - Mock config store for tests

## Important public API objects

* `CompositeConfigStore` - Multi-source merged configuration store
* `EmbeddedConfigStore` - Embedded read-only configuration store
* `LocalConfigStore` - Local filesystem configuration store
* `MockConfigStore` - Mock configuration store for tests
* `NewCompositeConfigStore()` - Creates composite config store
* `NewEmbeddedConfigStore()` - Creates embedded config store
* `NewLocalConfigStore()` - Creates local config store
* `NewMockConfigStore()` - Creates mock config store
