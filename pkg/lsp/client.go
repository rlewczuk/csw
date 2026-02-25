package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// Client implements the LSP interface for communicating with a language server.
type Client struct {
	serverPath string
	workingDir string

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	// request/response handling
	nextID    atomic.Int64
	pending   map[int64]chan *response
	pendingMu sync.RWMutex

	// diagnostics storage
	diagnostics   map[string][]Diagnostic // keyed by URI
	diagnosticsMu sync.RWMutex

	// initialization
	initialized atomic.Bool
	initErr     error
	initMu      sync.Mutex

	// shutdown
	ctx    context.Context
	cancel context.CancelFunc

	// open documents tracking
	openDocs   map[string]int // URI -> version
	openDocsMu sync.RWMutex
}

type response struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *responseError  `json:"error,omitempty"`
}

type responseError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type notification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// NewClient creates a new LSP client.
// serverPath is the path to the language server binary.
// workingDir is the project working directory.
func NewClient(serverPath, workingDir string) (*Client, error) {
	if serverPath == "" {
		return nil, fmt.Errorf("NewClient: serverPath is empty")
	}
	if workingDir == "" {
		return nil, fmt.Errorf("NewClient: workingDir is empty")
	}

	absWorkingDir, err := filepath.Abs(workingDir)
	if err != nil {
		return nil, fmt.Errorf("NewClient: failed to get absolute path for working directory: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &Client{
		serverPath:  serverPath,
		workingDir:  absWorkingDir,
		pending:     make(map[int64]chan *response),
		diagnostics: make(map[string][]Diagnostic),
		openDocs:    make(map[string]int),
		ctx:         ctx,
		cancel:      cancel,
	}

	c.nextID.Store(1)

	return c, nil
}

// Init initializes the LSP client. If sync is true, it will block until the server is ready.
func (c *Client) Init(sync bool) error {
	c.initMu.Lock()
	defer c.initMu.Unlock()

	if c.initialized.Load() {
		return c.initErr
	}

	// Start the language server process
	c.cmd = exec.CommandContext(c.ctx, c.serverPath)
	c.cmd.Dir = c.workingDir

	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		c.initErr = fmt.Errorf("Client.Init: failed to create stdin pipe: %w", err)
		return c.initErr
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		c.initErr = fmt.Errorf("Client.Init: failed to create stdout pipe: %w", err)
		return c.initErr
	}

	c.stderr, err = c.cmd.StderrPipe()
	if err != nil {
		c.initErr = fmt.Errorf("Client.Init: failed to create stderr pipe: %w", err)
		return c.initErr
	}

	if err := c.cmd.Start(); err != nil {
		c.initErr = fmt.Errorf("Client.Init: failed to start language server: %w", err)
		return c.initErr
	}

	// Start reading responses in the background
	go c.readLoop()
	go c.readStderr()

	// Send initialize request
	params := InitializeParams{
		ProcessID: intPtr(os.Getpid()),
		RootURI:   docURIPtr(pathToURI(c.workingDir)),
		Capabilities: ClientCapabilities{
			TextDocument: &TextDocumentClientCapabilities{
				Synchronization: &TextDocumentSyncClientCapabilities{
					DynamicRegistration: false,
					WillSave:            false,
					WillSaveWaitUntil:   false,
					DidSave:             false,
				},
				PublishDiagnostics: &PublishDiagnosticsCapabilities{
					RelatedInformation: true,
					VersionSupport:     true,
				},
				Definition: &DefinitionCapabilities{
					DynamicRegistration: false,
					LinkSupport:         false,
				},
				References: &ReferencesCapabilities{
					DynamicRegistration: false,
				},
				Hover: &HoverCapabilities{
					DynamicRegistration: false,
				},
			},
		},
	}

	var result InitializeResult
	if err := c.sendRequest("initialize", params, &result); err != nil {
		c.initErr = fmt.Errorf("Client.Init: initialize request failed: %w", err)
		return c.initErr
	}

	// Send initialized notification
	if err := c.sendNotification("initialized", struct{}{}); err != nil {
		c.initErr = fmt.Errorf("Client.Init: initialized notification failed: %w", err)
		return c.initErr
	}

	c.initialized.Store(true)
	return nil
}

