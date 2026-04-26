package lsp

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FindDefinition finds the definition of the symbol at the given location.
func (c *Client) FindDefinition(loc CursorLocation) ([]Location, error) {
	if !c.initialized.Load() {
		return nil, fmt.Errorf("Client.FindDefinition: client not initialized")
	}

	absPath, err := c.resolvePath(loc.Path)
	if err != nil {
		return nil, fmt.Errorf("Client.FindDefinition: failed to resolve path: %w", err)
	}

	uri := pathToURI(absPath)

	params := DefinitionParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position: Position{
				Line:      loc.Line,
				Character: loc.Col,
			},
		},
	}

	var result interface{}
	if err := c.sendRequest("textDocument/definition", params, &result); err != nil {
		return nil, fmt.Errorf("Client.FindDefinition: definition request failed: %w", err)
	}

	return c.parseLocationResult(result)
}

// FindReferences finds all references to the symbol at the given location.
func (c *Client) FindReferences(loc CursorLocation) ([]Location, error) {
	if !c.initialized.Load() {
		return nil, fmt.Errorf("Client.FindReferences: client not initialized")
	}

	absPath, err := c.resolvePath(loc.Path)
	if err != nil {
		return nil, fmt.Errorf("Client.FindReferences: failed to resolve path: %w", err)
	}

	uri := pathToURI(absPath)

	params := ReferenceParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position: Position{
				Line:      loc.Line,
				Character: loc.Col,
			},
		},
		Context: ReferenceContext{
			IncludeDeclaration: true,
		},
	}

	var result []Location
	if err := c.sendRequest("textDocument/references", params, &result); err != nil {
		return nil, fmt.Errorf("Client.FindReferences: references request failed: %w", err)
	}

	return result, nil
}

// Hover returns hover information for the symbol at the given location.
func (c *Client) Hover(loc CursorLocation) (string, string, error) {
	if !c.initialized.Load() {
		return "", "", fmt.Errorf("Client.Hover: client not initialized")
	}

	absPath, err := c.resolvePath(loc.Path)
	if err != nil {
		return "", "", fmt.Errorf("Client.Hover: failed to resolve path: %w", err)
	}

	uri := pathToURI(absPath)

	params := HoverParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position: Position{
				Line:      loc.Line,
				Character: loc.Col,
			},
		},
	}

	var result *Hover
	if err := c.sendRequest("textDocument/hover", params, &result); err != nil {
		return "", "", fmt.Errorf("Client.Hover: hover request failed: %w", err)
	}

	if result == nil || result.Contents == nil {
		return "", "", nil
	}

	return c.parseHoverContents(result.Contents)
}

// DocumentSymbols returns symbols for the given document.
func (c *Client) DocumentSymbols(path string) ([]DocumentSymbol, error) {
	if !c.initialized.Load() {
		return nil, fmt.Errorf("Client.DocumentSymbols: client not initialized")
	}

	absPath, err := c.resolvePath(path)
	if err != nil {
		return nil, fmt.Errorf("Client.DocumentSymbols: failed to resolve path: %w", err)
	}

	uri := pathToURI(absPath)

	params := DocumentSymbolParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	}

	var result []DocumentSymbol
	if err := c.sendRequest("textDocument/documentSymbol", params, &result); err != nil {
		return nil, fmt.Errorf("Client.DocumentSymbols: documentSymbol request failed: %w", err)
	}

	return result, nil
}

// WorkspaceSymbols returns symbols for the given query in the current workspace.
func (c *Client) WorkspaceSymbols(query string) ([]WorkspaceSymbol, error) {
	if !c.initialized.Load() {
		return nil, fmt.Errorf("Client.WorkspaceSymbols: client not initialized")
	}

	params := WorkspaceSymbolParams{
		Query: query,
	}

	var result []WorkspaceSymbol
	if err := c.sendRequest("workspace/symbol", params, &result); err != nil {
		return nil, fmt.Errorf("Client.WorkspaceSymbols: workspace/symbol request failed: %w", err)
	}

	return result, nil
}

