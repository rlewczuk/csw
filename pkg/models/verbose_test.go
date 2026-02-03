package models

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestObfuscateHeaderValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "short value",
			input:    "abc",
			expected: "***",
		},
		{
			name:     "medium value",
			input:    "abcdefgh",
			expected: "***",
		},
		{
			name:     "long value",
			input:    "sk-1234567890abcdefghijklmnopqrstuvwxyz",
			expected: "sk-1...wxyz",
		},
		{
			name:     "bearer token",
			input:    "Bearer sk-1234567890abcdefghijklmnopqrstuvwxyz",
			expected: "Bear...wxyz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := obfuscateHeaderValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestObfuscateHeaders(t *testing.T) {
	tests := []struct {
		name     string
		headers  http.Header
		validate func(t *testing.T, result http.Header)
	}{
		{
			name: "obfuscate Authorization header",
			headers: http.Header{
				"Authorization": []string{"Bearer sk-1234567890abcdefghijklmnopqrstuvwxyz"},
				"Content-Type":  []string{"application/json"},
			},
			validate: func(t *testing.T, result http.Header) {
				assert.Contains(t, result.Get("Authorization"), "...")
				assert.NotContains(t, result.Get("Authorization"), "1234567890abcdefghijklmnopqrstu")
				assert.Equal(t, "application/json", result.Get("Content-Type"))
			},
		},
		{
			name: "obfuscate x-api-key header",
			headers: http.Header{
				"X-Api-Key":    []string{"sk-proj-1234567890abcdefghijklmnopqrstuvwxyz"},
				"Content-Type": []string{"application/json"},
			},
			validate: func(t *testing.T, result http.Header) {
				assert.Contains(t, result.Get("X-Api-Key"), "...")
				assert.NotContains(t, result.Get("X-Api-Key"), "1234567890abcdefghijklmnopqrstu")
				assert.Equal(t, "application/json", result.Get("Content-Type"))
			},
		},
		{
			name: "preserve non-sensitive headers",
			headers: http.Header{
				"Content-Type":   []string{"application/json"},
				"Content-Length": []string{"1234"},
				"User-Agent":     []string{"Go-http-client/1.1"},
			},
			validate: func(t *testing.T, result http.Header) {
				assert.Equal(t, "application/json", result.Get("Content-Type"))
				assert.Equal(t, "1234", result.Get("Content-Length"))
				assert.Equal(t, "Go-http-client/1.1", result.Get("User-Agent"))
			},
		},
		{
			name: "multiple sensitive headers",
			headers: http.Header{
				"Authorization": []string{"Bearer token123456789"},
				"X-Api-Key":     []string{"key123456789"},
				"Content-Type":  []string{"application/json"},
			},
			validate: func(t *testing.T, result http.Header) {
				assert.Contains(t, result.Get("Authorization"), "...")
				assert.Contains(t, result.Get("X-Api-Key"), "...")
				assert.Equal(t, "application/json", result.Get("Content-Type"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := obfuscateHeaders(tt.headers)
			tt.validate(t, result)
		})
	}
}

func TestObfuscateJSONBody(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		validate func(t *testing.T, result string)
	}{
		{
			name:  "obfuscate api_key in JSON",
			input: []byte(`{"api_key":"sk-1234567890abcdefghijklmnopqrstuvwxyz","model":"gpt-4"}`),
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "...")
				assert.NotContains(t, result, "1234567890abcdefghijklmnopqrstu")
				assert.Contains(t, result, "gpt-4")
			},
		},
		{
			name:  "obfuscate apiKey in JSON",
			input: []byte(`{"apiKey":"sk-1234567890abcdefghijklmnopqrstuvwxyz","model":"gpt-4"}`),
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "...")
				assert.NotContains(t, result, "1234567890abcdefghijklmnopqrstu")
				assert.Contains(t, result, "gpt-4")
			},
		},
		{
			name:  "obfuscate authorization in JSON",
			input: []byte(`{"authorization":"Bearer sk-1234567890abcdefghijklmnopqrstuvwxyz","model":"gpt-4"}`),
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "...")
				assert.NotContains(t, result, "1234567890abcdefghijklmnopqrstu")
				assert.Contains(t, result, "gpt-4")
			},
		},
		{
			name:  "obfuscate nested api_key in JSON",
			input: []byte(`{"config":{"api_key":"sk-1234567890abcdefghijklmnopqrstuvwxyz"},"model":"gpt-4"}`),
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "...")
				assert.NotContains(t, result, "1234567890abcdefghijklmnopqrstu")
				assert.Contains(t, result, "gpt-4")
			},
		},
		{
			name:  "preserve non-sensitive JSON fields",
			input: []byte(`{"model":"gpt-4","temperature":0.7,"max_tokens":1000}`),
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "gpt-4")
				assert.Contains(t, result, "0.7")
				assert.Contains(t, result, "1000")
			},
		},
		{
			name:  "obfuscate password field",
			input: []byte(`{"username":"user123","password":"secretpass123456","email":"user@example.com"}`),
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "user123")
				assert.Contains(t, result, "user@example.com")
				assert.Contains(t, result, "...")
				assert.NotContains(t, result, "secretpass123456")
			},
		},
		{
			name:  "handle invalid JSON with regex fallback",
			input: []byte(`not valid json but has "api_key":"sk-1234567890abcdefghijklmnopqrstuvwxyz" in it`),
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "...")
				assert.NotContains(t, result, "1234567890abcdefghijklmnopqrstu")
			},
		},
		{
			name:  "obfuscate array with sensitive data",
			input: []byte(`{"items":[{"api_key":"sk-1234567890abcdefghijklmnopqrstuvwxyz"},{"api_key":"sk-9876543210zyxwvutsrqponmlkjihgfedcba"}]}`),
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "...")
				assert.NotContains(t, result, "1234567890abcdefghijklmnopqrstu")
				assert.NotContains(t, result, "9876543210zyxwvutsrqponmlkjihgf")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := obfuscateJSONBody(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestObfuscateBodyWithRegex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, result string)
	}{
		{
			name:  "obfuscate Bearer token",
			input: `Authorization: Bearer sk-1234567890abcdefghijklmnopqrstuvwxyz`,
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "Bearer")
				assert.Contains(t, result, "...")
				assert.NotContains(t, result, "1234567890abcdefghijklmnopqrstu")
			},
		},
		{
			name:  "obfuscate api_key in key-value format",
			input: `"api_key": "sk-1234567890abcdefghijklmnopqrstuvwxyz"`,
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "api_key")
				assert.Contains(t, result, "...")
				assert.NotContains(t, result, "1234567890abcdefghijklmnopqrstu")
			},
		},
		{
			name:  "preserve non-sensitive data",
			input: `"model": "gpt-4", "temperature": 0.7`,
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "gpt-4")
				assert.Contains(t, result, "0.7")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := obfuscateBodyWithRegex(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestLogVerboseRequest(t *testing.T) {
	tests := []struct {
		name    string
		verbose bool
		setup   func() (*http.Request, []byte)
		verify  func(t *testing.T)
	}{
		{
			name:    "verbose disabled - no logging",
			verbose: false,
			setup: func() (*http.Request, []byte) {
				req, _ := http.NewRequest("POST", "https://api.example.com/v1/chat", nil)
				req.Header.Set("Authorization", "Bearer secret-key-123456789")
				body := []byte(`{"api_key":"sk-123456789"}`)
				return req, body
			},
			verify: func(t *testing.T) {
				// When verbose is false, nothing should be logged
				// This test just verifies the function doesn't panic
			},
		},
		{
			name:    "verbose enabled - logging with obfuscation",
			verbose: true,
			setup: func() (*http.Request, []byte) {
				req, _ := http.NewRequest("POST", "https://api.example.com/v1/chat", nil)
				req.Header.Set("Authorization", "Bearer sk-1234567890abcdefghijklmnopqrstuvwxyz")
				req.Header.Set("Content-Type", "application/json")
				body := []byte(`{"api_key":"sk-1234567890abcdefghijklmnopqrstuvwxyz","model":"gpt-4"}`)
				return req, body
			},
			verify: func(t *testing.T) {
				// This test verifies the function doesn't panic and completes
				// Actual output verification would require capturing stdout
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, body := tt.setup()
			// Should not panic
			logVerboseRequest(req, body, tt.verbose)
			tt.verify(t)
		})
	}
}

func TestObfuscateSensitiveFields(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		validate func(t *testing.T, result map[string]interface{})
	}{
		{
			name: "obfuscate simple api_key",
			input: map[string]interface{}{
				"api_key": "sk-1234567890abcdefghijklmnopqrstuvwxyz",
				"model":   "gpt-4",
			},
			validate: func(t *testing.T, result map[string]interface{}) {
				apiKey := result["api_key"].(string)
				assert.Contains(t, apiKey, "...")
				assert.NotContains(t, apiKey, "1234567890abcdefghijklmnopqrstu")
				assert.Equal(t, "gpt-4", result["model"])
			},
		},
		{
			name: "obfuscate nested sensitive fields",
			input: map[string]interface{}{
				"config": map[string]interface{}{
					"api_key": "sk-1234567890abcdefghijklmnopqrstuvwxyz",
					"timeout": 30,
				},
				"model": "gpt-4",
			},
			validate: func(t *testing.T, result map[string]interface{}) {
				config := result["config"].(map[string]interface{})
				apiKey := config["api_key"].(string)
				assert.Contains(t, apiKey, "...")
				assert.NotContains(t, apiKey, "1234567890abcdefghijklmnopqrstu")
				assert.Equal(t, "gpt-4", result["model"])
			},
		},
		{
			name: "obfuscate in arrays",
			input: map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{
						"api_key": "sk-1234567890abcdefghijklmnopqrstuvwxyz",
					},
					map[string]interface{}{
						"password": "secretpass123456",
					},
				},
			},
			validate: func(t *testing.T, result map[string]interface{}) {
				items := result["items"].([]interface{})
				item0 := items[0].(map[string]interface{})
				apiKey := item0["api_key"].(string)
				assert.Contains(t, apiKey, "...")

				item1 := items[1].(map[string]interface{})
				password := item1["password"].(string)
				assert.Contains(t, password, "...")
			},
		},
		{
			name: "case insensitive matching",
			input: map[string]interface{}{
				"API_KEY": "sk-1234567890abcdefghijklmnopqrstuvwxyz",
				"ApiKey":  "sk-9876543210zyxwvutsrqponmlkjihgfedcba",
			},
			validate: func(t *testing.T, result map[string]interface{}) {
				apiKey1 := result["API_KEY"].(string)
				assert.Contains(t, apiKey1, "...")

				apiKey2 := result["ApiKey"].(string)
				assert.Contains(t, apiKey2, "...")
			},
		},
		{
			name: "preserve non-string sensitive fields",
			input: map[string]interface{}{
				"api_key": 12345,
				"model":   "gpt-4",
			},
			validate: func(t *testing.T, result map[string]interface{}) {
				// Non-string api_key should be preserved
				assert.Equal(t, 12345, result["api_key"])
				assert.Equal(t, "gpt-4", result["model"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of input to avoid mutation affecting the test
			obfuscateSensitiveFields(tt.input)
			tt.validate(t, tt.input)
		})
	}
}

