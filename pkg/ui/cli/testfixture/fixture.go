// Package testfixture provides CLI integration test fixtures.
package testfixture

import (
	"testing"

	coretestfixture "github.com/rlewczuk/csw/pkg/core/testfixture"
)

// CliFixture provides shared setup for CLI integration tests.
type CliFixture struct {
	*coretestfixture.SweSystemFixture
}

// NewCliFixture creates a CLI fixture with a SweSystem.
func NewCliFixture(t *testing.T, opts ...coretestfixture.SweSystemFixtureOption) *CliFixture {
	return &CliFixture{
		SweSystemFixture: coretestfixture.NewSweSystemFixture(t, opts...),
	}
}
