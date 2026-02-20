package presenter

import (
	"testing"

	coretestfixture "github.com/rlewczuk/csw/pkg/core/testfixture"
)

func newPresenterFixture(t *testing.T) *coretestfixture.SweSystemFixture {
	return coretestfixture.NewSweSystemFixture(t,
		coretestfixture.WithPromptGenerator(coretestfixture.NewStaticPromptGenerator("You are skilled software developer.")),
	)
}