// TouchAndValidate touches the file at the given path and returns any diagnostics latest changes may have introduced.
func (c *Client) TouchAndValidate(path string, sync bool) ([]Diagnostic, error) {
	if !c.initialized.Load() {
		return nil, fmt.Errorf("Client.TouchAndValidate: client not initialized")
	}

	absPath, err := c.resolvePath(path)
	if err != nil {
		return nil, fmt.Errorf("Client.TouchAndValidate: failed to resolve path: %w", err)
	}

	uri := pathToURI(absPath)

	// Read file content
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("Client.TouchAndValidate: failed to read file: %w", err)
	}

	// Check if document is already open
	c.openDocsMu.RLock()
	version, isOpen := c.openDocs[uri]
	c.openDocsMu.RUnlock()

	if !isOpen {
		// Open the document
		params := DidOpenTextDocumentParams{
			TextDocument: TextDocumentItem{
				URI:        DocumentURI(uri),
				LanguageID: "go", // TODO: detect language from file extension
				Version:    1,
				Text:       string(content),
			},
		}

		if err := c.sendNotification("textDocument/didOpen", params); err != nil {
			return nil, fmt.Errorf("Client.TouchAndValidate: didOpen notification failed: %w", err)
		}

		c.openDocsMu.Lock()
		c.openDocs[uri] = 1
		c.openDocsMu.Unlock()
	} else {
		// Document is already open, send change notification
		version++
		params := DidChangeTextDocumentParams{
			TextDocument: VersionedTextDocumentIdentifier{
				TextDocumentIdentifier: TextDocumentIdentifier{URI: uri},
				Version:                version,
			},
			ContentChanges: []TextDocumentContentChangeEvent{
				{
					Text: string(content),
				},
			},
		}

		if err := c.sendNotification("textDocument/didChange", params); err != nil {
			return nil, fmt.Errorf("Client.TouchAndValidate: didChange notification failed: %w", err)
		}

		c.openDocsMu.Lock()
		c.openDocs[uri] = version
		c.openDocsMu.Unlock()
	}

	if !sync {
		return nil, nil
	}

	// For sync mode, we need to wait for diagnostics
	// This is a simplified implementation - in production, you'd want to use channels or timeouts
	// For now, just return current diagnostics
	return c.getDiagnosticsForURI(uri), nil
}

// Diagnostics returns all current diagnostics known to the LSP client.
func (c *Client) Diagnostics() ([]Diagnostic, error) {
	if !c.initialized.Load() {
		return nil, fmt.Errorf("Client.Diagnostics: client not initialized")
	}

	c.diagnosticsMu.RLock()
	defer c.diagnosticsMu.RUnlock()

	var all []Diagnostic
	for _, diags := range c.diagnostics {
		all = append(all, diags...)
	}

	return all, nil
}

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

	// If no hover information is available, return empty strings
	if result == nil || result.Contents == nil {
		return "", "", nil
	}

	// Parse the contents and extract text and format
	return c.parseHoverContents(result.Contents)
}