func TestSensitiveHeadersDetection(t *testing.T) {
	tests := []struct {
		name            string
		headerKey       string
		shouldObfuscate bool
	}{
		{"Authorization", "Authorization", true},
		{"authorization", "authorization", true},
		{"AUTHORIZATION", "AUTHORIZATION", true},
		{"X-Api-Key", "X-Api-Key", true},
		{"x-api-key", "x-api-key", true},
		{"Api-Key", "Api-Key", true},
		{"ApiKey", "ApiKey", true},
		{"Content-Type", "Content-Type", false},
		{"User-Agent", "User-Agent", false},
		{"Accept", "Accept", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := http.Header{}
			headers.Set(tt.headerKey, "test-value-1234567890")
			result := obfuscateHeaders(headers)

			// Get the actual canonical key that was used
			var value string
			for k, v := range result {
				if strings.EqualFold(k, tt.headerKey) {
					if len(v) > 0 {
						value = v[0]
					}
					break
				}
			}

			if tt.shouldObfuscate {
				assert.Contains(t, value, "...", "Expected %s to be obfuscated", tt.headerKey)
				assert.NotContains(t, strings.ToLower(value), "1234567890", "Expected %s value to not contain full key", tt.headerKey)
			} else {
				assert.Equal(t, "test-value-1234567890", value, "Expected %s to not be obfuscated", tt.headerKey)
			}
		})
	}
}
