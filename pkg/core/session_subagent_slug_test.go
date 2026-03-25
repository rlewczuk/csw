package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSweSessionReserveUniqueSubAgentSlug(t *testing.T) {
	t.Run("returns error for nil session", func(t *testing.T) {
		var session *SweSession

		_, err := session.ReserveUniqueSubAgentSlug("child")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "session is nil")
	})

	t.Run("returns error for empty slug", func(t *testing.T) {
		session := NewSweSession(&SweSessionParams{})

		_, err := session.ReserveUniqueSubAgentSlug("   ")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "slug cannot be empty")
	})

	t.Run("reserves initial slug and appends numeric suffix for duplicates", func(t *testing.T) {
		session := NewSweSession(&SweSessionParams{})

		slug1, err := session.ReserveUniqueSubAgentSlug("child-task")
		require.NoError(t, err)
		assert.Equal(t, "child-task", slug1)

		slug2, err := session.ReserveUniqueSubAgentSlug("child-task")
		require.NoError(t, err)
		assert.Equal(t, "child-task-2", slug2)

		slug3, err := session.ReserveUniqueSubAgentSlug("child-task")
		require.NoError(t, err)
		assert.Equal(t, "child-task-3", slug3)
	})

	t.Run("reuses gaps in existing numeric suffixes", func(t *testing.T) {
		session := NewSweSession(&SweSessionParams{UsedSubAgentSlugs: map[string]struct{}{
			"child":   {},
			"child-2": {},
			"child-4": {},
		}})

		slug, err := session.ReserveUniqueSubAgentSlug("child")
		require.NoError(t, err)
		assert.Equal(t, "child-3", slug)
	})
}
