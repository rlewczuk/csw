## VR.7 - MCP Tool Integration


Comprehensive Model Context Protocol integration with security isolation and permission management.

MCP Architecture:
- Containerized MCP servers for security isolation
- Automated server installation and configuration (MCP registry), containers are ephpemeral anyway, resource limits configured on containers (if any);
- Internal tool architecture unified with MCP
- Docker Dynamic MCP - switching and limiting context occupation;
- Sampling and UI interaction support
- Calling back LLMs (when needed)

### Interfaces

* Mcp Server
  * starting, stopping, configuring (custom conf, eg. database credentials etc.);
  * listing tools and their descriptions;
* Tool Registry
  * lists all available tools (from all servers plus internal tools);
  * provides means to filter potentially needed tools based on context and role;
