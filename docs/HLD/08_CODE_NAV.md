## VR.8 - Codebase Navigation

Advanced code understanding and navigation system with LSP integration.

**Navigation Capabilities:**
- LSP-based symbol parsing and resolution
- Hierarchical code structure (modules → classes → methods)
- Intelligent code summarization (with LLM)
- Vector database integration for semantic search (embedded?)
- Context-aware system prompt customization

**Technology Integration:**
- Language Server Protocol (LSP) support
- Multiple programming language support
- Framework and technology detection
- Dependency analysis and mapping
- LSP servers also running in containers

### Interfaces

* LanguageServer - interacts with LSP, using all needed 
  * TBD LSP access via MCP ?? is there ready to use MCP server running LSP ?? 
