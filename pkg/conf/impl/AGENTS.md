# Package `pkg/conf/impl` Overview

Package `pkg/conf/impl` implements configuration store backends.

## Important files

* `composite.go` - Composite merged config store
* `embedded.go` - Embedded read-only config store
* `local.go` - Local filesystem config store
* `mock.go` - Mock config store for tests

## Important public API objects

* `CompositeConfigStore` - Multi-source merged configuration store
* `EmbeddedConfigStore` - Embedded configuration store
* `LocalConfigStore` - Local configuration store
* `MockConfigStore` - Mock configuration store
* `NewCompositeConfigStore()` - Creates composite config store
* `NewEmbeddedConfigStore()` - Creates embedded config store
* `NewLocalConfigStore()` - Creates local config store
* `NewMockConfigStore()` - Creates mock config store
