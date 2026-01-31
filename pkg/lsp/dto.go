package lsp

// This file contains all LSP related types and structures for communication as described in LSP specification at:
// https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/

type DiagnosticSeverity int

const (
	SeverityError       DiagnosticSeverity = 1
	SeverityWarning     DiagnosticSeverity = 2
	SeverityInformation DiagnosticSeverity = 3
	SeverityHint        DiagnosticSeverity = 4
)

// Position in a text document expressed as zero-based line and zero-based character offset.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range in a text document expressed as (zero-based) start and end positions.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location represents a location inside a resource, such as a line inside a text file.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// Diagnostic represents a diagnostic, such as a compiler error or warning.
type Diagnostic struct {
	Range    Range              `json:"range"`
	Severity DiagnosticSeverity `json:"severity,omitempty"`
	Code     interface{}        `json:"code,omitempty"` // integer | string
	Source   string             `json:"source,omitempty"`
	Message  string             `json:"message"`
	Tags     []int              `json:"tags,omitempty"`
}

// TextDocumentIdentifier identifies a text document using a URI.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// TextDocumentPositionParams contains a text document and a position inside that document.
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// DefinitionParams parameters for a textDocument/definition request.
type DefinitionParams struct {
	TextDocumentPositionParams
}

// ReferenceContext reference context for textDocument/references request.
type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

// ReferenceParams parameters for a textDocument/references request.
type ReferenceParams struct {
	TextDocumentPositionParams
	Context ReferenceContext `json:"context"`
}

// DocumentURI represents a URI of a document.
type DocumentURI string

// TextDocumentItem represents an item to transfer a text document from the client to the server.
type TextDocumentItem struct {
	URI        DocumentURI `json:"uri"`
	LanguageID string      `json:"languageId"`
	Version    int         `json:"version"`
	Text       string      `json:"text"`
}

// DidOpenTextDocumentParams parameters for textDocument/didOpen notification.
type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// VersionedTextDocumentIdentifier identifies a specific version of a text document.
type VersionedTextDocumentIdentifier struct {
	TextDocumentIdentifier
	Version int `json:"version"`
}

// TextDocumentContentChangeEvent represents a change to a text document.
type TextDocumentContentChangeEvent struct {
	Range       *Range `json:"range,omitempty"`
	RangeLength *int   `json:"rangeLength,omitempty"`
	Text        string `json:"text"`
}

// DidChangeTextDocumentParams parameters for textDocument/didChange notification.
type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

