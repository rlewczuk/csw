## VR.1 - Multiple Interface Support

The system must provide a unified experience across multiple interface modalities while maintaining state consistency and feature parity where applicable.

Available UIs:
- **Web-based GUI** (`VR.1.1`): Primary interface using modern web technologies, embeddable in IDE webview containers
- **Terminal UI** (`VR.1.3`): Full-featured text-based interface with all capabilities except image display

Additional channels for IDE integration:
- **ACP Integration** (`VR.1.4`): Agent Client Protocol support for compatible IDEs
- **MCP Integration** (`VR.1.6`): Symbol resolution and IDE integration via Model Context Protocol for IDEs exposing MCP servers (eg. in Jetbrains IDEs);
- **Custom IDE Backchannel** (`VR.1.2`): File change notification system for JetBrains and VSCode etc.

Additional assumptions:
* only one of user facing interactive interfaces is active at a time, so there is no need to synchronize state between them;
* all additional channels for IDE integration are stateless;
* all core features are available in both UIs;
* either HTTP(S)+SSE or stdin/stdout with JSON messages are used for interprocess communication (as supported by MCP and ACP);
* streaming from LLM is supported only in UI;


### Interfaces

* Agent-UI (internal, duplex);
* Agent-IDE (ACP) (external, duplex, per specification);
* Agent-Tool (MCP) (external, duplex, per specification):
  * supporting tool-priginated messaging (notifications etc.);
  * supporting tool queries (sending requests from tool to LLM);
  * supporting self advertising etc.
* Task list Management (internal) - for managing status of executed tasks/steps;


