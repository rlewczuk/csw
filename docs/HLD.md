# Top Level Requirements

Application name is `Codesnort SWE` and it is tool for end-to-end agentic software development, including specification, design, planning, coding and testing. It integrates spec driven development principles with iterative process of developing application. It is designed to work with large codebases and provide stable, predictable process of developing application features.

Top level requirements:
* **VR.1** - provides multiple ways to interact with it:
  * **VR.1.1** graphical interface (web based, embeddable in IDEs supporting embedded browser or web app rendering);
  * **VR.1.2** optional backchannel for notifying IDEs about changed files: jetbrains, vscode;
  * **VR.1.3** TUI for working in terminal (providing all features of graphical, except for displaying images);
  * **VR.1.4** ACP integration for IDEs supporting it;
  * **VR.1.5** supports streaming responses, rendered in task window (not necessarily in editor);
  * **VT.1.6** MCP integration with Jetbrains IDEs (via built in MCP server) - for symbol resolution etc.;
* **VR.2** LLM integrations
  * **VR.2.1** provides multiple integrations with LLMs, openai, anthropic, google, ollama, openrouter, deepseek;
  * **VR.2.2** selection of given LLM dependend on agent role (and possible to change interactively in local project or tasks);
  * **VR.2.3** ability to customize prompts or adding extra instructions for given LLM;
* **VR.3** supports checkpointing and proper change management using git;
  * **VR.3.1** navigation over checkpoint, merges etc. available from any user interface;
  * **VR.3.2** maintains feature branch per task (i.e. "task branch") with commits as checkpoints;
  * **VR.3.3** able to work either from local or remote repository;
* **VR.4** support for multiple agent roles (architect, coder, tester, documenter etc.):
  * **VR.4.1** custom system prompt for given role;
  * **VR.4.2** custom permissions for given role;
  * **VR.4.3** filtering rules for given role;
* **VR.5** supports multiple tasks working in parallel:
  * **VR.5.1** using git for versioning/applying changes, in a process similar to feature branches;
  * **VR.5.2** using container to maintain isolated workspace for a task (if needed);
  * **VR.5.3** virtualized layered FS based on git commit + delta, exposed to a container;
  * **VR.5.4** proper caching of certain workspace parts (eg. `node_modules`, certain build artifacts etc.);
  * **VR.5.5** support for running workspace containers on both local and remote system (incl kubernetes, remote SSH etc.), eg. for dependencies;
  * **VR.5.6** process per task, tasks may work on various hosts (i.e. distributed design);
* **VR.6** supports library of blueprints for projects and components, which contain:
  * **VR.6.1** templates of files added to project as part of blueprint;
  * **VR.6.2** or commands for creating project (or component), eg. calling npx/bunx with certain parameters;
  * **VR.6.3** predefined named actions along with fragments of system prompt describing how to handle result of given action;
  * **VR.6.4** rules for LLM added along with blueprint, governing agent future behavior;
* **VR.7.1** supports calls of external tools via MCP, properly isolated for security (except for common tools available via ACP);
  * **VR.7.2** isolated MCP servers running in containers (with optional mapping of project tree, read only or read-write);
  * **VR.7.3** automated installation configuration of MCP servers;
  * **VR.7.4** almost all tools work internally via MCP, especially ones calling external processes, eg. browser;
  * **VR.7.5** for each tool there are default permissions;
  * **VR.7.6** for each role there are permissions assigned to individual tools;
  * **VR.7.7** enable MCP sampling;
  * **VR.7.8** enable MCP UI for interaction;
* **VR.8** supports proper navigation over codebase:
  * **VR.8.1** parsing project structure and code files down to individual symbols like in modern IDEs - work with LSP for this end;
  * **VR.8.2** summarizing and making codebase searchable - hierarchical structure: modules, classes, methods;
  * **VR.8.3** customized additions to system prompt dependent on project structure, programming language, technologies used etc.
  * **VR.8.4** using vector database if needed (or other methos);
* **VR.9** support for proper navigation over development process - eg. SDD-like iterative process:
  * **VR.9.1** extend chat box to a more full editor, so that developer is incentivized to write more details what he wants; at minimum do use Ctrl-Enter instead of Enter to run, also make input box extendable, not scrollable and with some more editor features, plus prompt refinement etc.
  * **VR.9.2** iterate-over-prompt mode where LLM looks for ambiguities and asks questions to refine prompt;
  * **VR.9.3** iterative overall process with mini-waterfalls for significant tasks (i.e. requirements-refine-design-plan-implement-test-merge);
* **VR.10** support for detailed, interactive task planning
  * **VR.10.1** hierarchical planning tree (unlimited depth)
  * **VR.10.2** planning tree spanning all the way from top level requirements specification to low level code changes;
  * **VR.10.3** agent can decide to run certain parts of a todo list in a separate subtask, subtask can have lower level subtasks;
  * **VR.10.4** agent maintains status of todo items;
  * **VR.10.5** developer can alter todo lists and choose items to work on (granted that all dependencies are already executed);
* **VR.11** support predefined recipes for various tasks -- named entities for starting new tasks, containing:
  * **VR.11.1** role in which recipe should be started;
  * **VR.11.2** configurable prompt templates + parameters to ask;
  * **VR.11.3** additional data (if needed);
  * **VR.11.4** specific instructions and rules activated only within given recipe;
  * **VR.11.5** recipe can contain multiple steps and call subordinate recipes, can do some planning etc;