// parseHoverContents parses the contents field from a Hover response.
// The contents can be:
// - MarkupContent
// - MarkedString
// - MarkedString[]
// - string
// - string[]
func (c *Client) parseHoverContents(contents interface{}) (string, string, error) {
	// First, marshal to JSON to handle the interface{} type
	data, err := json.Marshal(contents)
	if err != nil {
		return "", "", fmt.Errorf("Client.parseHoverContents: failed to marshal contents: %w", err)
	}

	// Try to unmarshal as MarkupContent (preferred format)
	var markupContent MarkupContent
	if err := json.Unmarshal(data, &markupContent); err == nil {
		if markupContent.Kind != "" && markupContent.Value != "" {
			return markupContent.Value, string(markupContent.Kind), nil
		}
	}

	// Try to unmarshal as MarkedString array
	var markedStrings []MarkedString
	if err := json.Unmarshal(data, &markedStrings); err == nil && len(markedStrings) > 0 {
		// Prefer markdown format if available
		for _, ms := range markedStrings {
			if ms.Language == "" {
				// This is a markdown string
				return ms.Value, "markdown", nil
			}
		}
		// If no markdown, use the first code block as plaintext
		if len(markedStrings) > 0 {
			return markedStrings[0].Value, "plaintext", nil
		}
	}

	// Try to unmarshal as single MarkedString
	var markedString MarkedString
	if err := json.Unmarshal(data, &markedString); err == nil {
		if markedString.Value != "" {
			if markedString.Language == "" {
				return markedString.Value, "markdown", nil
			}
			return markedString.Value, "plaintext", nil
		}
	}

	// Try to unmarshal as string array
	var stringArray []string
	if err := json.Unmarshal(data, &stringArray); err == nil && len(stringArray) > 0 {
		// Concatenate all strings
		var result strings.Builder
		for i, s := range stringArray {
			if i > 0 {
				result.WriteString("\n")
			}
			result.WriteString(s)
		}
		return result.String(), "markdown", nil
	}

	// Try to unmarshal as single string
	var str string
	if err := json.Unmarshal(data, &str); err == nil && str != "" {
		return str, "markdown", nil
	}

	// No hover information available
	return "", "", nil
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

// Close closes the LSP client and shuts down the language server.
func (c *Client) Close() error {
	if !c.initialized.Load() {
		return nil
	}

	// Send shutdown request
	var shutdownResult interface{}
	_ = c.sendRequest("shutdown", nil, &shutdownResult)

	// Send exit notification
	_ = c.sendNotification("exit", nil)

	// Cancel context and wait for process to exit
	c.cancel()

	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Wait()
	}

	return nil
}

// sendRequest sends a request and waits for the response.
func (c *Client) sendRequest(method string, params interface{}, result interface{}) error {
	id := c.nextID.Add(1) - 1

	req := request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	// Create response channel
	respChan := make(chan *response, 1)
	c.pendingMu.Lock()
	c.pending[id] = respChan
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
	}()

	// Send request
	if err := c.writeMessage(req); err != nil {
		return fmt.Errorf("Client.sendRequest: failed to write message: %w", err)
	}

	// Wait for response
	select {
	case resp := <-respChan:
		if resp.Error != nil {
			return fmt.Errorf("Client.sendRequest: LSP error %d: %s", resp.Error.Code, resp.Error.Message)
		}

		if result != nil && resp.Result != nil {
			if err := json.Unmarshal(resp.Result, result); err != nil {
				return fmt.Errorf("Client.sendRequest: failed to unmarshal result: %w", err)
			}
		}

		return nil
	case <-c.ctx.Done():
		return fmt.Errorf("Client.sendRequest: context cancelled")
	}
}

// sendNotification sends a notification (no response expected).
func (c *Client) sendNotification(method string, params interface{}) error {
	notif := notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	if err := c.writeMessage(notif); err != nil {
		return fmt.Errorf("Client.sendNotification: failed to write message: %w", err)
	}

	return nil
}

// writeMessage writes a JSON-RPC message with LSP headers.
func (c *Client) writeMessage(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("Client.writeMessage: failed to marshal message: %w", err)
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := c.stdin.Write([]byte(header)); err != nil {
		return fmt.Errorf("Client.writeMessage: failed to write header: %w", err)
	}

	if _, err := c.stdin.Write(data); err != nil {
		return fmt.Errorf("Client.writeMessage: failed to write data: %w", err)
	}

	return nil
}

// readLoop reads messages from the language server.
func (c *Client) readLoop() {
	reader := bufio.NewReader(c.stdout)

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// Read headers
		headers := make(map[string]string)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					fmt.Fprintf(os.Stderr, "Client.readLoop: failed to read header line: %v\n", err)
				}
				return
			}

			line = strings.TrimSpace(line)
			if line == "" {
				break
			}

			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}

		// Get content length
		contentLengthStr, ok := headers["Content-Length"]
		if !ok {
			fmt.Fprintf(os.Stderr, "Client.readLoop: missing Content-Length header\n")
			continue
		}

		contentLength, err := strconv.Atoi(contentLengthStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Client.readLoop: invalid Content-Length: %v\n", err)
			continue
		}

		// Read content
		content := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, content); err != nil {
			fmt.Fprintf(os.Stderr, "Client.readLoop: failed to read content: %v\n", err)
			return
		}

		// Parse message
		c.handleMessage(content)
	}
}

