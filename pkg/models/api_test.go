package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestChatOptionsThinkingField tests that ChatOptions has the Thinking field.
func TestChatOptionsThinkingField(t *testing.T) {
	opts := &ChatOptions{
		Thinking: "high",
	}
	assert.Equal(t, "high", opts.Thinking)

	opts2 := &ChatOptions{
		Thinking: "true",
	}
	assert.Equal(t, "true", opts2.Thinking)

	opts3 := &ChatOptions{}
	assert.Equal(t, "", opts3.Thinking)
}
