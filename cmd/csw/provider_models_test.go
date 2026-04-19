package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOutputModelsList(t *testing.T) {
	modelsList := []modelEntry{
		{Provider: "provider1", Model: "model1"},
		{Provider: "provider1", Model: "model2"},
		{Provider: "provider2", Model: "model3"},
	}

	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputModelsList(modelsList)
	w.Close()
	os.Stdout = oldStdout

	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "PROVIDER")
	assert.Contains(t, output, "MODEL")
	assert.Contains(t, output, "provider1")
	assert.Contains(t, output, "provider2")
	assert.Contains(t, output, "model1")
	assert.Contains(t, output, "model2")
	assert.Contains(t, output, "model3")
}

func TestOutputModelsListEmpty(t *testing.T) {
	var modelsList []modelEntry

	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputModelsList(modelsList)
	w.Close()
	os.Stdout = oldStdout

	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "PROVIDER")
	assert.Contains(t, output, "MODEL")
}

func TestOutputModelsListJSON(t *testing.T) {
	modelsList := []modelEntry{
		{Provider: "provider1", Model: "model1"},
		{Provider: "provider2", Model: "model2"},
	}

	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputJSON(modelsList)
	w.Close()
	os.Stdout = oldStdout

	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, `"provider"`)
	assert.Contains(t, output, `"model"`)
	assert.Contains(t, output, `"provider1"`)
	assert.Contains(t, output, `"model1"`)
	assert.Contains(t, output, `"provider2"`)
	assert.Contains(t, output, `"model2"`)

	var decoded []modelEntry
	err = json.Unmarshal([]byte(output), &decoded)
	assert.NoError(t, err)
	assert.Len(t, decoded, 2)
	assert.Equal(t, "provider1", decoded[0].Provider)
	assert.Equal(t, "model1", decoded[0].Model)
}
