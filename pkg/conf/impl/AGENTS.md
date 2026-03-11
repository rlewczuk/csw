# Package `pkg/conf/impl` Overview

Package `pkg/conf/impl` contains configuration store implementations for the conf package. It provides local filesystem-based, embedded, composite (multi-source), and mock config stores.

## Important files

* `composite.go` - Multi-source config store merging layered sources
* `embedded.go` - Read-only embedded defaults config store
* `local.go` - Filesystem-backed config store with file watching
* `mock.go` - Test double config store with controllable data

## Important public API objects

* `CompositeConfigStore` - Merges configurations from multiple sources
* `EmbeddedConfigStore` - Read-only access to embedded configuration
* `LocalConfigStore` - Filesystem-based config with auto-reload
* `MockConfigStore` - Test double for conf.ConfigStore interface
* `NewCompositeConfigStore()` - Creates composite store from config path
* `NewEmbeddedConfigStore()` - Creates embedded config store
* `NewLocalConfigStore()` - Creates local filesystem config store
* `NewMockConfigStore()` - Creates mock config store for testing