* **VR.12** management of tasks and their logs
  * **VR.12.1** all information stored: interaction, starting git commit id, all file changes etc. 
  * **VR.12.2** fully repeatable task - we have all data needed to execute everything again;
  * **VR.12.3** store also cancelled steps and their data (at least for viewing);
  * **VR.12.4** pack old tasks so that there is less space used;
  * **VR.12.5** if configured, remove very old tasks;
* **VR.13** security considerataions
  * **VR.13.1** all credentials isolated from build and codebase;
  * **VR.13.2** all risky processes (build, tests etc.) executed in isolated environment (container);
* **VR.14** maintain actions (from library or blueprints);
  * **VR.14.1** action contains name and description;
  * **VR.14.2** containing command and extra information to interpret results;
  * **VR.14.3** action can be displayed as an icon or via menu, easy to run;
  * **VR.14.4** action can have structured input and/or output;
  * **VR.14.5** action refers to a single packaged command;
  * **VR.14.6** for each action there are default permissions defined (allow, deny, ask);
  * **VR.14.7** for each role there are permissions assigned for individual actions;
  * **VR.14.8** action can be also a prompt

---





## Cross-Cutting Concerns and System Dependencies

### Critical Dependency Chains

1. **Task Execution Chain**: [`VR.10`](PRD/HLD.md:54) → [`VR.5`](PRD/HLD.md:25) → [`VR.3`](PRD/HLD.md:17) → [`VR.12`](PRD/HLD.md:66)
2. **Security Chain**: [`VR.13`](PRD/HLD.md:72) → [`VR.7`](PRD/HLD.md:37) → [`VR.5`](PRD/HLD.md:25) → [`VR.14`](PRD/HLD.md:75)
3. **Interface Chain**: [`VR.1`](PRD/HLD.md:6) → [`VR.4`](PRD/HLD.md:21) → [`VR.2`](PRD/HLD.md:13) → [`VR.7`](PRD/HLD.md:37)
4. **Content Chain**: [`VR.8`](PRD/HLD.md:45) → [`VR.10`](PRD/HLD.md:54) → [`VR.9`](PRD/HLD.md:50) → [`VR.11`](PRD/HLD.md:60)

### System-Wide Ambiguities

1. **State Consistency**: How is state synchronized across parallel tasks, multiple interfaces, and distributed execution?
2. **Resource Management**: How are system resources allocated and managed across all components?
3. **Error Recovery**: What is the system-wide error recovery and rollback strategy?
4. **Performance Requirements**: What are the performance benchmarks and SLAs for each component?
5. **Scalability Limits**: What are the scalability limits for concurrent users, parallel tasks, and project sizes?

### Integration Complexity Matrix

| Component | High Complexity | Medium Complexity | Low Complexity |
|-----------|----------------|-------------------|----------------|
| VR.1 (Interfaces) | VR.4, VR.7, VR.12 | VR.2, VR.13 | VR.8, VR.9 |
| VR.5 (Parallel Tasks) | VR.3, VR.12, VR.13 | VR.8, VR.10 | VR.1, VR.2 |
| VR.7 (MCP Integration) | VR.1, VR.4, VR.13 | VR.14, VR.8 | VR.2, VR.6 |

### Architecture Decision Points

1. **Container Orchestration**: Choice between Docker Compose, Kubernetes, or custom orchestration for [`VR.5`](PRD/HLD.md:25) parallel tasks
2. **State Management**: Centralized vs. distributed state management across [`VR.1`](PRD/HLD.md:6) multiple interfaces
3. **Database Architecture**: Choice of database technology for [`VR.12`](PRD/HLD.md:66) task management and [`VR.8`](PRD/HLD.md:45) code navigation
4. **Security Model**: Zero-trust vs. perimeter-based security for [`VR.13`](PRD/HLD.md:72) implementation
5. **Communication Protocol**: gRPC vs. REST vs. WebSocket for inter-component communication

### Implementation Priority Recommendations

**Phase 1 (Foundation):**
- [`VR.13`](PRD/HLD.md:72) Security Considerations
- [`VR.3`](PRD/HLD.md:17) Git-Based Change Management
- [`VR.2`](PRD/HLD.md:13) LLM Integration Architecture

**Phase 2 (Core Features):**
- [`VR.1`](PRD/HLD.md:6) Multiple Interface Support
- [`VR.4`](PRD/HLD.md:21) Multiple Agent Roles Support
- [`VR.12`](PRD/HLD.md:66) Task and Log Management

**Phase 3 (Advanced Features):**
- [`VR.5`](PRD/HLD.md:25) Parallel Task Support
- [`VR.7`](PRD/HLD.md:37) MCP Tool Integration
- [`VR.8`](PRD/HLD.md:45) Codebase Navigation

**Phase 4 (Intelligence Layer):**
- [`VR.10`](PRD/HLD.md:54) Interactive Task Planning
- [`VR.9`](PRD/HLD.md:50) Development Process Navigation
- [`VR.11`](PRD/HLD.md:60) Predefined Recipes

**Phase 5 (Extensibility):**
- [`VR.6`](PRD/HLD.md:32) Blueprint Library Support
- [`VR.14`](PRD/HLD.md:75) Actions Maintenance
