// Package testfixture provides CLI integration test fixtures.
package testfixture

import (
	"testing"

	coretestfixture "github.com/rlewczuk/csw/pkg/core/testfixture"
	presentertestfixture "github.com/rlewczuk/csw/pkg/presenter/testfixture"
)

// CliFixture provides shared setup for CLI integration tests.
type CliFixture struct {
	*presentertestfixture.PresenterFixture
}

// NewCliFixture creates a CLI fixture with a SweSystem.
func NewCliFixture(t *testing.T, opts ...coretestfixture.SweSystemFixtureOption) *CliFixture {
	return &CliFixture{
		PresenterFixture: presentertestfixture.NewPresenterFixture(t, opts...),
	}
}
