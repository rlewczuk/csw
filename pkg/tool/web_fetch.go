package tool

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/shared/godown"
)

const defaultWebFetchTimeout = 30 * time.Second

// WebFetchTool implements the webFetch tool for retrieving content from URLs.
type WebFetchTool struct {
	httpClient *http.Client
}

// NewWebFetchTool creates a new WebFetchTool instance.
func NewWebFetchTool(httpClient *http.Client) *WebFetchTool {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultWebFetchTimeout}
	}

	return &WebFetchTool{httpClient: httpClient}
}

// Execute executes the tool with the given arguments and returns the response.
func (t *WebFetchTool) Execute(args *ToolCall) *ToolResponse {
	rawURL, ok := args.Arguments.StringOK("url")
	if !ok || strings.TrimSpace(rawURL) == "" {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("WebFetchTool.Execute() [web_fetch.go]: missing required argument: url"),
			Done:  true,
		}
	}

	format, ok := args.Arguments.StringOK("format")
	if !ok {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("WebFetchTool.Execute() [web_fetch.go]: missing required argument: format"),
			Done:  true,
		}
	}

	if format != "markdown" && format != "raw" {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("WebFetchTool.Execute() [web_fetch.go]: invalid format: %s", format),
			Done:  true,
		}
	}

	timeout := defaultWebFetchTimeout
	if timeoutSeconds, hasTimeout := args.Arguments.IntOK("timeout"); hasTimeout {
		if timeoutSeconds <= 0 {
			return &ToolResponse{
				Call:  args,
				Error: fmt.Errorf("WebFetchTool.Execute() [web_fetch.go]: timeout must be positive, got %d", timeoutSeconds),
				Done:  true,
			}
		}
		timeout = time.Duration(timeoutSeconds) * time.Second
	}

	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("WebFetchTool.Execute() [web_fetch.go]: invalid url %q: %w", rawURL, err),
			Done:  true,
		}
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("WebFetchTool.Execute() [web_fetch.go]: unsupported url scheme: %s", parsedURL.Scheme),
			Done:  true,
		}
	}

	client := *t.httpClient
	client.Timeout = timeout

	response, err := client.Get(rawURL)
	if err != nil {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("WebFetchTool.Execute() [web_fetch.go]: failed to fetch url %q: %w", rawURL, err),
			Done:  true,
		}
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("WebFetchTool.Execute() [web_fetch.go]: request failed with status %d", response.StatusCode),
			Done:  true,
		}
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("WebFetchTool.Execute() [web_fetch.go]: failed to read response body: %w", err),
			Done:  true,
		}
	}

	content := string(body)
	contentType := response.Header.Get("Content-Type")

	if format == "markdown" {
		if !isHTMLContent(contentType, content) {
			return &ToolResponse{
				Call:  args,
				Error: fmt.Errorf("WebFetchTool.Execute() [web_fetch.go]: markdown conversion currently supports only html content"),
				Done:  true,
			}
		}

		converted, convertErr := godown.CovertStr(content, nil)
		if convertErr != nil {
			return &ToolResponse{
				Call:  args,
				Error: fmt.Errorf("WebFetchTool.Execute() [web_fetch.go]: failed to convert html to markdown: %w", convertErr),
				Done:  true,
			}
		}
		content = converted
	}

	var result ToolValue
	result.Set("url", rawURL)
	result.Set("format", format)
	result.Set("statusCode", response.StatusCode)
	result.Set("contentType", contentType)
	result.Set("content", content)

	return &ToolResponse{
		Call:   args,
		Result: result,
		Done:   true,
	}
}

// Render returns a string representation of the tool call.
func (t *WebFetchTool) Render(call *ToolCall) (string, string, map[string]string) {
	urlValue, _ := call.Arguments.StringOK("url")
	oneLiner := truncateString("fetch "+urlValue, 128)
	full := oneLiner + "\n\n"

	if content, ok := call.Arguments.StringOK("content"); ok && content != "" {
		full += content
	}

	return oneLiner, full, make(map[string]string)
}

func isHTMLContent(contentType string, content string) bool {
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		return true
	}

	trimmed := strings.TrimSpace(strings.ToLower(content))
	return strings.HasPrefix(trimmed, "<!doctype html") || strings.HasPrefix(trimmed, "<html")
}
