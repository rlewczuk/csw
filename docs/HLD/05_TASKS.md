## VR.5 - Parallel Task Support

Advanced task isolation using containerization and virtualized file systems.

**Isolation Mechanisms:**
- Git-based versioning with feature branches
- Container-based workspace isolation
- Virtualized layered filesystem (git commit + delta)
- Selective caching of build artifacts and dependencies
- Distributed task execution across multiple hosts

**Infrastructure Support:**
- Local container execution
- Remote execution (SSH, Kubernetes)
- Hybrid local/remote task distribution (eg. git operation local, build inside container);

Other remarks:
* parallel tasks are in principle independent, any dependencies should be resolved prior to starting them (eg. in tasklist level);
* containers are destroyed after task is finished; also task can potentially spawn more than one container or even use container per op;
* cache is mapped between containers (if local) but not kept coherent, ordering of operations (eg. fetching deps in separate steps) is responsible for maintaining consistency;
* docker network is used, so containers have separate ip addresses (i.e. no port conflicts);
  * that is, unless port mapping is required but this is rare;
* host to run a task is in task configuration (i.e. is explicitly configured);

### Interfaces

* container runtime  (or remote workspace)
  * starting, stopping, purging etc.
  * mounting vfs, accessing files (from core agent), getting and committing changes, rolling back to commit etc.
  * executing commands

* cached content
  * creating, updating, preserving, purging etc.
  * local cached content;
* package manager cache (eg. cache for npm) -- future 