// CallHierarchy prepares the call hierarchy for the symbol at the given location.
func (c *Client) CallHierarchy(loc CursorLocation) ([]CallHierarchyItem, error) {
	if !c.initialized.Load() {
		return nil, fmt.Errorf("Client.CallHierarchy: client not initialized")
	}

	absPath, err := c.resolvePath(loc.Path)
	if err != nil {
		return nil, fmt.Errorf("Client.CallHierarchy: failed to resolve path: %w", err)
	}

	uri := pathToURI(absPath)

	params := CallHierarchyPrepareParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position: Position{
				Line:      loc.Line,
				Character: loc.Col,
			},
		},
	}

	var result []CallHierarchyItem
	if err := c.sendRequest("textDocument/prepareCallHierarchy", params, &result); err != nil {
		return nil, fmt.Errorf("Client.CallHierarchy: prepareCallHierarchy request failed: %w", err)
	}

	return result, nil
}

// IncomingCalls returns incoming calls for the given call hierarchy item.
func (c *Client) IncomingCalls(item CallHierarchyItem) ([]CallHierarchyIncomingCall, error) {
	if !c.initialized.Load() {
		return nil, fmt.Errorf("Client.IncomingCalls: client not initialized")
	}

	params := CallHierarchyIncomingCallsParams{
		Item: item,
	}

	var result []CallHierarchyIncomingCall
	if err := c.sendRequest("callHierarchy/incomingCalls", params, &result); err != nil {
		return nil, fmt.Errorf("Client.IncomingCalls: incomingCalls request failed: %w", err)
	}

	return result, nil
}

// OutgoingCalls returns outgoing calls for the given call hierarchy item.
func (c *Client) OutgoingCalls(item CallHierarchyItem) ([]CallHierarchyOutgoingCall, error) {
	if !c.initialized.Load() {
		return nil, fmt.Errorf("Client.OutgoingCalls: client not initialized")
	}

	params := CallHierarchyOutgoingCallsParams{
		Item: item,
	}

	var result []CallHierarchyOutgoingCall
	if err := c.sendRequest("callHierarchy/outgoingCalls", params, &result); err != nil {
		return nil, fmt.Errorf("Client.OutgoingCalls: outgoingCalls request failed: %w", err)
	}

	return result, nil
}

// parseHoverContents parses the contents field from a Hover response.
func (c *Client) parseHoverContents(contents interface{}) (string, string, error) {
	data, err := json.Marshal(contents)
	if err != nil {
		return "", "", fmt.Errorf("Client.parseHoverContents: failed to marshal contents: %w", err)
	}

	var markupContent MarkupContent
	if err := json.Unmarshal(data, &markupContent); err == nil {
		if markupContent.Kind != "" && markupContent.Value != "" {
			return markupContent.Value, string(markupContent.Kind), nil
		}
	}

	var markedStrings []MarkedString
	if err := json.Unmarshal(data, &markedStrings); err == nil && len(markedStrings) > 0 {
		for _, ms := range markedStrings {
			if ms.Language == "" {
				return ms.Value, "markdown", nil
			}
		}
		return markedStrings[0].Value, "plaintext", nil
	}

	var markedString MarkedString
	if err := json.Unmarshal(data, &markedString); err == nil {
		if markedString.Value != "" {
			if markedString.Language == "" {
				return markedString.Value, "markdown", nil
			}
			return markedString.Value, "plaintext", nil
		}
	}

	var stringArray []string
	if err := json.Unmarshal(data, &stringArray); err == nil && len(stringArray) > 0 {
		var result strings.Builder
		for i, s := range stringArray {
			if i > 0 {
				result.WriteString("\n")
			}
			result.WriteString(s)
		}
		return result.String(), "markdown", nil
	}

	var str string
	if err := json.Unmarshal(data, &str); err == nil && str != "" {
		return str, "markdown", nil
	}

	return "", "", nil
}

// parseLocationResult parses the result from textDocument/definition.
func (c *Client) parseLocationResult(result interface{}) ([]Location, error) {
	if result == nil {
		return nil, nil
	}

	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("Client.parseLocationResult: failed to marshal result: %w", err)
	}

	var locations []Location
	if err := json.Unmarshal(data, &locations); err == nil {
		return locations, nil
	}

	var location Location
	if err := json.Unmarshal(data, &location); err == nil {
		return []Location{location}, nil
	}

	return nil, fmt.Errorf("Client.parseLocationResult: unsupported result format")
}