// handleMessage handles an incoming message from the language server.
func (c *Client) handleMessage(data []byte) {
	var baseMsg struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      *int64          `json:"id,omitempty"`
		Method  string          `json:"method,omitempty"`
		Result  json.RawMessage `json:"result,omitempty"`
		Error   *responseError  `json:"error,omitempty"`
		Params  json.RawMessage `json:"params,omitempty"`
	}

	if err := json.Unmarshal(data, &baseMsg); err != nil {
		fmt.Fprintf(os.Stderr, "Client.handleMessage: failed to unmarshal message: %v\n", err)
		return
	}

	// Check if it's a response
	if baseMsg.ID != nil && baseMsg.Method == "" {
		c.pendingMu.RLock()
		respChan, ok := c.pending[*baseMsg.ID]
		c.pendingMu.RUnlock()

		if ok {
			resp := &response{
				ID:     *baseMsg.ID,
				Result: baseMsg.Result,
				Error:  baseMsg.Error,
			}
			select {
			case respChan <- resp:
			default:
			}
		}
		return
	}

	// It's a notification
	if baseMsg.Method != "" {
		c.handleNotification(baseMsg.Method, baseMsg.Params)
	}
}

// handleNotification handles notifications from the language server.
func (c *Client) handleNotification(method string, params json.RawMessage) {
	switch method {
	case "textDocument/publishDiagnostics":
		var p PublishDiagnosticsParams
		if err := json.Unmarshal(params, &p); err != nil {
			fmt.Fprintf(os.Stderr, "Client.handleNotification: failed to unmarshal publishDiagnostics: %v\n", err)
			return
		}

		c.diagnosticsMu.Lock()
		c.diagnostics[string(p.URI)] = p.Diagnostics
		c.diagnosticsMu.Unlock()

	case "window/logMessage":
		// Ignore log messages for now
	}
}

// readStderr reads stderr from the language server (for debugging).
func (c *Client) readStderr() {
	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		// Silently consume stderr for now
		// In production, you might want to log this
		_ = scanner.Text()
	}
}

// getDiagnosticsForURI returns diagnostics for a specific URI.
func (c *Client) getDiagnosticsForURI(uri string) []Diagnostic {
	c.diagnosticsMu.RLock()
	defer c.diagnosticsMu.RUnlock()

	diags, ok := c.diagnostics[uri]
	if !ok {
		return nil
	}

	return diags
}

// parseLocationResult parses the result from textDocument/definition which can be:
// - null
// - Location
// - Location[]
// - LocationLink[]
func (c *Client) parseLocationResult(result interface{}) ([]Location, error) {
	if result == nil {
		return nil, nil
	}

	// Try to unmarshal as []Location
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("Client.parseLocationResult: failed to marshal result: %w", err)
	}

	var locations []Location
	if err := json.Unmarshal(data, &locations); err == nil {
		return locations, nil
	}

	// Try to unmarshal as single Location
	var location Location
	if err := json.Unmarshal(data, &location); err == nil {
		return []Location{location}, nil
	}

	return nil, fmt.Errorf("Client.parseLocationResult: unsupported result format")
}

// pathToURI converts a file path to a URI.
func pathToURI(path string) string {
	// Ensure path is absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// Convert to URI format
	// On Windows, paths like C:\foo\bar should become file:///C:/foo/bar
	// On Unix, paths like /foo/bar should become file:///foo/bar
	absPath = filepath.ToSlash(absPath)

	// If path doesn't start with /, add it (Windows case)
	if !strings.HasPrefix(absPath, "/") {
		absPath = "/" + absPath
	}

	return "file://" + absPath
}

// resolvePath resolves path to absolute filesystem path using the client working directory
// for relative paths.
func (c *Client) resolvePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("Client.resolvePath: path is empty")
	}

	if filepath.IsAbs(path) {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("Client.resolvePath: failed to resolve absolute path: %w", err)
		}
		return absPath, nil
	}

	absPath, err := filepath.Abs(filepath.Join(c.workingDir, path))
	if err != nil {
		return "", fmt.Errorf("Client.resolvePath: failed to resolve relative path: %w", err)
	}

	return absPath, nil
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func docURIPtr(uri string) *DocumentURI {
	u := DocumentURI(uri)
	return &u
}
