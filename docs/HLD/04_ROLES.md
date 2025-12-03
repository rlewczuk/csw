## VR.4 - Multiple Agent Roles Support

Role-based agent system with customizable prompts, permissions, and filtering rules.

**Core Roles:**
- Architect: High-level design and planning
- Coder: Implementation and code generation
- Tester: Test creation and validation
- Documenter: Documentation generation and maintenance
- custom roles (configurable);

TBD ?? role hierarchies ??

**Role Configuration:**
- Custom system prompts per role
- Permission matrices for tool access
  - Certain permissions available only inside working containers (eg. file operations, terminal commands etc.);
- Content filtering rules
- Role-specific UI adaptations
- Roles can change inside a single task, eg. we can go from architect to coder etc.

### Data Structures and Interfaces

* AgentRole (TBD structure here)
