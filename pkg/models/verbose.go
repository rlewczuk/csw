package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// sensitiveHeaders are HTTP headers that should be obfuscated in logs
var sensitiveHeaders = []string{
	"authorization",
	"x-api-key",
	"api-key",
	"apikey",
}

// obfuscateHeaderValue masks sensitive header values, showing only first/last few characters
func obfuscateHeaderValue(value string) string {
	if len(value) <= 8 {
		return "***"
	}
	return value[:4] + "..." + value[len(value)-4:]
}

// obfuscateHeaders returns a copy of headers with sensitive values obfuscated
func obfuscateHeaders(headers http.Header) http.Header {
	obfuscated := make(http.Header)
	for key, values := range headers {
		lowerKey := strings.ToLower(key)
		isSensitive := false
		for _, sensitiveKey := range sensitiveHeaders {
			if lowerKey == sensitiveKey {
				isSensitive = true
				break
			}
		}

		if isSensitive {
			obfuscatedValues := make([]string, len(values))
			for i, value := range values {
				obfuscatedValues[i] = obfuscateHeaderValue(value)
			}
			obfuscated[key] = obfuscatedValues
		} else {
			obfuscated[key] = values
		}
	}
	return obfuscated
}

// obfuscateJSONBody attempts to obfuscate sensitive fields in JSON body
// It looks for fields like "api_key", "apiKey", "authorization", etc.
func obfuscateJSONBody(body []byte) string {
	// Try to parse as JSON
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		// If not valid JSON, try to obfuscate using regex
		return obfuscateBodyWithRegex(string(body))
	}

	// Obfuscate sensitive fields
	obfuscateSensitiveFields(data)

	// Marshal back to JSON
	obfuscated, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return string(body)
	}
	return string(obfuscated)
}

// obfuscateSensitiveFields recursively obfuscates sensitive fields in a map
func obfuscateSensitiveFields(data map[string]interface{}) {
	sensitiveFieldNames := []string{"api_key", "apikey", "apiKey", "authorization", "password", "secret", "token"}

	for key, value := range data {
		lowerKey := strings.ToLower(key)
		isSensitive := false
		for _, sensitiveField := range sensitiveFieldNames {
			if strings.Contains(lowerKey, strings.ToLower(sensitiveField)) {
				isSensitive = true
				break
			}
		}

		if isSensitive {
			if strValue, ok := value.(string); ok {
				data[key] = obfuscateHeaderValue(strValue)
			}
		} else if mapValue, ok := value.(map[string]interface{}); ok {
			// Recursively obfuscate nested maps
			obfuscateSensitiveFields(mapValue)
		} else if arrayValue, ok := value.([]interface{}); ok {
			// Handle arrays
			for _, item := range arrayValue {
				if mapItem, ok := item.(map[string]interface{}); ok {
					obfuscateSensitiveFields(mapItem)
				}
			}
		}
	}
}

// obfuscateBodyWithRegex uses regex to obfuscate sensitive patterns in non-JSON bodies
func obfuscateBodyWithRegex(body string) string {
	// Pattern to match Bearer tokens
	bearerRegex := regexp.MustCompile(`(Bearer\s+)([A-Za-z0-9\-._~+/]+=*)`)
	body = bearerRegex.ReplaceAllStringFunc(body, func(match string) string {
		parts := bearerRegex.FindStringSubmatch(match)
		if len(parts) == 3 {
			return parts[1] + obfuscateHeaderValue(parts[2])
		}
		return match
	})

	// Pattern to match key-value pairs with sensitive keys
	keyValueRegex := regexp.MustCompile(`("(?:api_key|apiKey|authorization|password|secret|token)":\s*")([^"]+)(")`)
	body = keyValueRegex.ReplaceAllStringFunc(body, func(match string) string {
		parts := keyValueRegex.FindStringSubmatch(match)
		if len(parts) == 4 {
			return parts[1] + obfuscateHeaderValue(parts[2]) + parts[3]
		}
		return match
	})

	return body
}

// logVerboseRequest prints the HTTP request details if verbose mode is enabled.
// It prints the method, URL, headers, and body with sensitive data obfuscated.
func logVerboseRequest(req *http.Request, body []byte, verbose bool) {
	if !verbose {
		return
	}

	fmt.Println("=== Request ===")
	fmt.Printf("%s %s\n", req.Method, req.URL.String())
	fmt.Println("\n=== Request Headers ===")
	obfuscatedHeaders := obfuscateHeaders(req.Header)
	for key, values := range obfuscatedHeaders {
		for _, value := range values {
			fmt.Printf("%s: %s\n", key, value)
		}
	}
	fmt.Println("\n=== Request Body ===")
	fmt.Println(obfuscateJSONBody(body))
	fmt.Println()
}

// logVerboseResponse prints the HTTP response details if verbose mode is enabled.
// It returns the response body bytes so they can be reused after logging.
// This function reads the response body, logs it, and returns the bytes for further processing.
func logVerboseResponse(resp *http.Response, verbose bool) ([]byte, error) {
	if !verbose {
		// If not verbose, just read and return the body
		return io.ReadAll(resp.Body)
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("logVerboseResponse: failed to read response body: %w", err)
	}

	// Print response headers and body with obfuscation
	fmt.Println("=== Response Headers ===")
	obfuscatedHeaders := obfuscateHeaders(resp.Header)
	for key, values := range obfuscatedHeaders {
		for _, value := range values {
			fmt.Printf("%s: %s\n", key, value)
		}
	}
	fmt.Println("\n=== Raw Response ===")
	fmt.Println(obfuscateJSONBody(bodyBytes))
	fmt.Println("=== End of Response ===")
	fmt.Println()

	return bodyBytes, nil
}

// logVerboseStreamResponseHeaders prints the HTTP streaming response headers if verbose mode is enabled.
func logVerboseStreamResponseHeaders(resp *http.Response, verbose bool) {
	if !verbose {
		return
	}

	fmt.Println("=== Response Headers ===")
	obfuscatedHeaders := obfuscateHeaders(resp.Header)
	for key, values := range obfuscatedHeaders {
		for _, value := range values {
			fmt.Printf("%s: %s\n", key, value)
		}
	}
	fmt.Println("\n=== Streaming Response ===")
}

// wrapResponseBodyForLogging wraps an http.Response to allow reading the body
// for logging while preserving it for further processing.
// This is useful when checkStatusCode needs to read the body, but we also want to log it.
func wrapResponseBodyForLogging(resp *http.Response, verbose bool) error {
	if !verbose {
		return nil
	}

	// Read the entire body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("wrapResponseBodyForLogging: failed to read response body: %w", err)
	}

	// Close the original body
	resp.Body.Close()

	// Replace with a new reader for the body bytes so it can be read again
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Print response headers and body with obfuscation
	fmt.Println("=== Response Headers ===")
	obfuscatedHeaders := obfuscateHeaders(resp.Header)
	for key, values := range obfuscatedHeaders {
		for _, value := range values {
			fmt.Printf("%s: %s\n", key, value)
		}
	}
	fmt.Println("\n=== Raw Response ===")
	fmt.Println(obfuscateJSONBody(bodyBytes))
	fmt.Println("=== End of Response ===")
	fmt.Println()

	return nil
}
