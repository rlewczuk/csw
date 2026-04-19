package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/stretchr/testify/assert"
)

func TestOutputProviderList(t *testing.T) {
	configs := map[string]*conf.ModelProviderConfig{
		"provider1": {
			Name:        "provider1",
			Type:        "openai",
			Description: "Provider 1",
		},
		"provider2": {
			Name:        "provider2",
			Type:        "ollama",
			Description: "Provider 2",
		},
	}

	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputProviderList(configs)
	w.Close()
	os.Stdout = oldStdout

	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "provider1")
	assert.Contains(t, output, "provider2")
	assert.Contains(t, output, "openai")
	assert.Contains(t, output, "ollama")
}
