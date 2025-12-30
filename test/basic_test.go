package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAgentCoreInitialization(t *testing.T) {
	t.Run("basic initialization", func(t *testing.T) {
		system := NewTestSystem()
		assert.NotNil(t, system)

		project := system.NewProject("py_simple1")
		assert.NotNil(t, project)

		session := project.NewSession()
		assert.NotNil(t, session)

		err := session.SetRole("developer")
		assert.NoError(t, err)

		err = session.Prompt("Implement Hello World program in Python")
		assert.NoError(t, err)

	})
}