// DidCloseTextDocumentParams parameters for textDocument/didClose notification.
type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// PublishDiagnosticsParams parameters for textDocument/publishDiagnostics notification.
type PublishDiagnosticsParams struct {
	URI         DocumentURI  `json:"uri"`
	Version     *int         `json:"version,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// InitializeParams parameters for initialize request.
type InitializeParams struct {
	ProcessID             *int               `json:"processId"`
	ClientInfo            *ClientInfo        `json:"clientInfo,omitempty"`
	Locale                string             `json:"locale,omitempty"`
	RootPath              *string            `json:"rootPath,omitempty"`
	RootURI               *DocumentURI       `json:"rootUri"`
	InitializationOptions interface{}        `json:"initializationOptions,omitempty"`
	Capabilities          ClientCapabilities `json:"capabilities"`
	Trace                 string             `json:"trace,omitempty"`
	WorkspaceFolders      []WorkspaceFolder  `json:"workspaceFolders,omitempty"`
}

// ClientInfo information about the client.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// WorkspaceFolder represents a workspace folder.
type WorkspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

// ClientCapabilities defines capabilities provided by the client.
type ClientCapabilities struct {
	Workspace    *WorkspaceClientCapabilities    `json:"workspace,omitempty"`
	TextDocument *TextDocumentClientCapabilities `json:"textDocument,omitempty"`
	Window       *WindowClientCapabilities       `json:"window,omitempty"`
	General      *GeneralClientCapabilities      `json:"general,omitempty"`
}

// WorkspaceClientCapabilities workspace specific client capabilities.
type WorkspaceClientCapabilities struct {
	ApplyEdit              bool                                `json:"applyEdit,omitempty"`
	WorkspaceEdit          *WorkspaceEditCapabilities          `json:"workspaceEdit,omitempty"`
	DidChangeConfiguration *DidChangeConfigurationCapabilities `json:"didChangeConfiguration,omitempty"`
	DidChangeWatchedFiles  *DidChangeWatchedFilesCapabilities  `json:"didChangeWatchedFiles,omitempty"`
	Symbol                 *WorkspaceSymbolCapabilities        `json:"symbol,omitempty"`
	ExecuteCommand         *ExecuteCommandCapabilities         `json:"executeCommand,omitempty"`
}

// WorkspaceEditCapabilities workspace edit capabilities.
type WorkspaceEditCapabilities struct {
	DocumentChanges    bool     `json:"documentChanges,omitempty"`
	ResourceOperations []string `json:"resourceOperations,omitempty"`
	FailureHandling    string   `json:"failureHandling,omitempty"`
}

// DidChangeConfigurationCapabilities didChangeConfiguration capabilities.
type DidChangeConfigurationCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// DidChangeWatchedFilesCapabilities didChangeWatchedFiles capabilities.
type DidChangeWatchedFilesCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// WorkspaceSymbolCapabilities workspace symbol capabilities.
type WorkspaceSymbolCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// ExecuteCommandCapabilities execute command capabilities.
type ExecuteCommandCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// TextDocumentClientCapabilities text document specific client capabilities.
type TextDocumentClientCapabilities struct {
	Synchronization    *TextDocumentSyncClientCapabilities   `json:"synchronization,omitempty"`
	Completion         *CompletionCapabilities               `json:"completion,omitempty"`
	Hover              *HoverCapabilities                    `json:"hover,omitempty"`
	SignatureHelp      *SignatureHelpCapabilities            `json:"signatureHelp,omitempty"`
	Declaration        *DeclarationCapabilities              `json:"declaration,omitempty"`
	Definition         *DefinitionCapabilities               `json:"definition,omitempty"`
	TypeDefinition     *TypeDefinitionCapabilities           `json:"typeDefinition,omitempty"`
	Implementation     *ImplementationCapabilities           `json:"implementation,omitempty"`
	References         *ReferencesCapabilities               `json:"references,omitempty"`
	DocumentHighlight  *DocumentHighlightCapabilities        `json:"documentHighlight,omitempty"`
	DocumentSymbol     *DocumentSymbolCapabilities           `json:"documentSymbol,omitempty"`
	CodeAction         *CodeActionCapabilities               `json:"codeAction,omitempty"`
	CodeLens           *CodeLensCapabilities                 `json:"codeLens,omitempty"`
	DocumentLink       *DocumentLinkCapabilities             `json:"documentLink,omitempty"`
	ColorProvider      *DocumentColorCapabilities            `json:"colorProvider,omitempty"`
	Formatting         *DocumentFormattingCapabilities       `json:"formatting,omitempty"`
	RangeFormatting    *DocumentRangeFormattingCapabilities  `json:"rangeFormatting,omitempty"`
	OnTypeFormatting   *DocumentOnTypeFormattingCapabilities `json:"onTypeFormatting,omitempty"`
	Rename             *RenameCapabilities                   `json:"rename,omitempty"`
	PublishDiagnostics *PublishDiagnosticsCapabilities       `json:"publishDiagnostics,omitempty"`
	FoldingRange       *FoldingRangeCapabilities             `json:"foldingRange,omitempty"`
}

// TextDocumentSyncClientCapabilities text document sync capabilities.
type TextDocumentSyncClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	WillSave            bool `json:"willSave,omitempty"`
	WillSaveWaitUntil   bool `json:"willSaveWaitUntil,omitempty"`
	DidSave             bool `json:"didSave,omitempty"`
}

// CompletionCapabilities completion capabilities.
type CompletionCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// HoverCapabilities hover capabilities.
type HoverCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// SignatureHelpCapabilities signature help capabilities.
type SignatureHelpCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// DeclarationCapabilities declaration capabilities.
type DeclarationCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	LinkSupport         bool `json:"linkSupport,omitempty"`
}

// DefinitionCapabilities definition capabilities.
type DefinitionCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	LinkSupport         bool `json:"linkSupport,omitempty"`
}

// TypeDefinitionCapabilities type definition capabilities.
type TypeDefinitionCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	LinkSupport         bool `json:"linkSupport,omitempty"`
}

// ImplementationCapabilities implementation capabilities.
type ImplementationCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	LinkSupport         bool `json:"linkSupport,omitempty"`
}

// ReferencesCapabilities references capabilities.
type ReferencesCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// DocumentHighlightCapabilities document highlight capabilities.
type DocumentHighlightCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// DocumentSymbolCapabilities document symbol capabilities.
type DocumentSymbolCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// CodeActionCapabilities code action capabilities.
type CodeActionCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// CodeLensCapabilities code lens capabilities.
type CodeLensCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// DocumentLinkCapabilities document link capabilities.
type DocumentLinkCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// DocumentColorCapabilities document color capabilities.
type DocumentColorCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// DocumentFormattingCapabilities document formatting capabilities.
type DocumentFormattingCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// DocumentRangeFormattingCapabilities document range formatting capabilities.
type DocumentRangeFormattingCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// DocumentOnTypeFormattingCapabilities document on type formatting capabilities.
type DocumentOnTypeFormattingCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// RenameCapabilities rename capabilities.
type RenameCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	PrepareSupport      bool `json:"prepareSupport,omitempty"`
}

// PublishDiagnosticsCapabilities publish diagnostics capabilities.
type PublishDiagnosticsCapabilities struct {
	RelatedInformation bool `json:"relatedInformation,omitempty"`
	TagSupport         *struct {
		ValueSet []int `json:"valueSet"`
	} `json:"tagSupport,omitempty"`
	VersionSupport bool `json:"versionSupport,omitempty"`
}

// FoldingRangeCapabilities folding range capabilities.
type FoldingRangeCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// WindowClientCapabilities window specific client capabilities.
type WindowClientCapabilities struct {
	WorkDoneProgress bool `json:"workDoneProgress,omitempty"`
}

// GeneralClientCapabilities general client capabilities.
type GeneralClientCapabilities struct {
	RegularExpressions *RegularExpressionsCapabilities `json:"regularExpressions,omitempty"`
	Markdown           *MarkdownCapabilities           `json:"markdown,omitempty"`
}

// RegularExpressionsCapabilities regular expressions capabilities.
type RegularExpressionsCapabilities struct {
	Engine  string `json:"engine"`
	Version string `json:"version,omitempty"`
}

// MarkdownCapabilities markdown capabilities.
type MarkdownCapabilities struct {
	Parser  string `json:"parser"`
	Version string `json:"version,omitempty"`
}

// InitializeResult result of initialize request.
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   *ServerInfo        `json:"serverInfo,omitempty"`
}

// ServerInfo information about the server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// ServerCapabilities defines capabilities provided by a language server.
type ServerCapabilities struct {
	TextDocumentSync                 interface{}                      `json:"textDocumentSync,omitempty"`
	CompletionProvider               *CompletionOptions               `json:"completionProvider,omitempty"`
	HoverProvider                    interface{}                      `json:"hoverProvider,omitempty"`
	SignatureHelpProvider            *SignatureHelpOptions            `json:"signatureHelpProvider,omitempty"`
	DeclarationProvider              interface{}                      `json:"declarationProvider,omitempty"`
	DefinitionProvider               interface{}                      `json:"definitionProvider,omitempty"`
	TypeDefinitionProvider           interface{}                      `json:"typeDefinitionProvider,omitempty"`
	ImplementationProvider           interface{}                      `json:"implementationProvider,omitempty"`
	ReferencesProvider               interface{}                      `json:"referencesProvider,omitempty"`
	DocumentHighlightProvider        interface{}                      `json:"documentHighlightProvider,omitempty"`
	DocumentSymbolProvider           interface{}                      `json:"documentSymbolProvider,omitempty"`
	CodeActionProvider               interface{}                      `json:"codeActionProvider,omitempty"`
	CodeLensProvider                 *CodeLensOptions                 `json:"codeLensProvider,omitempty"`
	DocumentLinkProvider             *DocumentLinkOptions             `json:"documentLinkProvider,omitempty"`
	ColorProvider                    interface{}                      `json:"colorProvider,omitempty"`
	DocumentFormattingProvider       interface{}                      `json:"documentFormattingProvider,omitempty"`
	DocumentRangeFormattingProvider  interface{}                      `json:"documentRangeFormattingProvider,omitempty"`
	DocumentOnTypeFormattingProvider *DocumentOnTypeFormattingOptions `json:"documentOnTypeFormattingProvider,omitempty"`
	RenameProvider                   interface{}                      `json:"renameProvider,omitempty"`
	FoldingRangeProvider             interface{}                      `json:"foldingRangeProvider,omitempty"`
	ExecuteCommandProvider           *ExecuteCommandOptions           `json:"executeCommandProvider,omitempty"`
	WorkspaceSymbolProvider          interface{}                      `json:"workspaceSymbolProvider,omitempty"`
	Workspace                        *WorkspaceServerCapabilities     `json:"workspace,omitempty"`
}

// CompletionOptions completion options.
type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
	ResolveProvider   bool     `json:"resolveProvider,omitempty"`
}

// SignatureHelpOptions signature help options.
type SignatureHelpOptions struct {
	TriggerCharacters   []string `json:"triggerCharacters,omitempty"`
	RetriggerCharacters []string `json:"retriggerCharacters,omitempty"`
}

// CodeLensOptions code lens options.
type CodeLensOptions struct {
	ResolveProvider bool `json:"resolveProvider,omitempty"`
}

// DocumentLinkOptions document link options.
type DocumentLinkOptions struct {
	ResolveProvider bool `json:"resolveProvider,omitempty"`
}

// DocumentOnTypeFormattingOptions document on type formatting options.
type DocumentOnTypeFormattingOptions struct {
	FirstTriggerCharacter string   `json:"firstTriggerCharacter"`
	MoreTriggerCharacter  []string `json:"moreTriggerCharacter,omitempty"`
}

// ExecuteCommandOptions execute command options.
type ExecuteCommandOptions struct {
	Commands []string `json:"commands"`
}

// WorkspaceServerCapabilities workspace specific server capabilities.
type WorkspaceServerCapabilities struct {
	WorkspaceFolders *WorkspaceFoldersServerCapabilities `json:"workspaceFolders,omitempty"`
}

// WorkspaceFoldersServerCapabilities workspace folders server capabilities.
type WorkspaceFoldersServerCapabilities struct {
	Supported           bool   `json:"supported,omitempty"`
	ChangeNotifications string `json:"changeNotifications,omitempty"`
}

// HoverParams parameters for a textDocument/hover request.
type HoverParams struct {
	TextDocumentPositionParams
}

// MarkupKind describes the content type of a MarkupContent.
type MarkupKind string

const (
	PlainText MarkupKind = "plaintext"
	Markdown  MarkupKind = "markdown"
)

// MarkupContent represents a string value which content can be represented in different formats.
type MarkupContent struct {
	Kind  MarkupKind `json:"kind"`
	Value string     `json:"value"`
}

// MarkedString can be used to render human readable text. It is either a markdown string
// or a code-block that provides a language and a code snippet.
type MarkedString struct {
	Language string `json:"language,omitempty"`
	Value    string `json:"value"`
}

// Hover represents the result of a hover request.
type Hover struct {
	Contents interface{} `json:"contents"` // MarkedString | MarkedString[] | MarkupContent
	Range    *Range      `json:"range,omitempty"`
}

// SymbolKind represents the kind of a symbol.
type SymbolKind int

const (
	SymbolKindFile          SymbolKind = 1
	SymbolKindModule        SymbolKind = 2
	SymbolKindNamespace     SymbolKind = 3
	SymbolKindPackage       SymbolKind = 4
	SymbolKindClass         SymbolKind = 5
	SymbolKindMethod        SymbolKind = 6
	SymbolKindProperty      SymbolKind = 7
	SymbolKindField         SymbolKind = 8
	SymbolKindConstructor   SymbolKind = 9
	SymbolKindEnum          SymbolKind = 10
	SymbolKindInterface     SymbolKind = 11
	SymbolKindFunction      SymbolKind = 12
	SymbolKindVariable      SymbolKind = 13
	SymbolKindConstant      SymbolKind = 14
	SymbolKindString        SymbolKind = 15
	SymbolKindNumber        SymbolKind = 16
	SymbolKindBoolean       SymbolKind = 17
	SymbolKindArray         SymbolKind = 18
	SymbolKindObject        SymbolKind = 19
	SymbolKindKey           SymbolKind = 20
	SymbolKindNull          SymbolKind = 21
	SymbolKindEnumMember    SymbolKind = 22
	SymbolKindStruct        SymbolKind = 23
	SymbolKindEvent         SymbolKind = 24
	SymbolKindOperator      SymbolKind = 25
	SymbolKindTypeParameter SymbolKind = 26
)

// SymbolTag represents extra annotations for symbols.
type SymbolTag int

const (
	SymbolTagDeprecated SymbolTag = 1
)

// DocumentSymbol represents programming constructs like variables, classes, interfaces etc.
// that appear in a document. Document symbols can be hierarchical and they have two ranges:
// one that encloses its definition and one that points to its most interesting range.
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           SymbolKind       `json:"kind"`
	Tags           []SymbolTag      `json:"tags,omitempty"`
	Deprecated     bool             `json:"deprecated,omitempty"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

// DocumentSymbolParams parameters for a textDocument/documentSymbol request.
type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// WorkspaceSymbol represents a symbol found in a workspace.
// WorkspaceSymbol can be either a simple symbol or a detailed symbol with location information.
type WorkspaceSymbol struct {
	Name string      `json:"name"`
	Kind SymbolKind  `json:"kind"`
	Tags []SymbolTag `json:"tags,omitempty"`
	// The name of the symbol containing this symbol. This information is for
	// user interface purposes (e.g. to render a qualifier in the user interface
	// if necessary). It can't be used to re-infer a hierarchy for the document
	// symbols.
	ContainerName string   `json:"containerName,omitempty"`
	Location      Location `json:"location"`
	// A data entry field that is preserved on a workspace symbol between a
	// workspace symbol request and a workspace symbol resolve request.
	Data interface{} `json:"data,omitempty"`
}

// WorkspaceSymbolParams parameters for a workspace/symbol request.
type WorkspaceSymbolParams struct {
	// A query string to filter symbols by. Clients may send an empty
	// string here to request all symbols.
	Query string `json:"query"`
}

// CallHierarchyPrepareParams parameters for a textDocument/prepareCallHierarchy request.
type CallHierarchyPrepareParams struct {
	TextDocumentPositionParams
}

// CallHierarchyItem represents a call hierarchy item.
type CallHierarchyItem struct {
	// The name of this item.
	Name string `json:"name"`
	// The kind of this item.
	Kind SymbolKind `json:"kind"`
	// Tags for this item.
	Tags []SymbolTag `json:"tags,omitempty"`
	// More detail for this item, e.g. the signature of a function.
	Detail string `json:"detail,omitempty"`
	// The resource identifier of this item.
	URI string `json:"uri"`
	// The range enclosing this symbol not including leading/trailing whitespace
	// but everything else, e.g. comments and code.
	Range Range `json:"range"`
	// The range that should be selected and revealed when this symbol is being
	// picked, e.g. the name of a function. Must be contained by the `range`.
	SelectionRange Range `json:"selectionRange"`
	// A data entry field that is preserved between a call hierarchy prepare and
	// incoming calls or outgoing calls requests.
	Data interface{} `json:"data,omitempty"`
}

// CallHierarchyIncomingCallsParams parameters for a callHierarchy/incomingCalls request.
type CallHierarchyIncomingCallsParams struct {
	Item CallHierarchyItem `json:"item"`
}

// CallHierarchyIncomingCall represents an incoming call, e.g. a caller of a method or constructor.
type CallHierarchyIncomingCall struct {
	// The item that makes the call.
	From CallHierarchyItem `json:"from"`
	// The ranges at which the calls appear. This is relative to the caller
	// denoted by `from`.
	FromRanges []Range `json:"fromRanges"`
}

// CallHierarchyOutgoingCallsParams parameters for a callHierarchy/outgoingCalls request.
type CallHierarchyOutgoingCallsParams struct {
	Item CallHierarchyItem `json:"item"`
}

// CallHierarchyOutgoingCall represents an outgoing call, e.g. calling a getter from a method or
// constructing an object via a constructor.
type CallHierarchyOutgoingCall struct {
	// The item that is called.
	To CallHierarchyItem `json:"to"`
	// The range at which this item is called. This is the range relative to
	// the caller, e.g the item passed to `callHierarchy/outgoingCalls` request.
	FromRanges []Range `json:"fromRanges"`
}
