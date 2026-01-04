Implement mechanism of agent roles and integrate it into agent core:
* create `pkg/core/role.go` with `AgentRole` definition;
* integrate role into `SweSession`:
  * add `role` field to `SweSession`;
  * add `SetRole()` method to `SweSession` that changes role;
  * add `Role()` method to `SweSession` that returns current role;
  * add or change system prompt at the beginning of conversation when role is changed;
* enforce VFS privileges defined in role by wrapping `SweSession.VFS` with `AccessControlVFS` from `pkg/vfs/access.go` during construction;
* enforce tool access defined in role by wrapping `SweSession.Tools` with `AccessControlTool` from `pkg/tool/access.go` during construction;
* session can choose from role map present in `SweSystem` object which created it, see `Roles` field;   

