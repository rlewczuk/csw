package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/stretchr/testify/assert"
)

func TestOutputProviderDetails(t *testing.T) {
	config := &conf.ModelProviderConfig{
		Name:        "test-provider",
		Type:        "openai",
		URL:         "https://api.openai.com/v1",
		AuthURL:     "https://auth.openai.com/oauth/authorize",
		Description: "Test provider",
		APIKey:      "sk-1234567890abcdef",
	}

	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputProviderDetails(config)
	w.Close()
	os.Stdout = oldStdout

	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "test-provider")
	assert.Contains(t, output, "openai")
	assert.Contains(t, output, "https://api.openai.com/v1")
	assert.Contains(t, output, "https://auth.openai.com/oauth/authorize")
	assert.Contains(t, output, "Test provider")
	assert.Contains(t, output, "sk-1****cdef")
}
