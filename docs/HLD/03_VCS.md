
## VR.3 - Git-Based Change Management

Comprehensive version control integration with checkpoint-based development workflow.

**Git Workflow:**
- Feature branch per task ("task branches" under `swe/tasks/<task-name>`)
- Commits as development checkpoints (automated, squashed together at the end);
- Support for both local and remote repositories
- Navigation interface for checkpoint history
- Merge conflict resolution assistance
- Authentication for remote repositories taken from commandline git (eg. SSH keys etc);
- Each checkpoint has UUID generated and added to commit as metadata;
- All steps automated for short tasks, including merge if there are no conflicts;

### Interfaces

* git versioned repository:
  * managing branches, commits, tags etc.
  * cloning, pushing, pulling, merging, rebasing etc.
* virtual snapshot
  * checking out from git repo;
  * commiting changes
  * mapping to a container (without need to chec)


