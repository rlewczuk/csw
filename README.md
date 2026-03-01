# Codesnort Software Worker

Codesnort is a software engineering (SWE) agent designed primarily for secure operation in non-interactive mode. It leverages AI capabilities to assist with code development, refactoring, debugging, and other software engineering tasks while maintaining security and safety as core principles.

## Features

- **AI-Powered Code Assistance**: Leverages LLMs to help with development, refactoring, and debugging tasks
- **Secure Non-Interactive Mode**: Designed for safe automated operation with configurable permissions
- **Git Worktree Support**: Isolated branch-based sessions with optional auto-merge
- **Containerized Execution**: Run commands in isolated containers for enhanced security
- **LSP Integration**: Language Server Protocol support for intelligent code operations
- **Customizable Roles**: Define roles with specific permissions, tools, and prompt fragments
- **Custom Tools**: Extend functionality with user-defined tools
- **Multi-Provider Support**: Works with OpenAI, Ollama, and other providers (including OAuth)
- **Session Management**: Save, resume, and continue previous sessions

## Current Status

**Work in Progress**: This project is actively under development and is not yet ready for production use. Features, APIs, and behavior may change significantly as the project evolves. Use at your own risk.

## Building and Running

To build the agent:

```bash
go build ./cmd/csw
```

To run the agent:

```bash
go run ./cmd/csw
```

For detailed usage instructions and user manual, please refer to [docs/USER.md](docs/USER.md).

## License

This project is licensed under the Apache License 2.0.

## Credits

This project was bootstrapped using the fabulous [Opencode](https://github.com/anomalyco/opencode) tool.

This is an experiment in implementing larger software projects using (almost) exclusively AI for code generation. 
The Opencode project has been instrumental in making this possible, great thanks to authors for making it available for everyone.

