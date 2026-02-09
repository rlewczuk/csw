package models

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// logHTTPRequest logs an HTTP request with the given logger.
// It logs the request URL, method, headers, and body as structured fields.
// The body is logged as a JSON object (not a string) and should be the provider-specific request DTO.
// This function is a no-op if logger is nil.
func logHTTPRequest(logger *slog.Logger, req *http.Request, body interface{}) {
	if logger == nil {
		return
	}

	// Convert headers to map[string]string
	headers := make(map[string]string)
	for key, values := range req.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	logger.Info("llm_request",
		slog.String("url", req.URL.String()),
		slog.String("method", req.Method),
		slog.Any("headers", headers),
		slog.Any("body", body),
	)
}

// logHTTPResponse logs an HTTP response with the given logger.
// It logs the response status, headers, and body as structured fields.
// The body is logged as a JSON object (not a string) and should be the provider-specific response DTO.
// This function is a no-op if logger is nil.
func logHTTPResponse(logger *slog.Logger, resp *http.Response, body interface{}) {
	if logger == nil {
		return
	}

	// Convert headers to map[string]string
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	logger.Info("llm_response",
		slog.Int("status", resp.StatusCode),
		slog.Any("headers", headers),
		slog.Any("body", body),
	)
}

// logHTTPResponseChunk logs a streaming response chunk with the given logger.
// It logs the chunk body as a JSON object (not a string) and should be the provider-specific chunk DTO.
// This function is a no-op if logger is nil.
func logHTTPResponseChunk(logger *slog.Logger, chunk interface{}) {
	if logger == nil {
		return
	}

	logger.Info("llm_response",
		slog.Any("body", chunk),
	)
}

// obfuscateSensitiveHeaders returns a copy of headers with sensitive values obfuscated.
// This is used for security when logging headers that may contain API keys.
func obfuscateSensitiveHeadersForLogging(headers http.Header) map[string]string {
	sensitiveHeaders := []string{
		"authorization",
		"x-api-key",
		"api-key",
		"apikey",
	}

	result := make(map[string]string)
	for key, values := range headers {
		if len(values) == 0 {
			continue
		}

		value := values[0]
		lowerKey := http.CanonicalHeaderKey(key)
		for _, sensitive := range sensitiveHeaders {
			if lowerKey == http.CanonicalHeaderKey(sensitive) {
				if len(value) > 8 {
					value = value[:4] + "..." + value[len(value)-4:]
				} else {
					value = "***"
				}
				break
			}
		}
		result[key] = value
	}
	return result
}

// logHTTPRequestWithObfuscation logs an HTTP request with sensitive header values obfuscated.
// This is a convenience function that combines header obfuscation with request logging.
func logHTTPRequestWithObfuscation(logger *slog.Logger, req *http.Request, body interface{}) {
	if logger == nil {
		return
	}

	headers := obfuscateSensitiveHeadersForLogging(req.Header)

	logger.Info("llm_request",
		slog.String("url", req.URL.String()),
		slog.String("method", req.Method),
		slog.Any("headers", headers),
		slog.Any("body", body),
	)
}

// logHTTPResponseWithObfuscation logs an HTTP response with sensitive header values obfuscated.
// This is a convenience function that combines header obfuscation with response logging.
func logHTTPResponseWithObfuscation(logger *slog.Logger, resp *http.Response, body interface{}) {
	if logger == nil {
		return
	}

	headers := obfuscateSensitiveHeadersForLogging(resp.Header)

	logger.Info("llm_response",
		slog.Int("status", resp.StatusCode),
		slog.Any("headers", headers),
		slog.Any("body", body),
	)
}

// parseJSONBody parses a JSON byte slice into a map[string]interface{} for logging.
// If parsing fails, it returns nil.
func parseJSONBody(body []byte) interface{} {
	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil
	}
	return result
}

// logHTTPErrorResponse logs an HTTP error response with the given logger.
// It logs the response status, headers, and raw body as structured fields.
// This function is a no-op if logger is nil.
// The body is logged as a raw string (not parsed JSON) to preserve error details.
func logHTTPErrorResponse(logger *slog.Logger, resp *http.Response, body []byte) {
	if logger == nil {
		return
	}

	headers := obfuscateSensitiveHeadersForLogging(resp.Header)

	logger.Info("llm_response_error",
		slog.Int("status", resp.StatusCode),
		slog.Any("headers", headers),
		slog.String("body", string(body)),
	)
}
