package models

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

// logVerboseRequest prints the HTTP request details if verbose mode is enabled.
// It prints the method, URL, headers, and body.
func logVerboseRequest(req *http.Request, body []byte, verbose bool) {
	if !verbose {
		return
	}

	fmt.Println("=== Request ===")
	fmt.Printf("%s %s\n", req.Method, req.URL.String())
	fmt.Println("\n=== Request Headers ===")
	for key, values := range req.Header {
		for _, value := range values {
			fmt.Printf("%s: %s\n", key, value)
		}
	}
	fmt.Println("\n=== Request Body ===")
	fmt.Println(string(body))
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
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Print response headers and body
	fmt.Println("=== Response Headers ===")
	for key, values := range resp.Header {
		for _, value := range values {
			fmt.Printf("%s: %s\n", key, value)
		}
	}
	fmt.Println("\n=== Raw Response ===")
	fmt.Println(string(bodyBytes))
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
	for key, values := range resp.Header {
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
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Close the original body
	resp.Body.Close()

	// Replace with a new reader for the body bytes so it can be read again
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Print response headers and body
	fmt.Println("=== Response Headers ===")
	for key, values := range resp.Header {
		for _, value := range values {
			fmt.Printf("%s: %s\n", key, value)
		}
	}
	fmt.Println("\n=== Raw Response ===")
	fmt.Println(string(bodyBytes))
	fmt.Println("=== End of Response ===")
	fmt.Println()

	return nil
}
