### VR.11 - Predefined Recipes

#### Technical Specifications
Template-based task initiation system with configurable workflows and nested recipe support.

**Recipe Components:**
- Starting role specification
- Parameterized prompt templates
- Additional context data
- Recipe-specific rules and instructions
- Multi-step workflow definitions
- Subordinate recipe invocation

**Recipe Capabilities:**
- Parameter collection and validation
- Conditional workflow branching
- Planning integration
- Role transitions within recipes

#### Ambiguities & Questions
1. **Recipe Discovery**: How are recipes organized, searched, and discovered by users?
2. **Recipe Composition**: Can recipes be composed from other recipes, and how deep can nesting go?
3. **Parameter Validation**: What validation is performed on recipe parameters?
4. **Recipe Versioning**: How are recipe versions managed and backward compatibility ensured?
5. **Custom Recipes**: Can users create and share custom recipes?
6. **Recipe Debugging**: How are recipe execution issues diagnosed and debugged?

#### Dependencies & Integration Points
- **Uses**: [`VR.4`](PRD/HLD.md:21) (Agent Roles) - Recipes specify starting roles
- **Integrates with**: [`VR.6`](PRD/HLD.md:32) (Blueprints) - Recipes may reference blueprints
- **Supports**: [`VR.10`](PRD/HLD.md:54) (Task Planning) - Recipes can generate planning structures
- **Uses**: [`VR.2`](PRD/HLD.md:13) (LLM Integration) - Recipe-specific prompt customization

