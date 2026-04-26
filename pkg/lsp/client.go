package lsp

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
