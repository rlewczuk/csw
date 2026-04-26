package models

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"os"

	"github.com/rlewczuk/csw/pkg/tool"
)

func (c *OllamaClient) emitRawStreamChunk(line string) {
	if c == nil || c.rawLLMCallback == nil {
		return
	}

	c.emitRawLLMLine("<<< CHUNK " + obfuscateBodyWithRegex(line))
}

// ChatStream sends a chat request and returns a standard Go iterator for streaming responses.
func (m *OllamaChatModel) ChatStream(ctx context.Context, messages []*ChatMessage, options *ChatOptions, tools []tool.ToolInfo) iter.Seq[*ChatMessage] {
	return func(yield func(*ChatMessage) bool) {
		if messages == nil || len(messages) == 0 {
			fmt.Fprintf(os.Stderr, "ERROR: OllamaChatModel.ChatStream() [ollama_client_stream.go]: messages cannot be empty\n")
			return
		}

		if m.model == "" {
			fmt.Fprintf(os.Stderr, "ERROR: OllamaChatModel.ChatStream() [ollama_client_stream.go]: model cannot be empty\n")
			return
		}

		effectiveOptions := options
		if effectiveOptions == nil {
			effectiveOptions = m.options
		}

		ollamaMessages := make([]OllamaMessage, len(messages))
		for i, msg := range messages {
			ollamaMessages[i] = convertToOllamaMessage(msg)
		}

		chatReq := OllamaChatRequest{
			Model:    m.model,
			Messages: ollamaMessages,
			Stream:   true,
			Tools:    convertToolsToOllama(tools),
		}

		if effectiveOptions != nil {
			chatReq.Options = &OllamaModelOptions{
				Temperature: float64(effectiveOptions.Temperature),
				TopP:        float64(effectiveOptions.TopP),
				TopK:        effectiveOptions.TopK,
			}
		}

		if m.client.config != nil && m.client.config.MaxTokens > 0 {
			if chatReq.Options == nil {
				chatReq.Options = &OllamaModelOptions{}
			}
			chatReq.Options.NumPredict = m.client.config.MaxTokens
		} else {
			if chatReq.Options == nil {
				chatReq.Options = &OllamaModelOptions{}
			}
			chatReq.Options.NumPredict = DefaultMaxTokens
		}

		url := m.client.baseURL + "/api/chat"

		body, err := json.Marshal(chatReq)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: OllamaChatModel.ChatStream() [ollama_client_stream.go]: failed to marshal request: %v\n", err)
			return
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: OllamaChatModel.ChatStream() [ollama_client_stream.go]: failed to create request: %v\n", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		setUserAgentHeader(req)
		m.client.applyConfiguredHeaders(req)
		m.client.applyConfiguredQueryParams(req)
		applyOptionsHeaders(req, effectiveOptions)

		logVerboseRequest(req, body, effectiveOptions != nil && effectiveOptions.Verbose)

		if effectiveOptions != nil && effectiveOptions.Logger != nil {
			logHTTPRequestWithObfuscation(effectiveOptions.Logger, req, chatReq)
		}

		resp, err := m.client.httpClient.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: OllamaChatModel.ChatStream() [ollama_client_stream.go]: HTTP request failed: %v\n", err)
			return
		}
		defer resp.Body.Close()

		logVerboseStreamResponseHeaders(resp, effectiveOptions != nil && effectiveOptions.Verbose)

		if err := m.client.checkStatusCode(resp); err != nil {
			bodyBytes, _ := io.ReadAll(resp.Body)
			if effectiveOptions != nil && effectiveOptions.Logger != nil {
				logHTTPErrorResponse(effectiveOptions.Logger, resp, bodyBytes)
			}
			fmt.Fprintf(os.Stderr, "ERROR: OllamaChatModel.ChatStream() [ollama_client_stream.go]: API error (status %d): %v\n", resp.StatusCode, err)
			return
		}

		decoder := json.NewDecoder(resp.Body)
		usage := TokenUsage{}
		contextLength := 0
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			var chatResp OllamaChatResponse
			if err := decoder.Decode(&chatResp); err != nil {
				if err == io.EOF {
					if effectiveOptions != nil && effectiveOptions.Verbose {
						fmt.Println("=== End of Streaming Response ===")
						fmt.Println()
					}
					return
				}
				if effectiveOptions != nil && effectiveOptions.Verbose {
					fmt.Println("=== End of Streaming Response ===")
					fmt.Println()
				}
				return
			}

			if effectiveOptions != nil && effectiveOptions.Verbose {
				jsonBytes, _ := json.Marshal(chatResp)
				fmt.Println(string(jsonBytes))
			}

			if effectiveOptions != nil && effectiveOptions.Logger != nil {
				logHTTPResponseChunk(effectiveOptions.Logger, chatResp)
			}

			if chatResp.PromptEvalCount > 0 {
				usage.InputTokens += chatResp.PromptEvalCount
				usage.InputNonCachedTokens = usage.InputTokens
			}
			if chatResp.EvalCount > 0 {
				usage.OutputTokens += chatResp.EvalCount
			}
			if usage.InputTokens > 0 || usage.OutputTokens > 0 {
				usage.TotalTokens = usage.InputTokens + usage.OutputTokens
				contextLength = usage.TotalTokens
			}

			result := convertFromOllamaMessage(chatResp.Message)

			if chatResp.Done {
				if usage.TotalTokens > 0 {
					usageMsg := &ChatMessage{Role: ChatRoleAssistant}
					usageCopy := usage
					usageMsg.TokenUsage = &usageCopy
					usageMsg.ContextLengthTokens = contextLength
					if !yield(usageMsg) {
						return
					}
				}
				if effectiveOptions != nil && effectiveOptions.Verbose {
					fmt.Println("=== End of Streaming Response ===")
					fmt.Println()
				}
				if len(result.Parts) > 0 && (result.GetText() != "" || len(result.GetToolCalls()) > 0) {
					if !yield(result) {
						return
					}
				}
				return
			}

			if len(result.Parts) > 0 && (result.GetText() != "" || len(result.GetToolCalls()) > 0) {
				if !yield(result) {
					return
				}
			}
		}
	}
}
