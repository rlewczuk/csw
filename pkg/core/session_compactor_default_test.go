package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSweSession_DefaultCompactor(t *testing.T) {
	testCases := []struct {
		name   string
		params *SweSessionParams
	}{
		{
			name:   "uses kimi compactor when params are nil",
			params: nil,
		},
		{
			name:   "uses kimi compactor when params are empty",
			params: &SweSessionParams{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			session := NewSweSession(testCase.params)
			require.NotNil(t, session)

			kimiCompactor, ok := session.compactor.(*KimiCompactor)
			require.True(t, ok)
			assert.Nil(t, kimiCompactor.model)
			assert.Equal(t, defaultKimiCompactorMessagesToKeep, kimiCompactor.nmessages)
		})
	}
}
