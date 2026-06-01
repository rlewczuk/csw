# Question

Find materials, papers, sample projects regarding Agentic Context Management - a feature that allows agent to effectively manage its session context:
* look for ways to effectively manage context in agentic system working with model of limited context size (eg. 96k or 128k tokens)
* look for projects/agents/plugins implementing more advanced context management strategies (more advanced than simple summarization):
  * especially ones that leverage agent reasoning and memory to manage context effectively, 
  * ones involving saving fragments of context to external storage, retrievable by agent on demand
  * ones that are capable of splitting task into smaller subtasks and clean up fragments of context after subtasks are completed
  * ones implementing progressive disclosure and compression
  * ones implementing dynamic truncation and precise forgetting (eg. when working with hierarchical task decomposition)
  * code level strategies (eg. knowledge graph indexing, knowledge graph indexing)
  * other strategies you can find
* context management strategies should be optimized for working with software projects containing code files (lots of, but highly structured) and documentation
* context management strategies should work well with kv caching (i.e. do not break cache by rewriting context from scratch at every request)
* you can leverage LSP, for example to limit the amount of context retrieved by the agent to only the relevant fragments of code files and documentation
* below are sample projects that demonstrate parts of it that I found useful, 
  * session tree feature in Pi agent
  * https://github.com/ttttmr/pi-context - agentic context management plugin for pi  (https://blog.xlab.app/p/51d26495/ - blog post regarding its implementation)
  * d-mail feature in kimi-cli
  * please find more projects implementing various aspects of context management mentioned above:
* please look for:
  * projects implementing above mentioned strategies
  * papers, blog posts and other resources describing ideas and concrete implementations
  * please group and summarize the findings
  * please find as many projects, papers, ideas and resources as possible


# Agentic Context Management for Code Agents

## What the evidence says

The strongest pattern across current research and open-source implementations is that **“context management” is no longer just summarization**. The best systems combine: a lossless or near-lossless session store, selective retrieval from external memory, code-aware structural indexing, subagent-based context quarantine, and non-destructive pruning that changes what is *sent* to the model without rewriting the underlying session history. This is the common thread running through Kimi Code CLI’s checkpoint/D-Mail mechanism, Pi’s session tree and related plugins, Letta/MemGPT’s virtual memory model, LangChain Deep Agents’ filesystem-backed memory plus subagents, and code-aware retrieval systems such as Aider, Continue, and Sourcegraph Cody. [Kimi](https://moonshotai.github.io/kimi-cli/en/customization/agents.html) [Aider](https://aider.chat/docs/repomap.html) 

For software projects with many code files and docs, the most promising designs are **structure-first** rather than **transcript-first**. In practice, that means using ASTs, call graphs, symbol indexes, repo maps, or knowledge graphs so the agent can ask for *the right slices* of code instead of replaying whole files or ever-growing chat logs. Aider’s repository map, Sourcegraph’s code-intelligence-backed retrieval and ranking, Continue’s repository-map/provider model, and newer MCP servers such as AutoDocs, code-review-graph, Qartez, and Open Aware all move in this direction. [Aider](https://aider.chat/docs/repomap.html) 

If your goal is a coding agent that works well with **96k–128k models and KV/prefix caching**, the evidence points toward a specific architectural bias: keep an immutable or slowly changing prefix stable, externalize volatile detail into files/DB/graph indexes, use append-only or checkpointed state rather than full prompt rewrites, and let the agent fetch detail on demand. This matches Anthropic’s prompt-caching guidance, vLLM’s automatic prefix caching design, and several agent frameworks that isolate heavy work in subagents or filesystems. [Anthropic Prompt Caching](https://platform.claude.com/docs/en/build-with-claude/prompt-caching)

My bottom line: **there is no single project that already solves the entire problem cleanly**, but the closest composite solution today is a blend of **Pi/Kimi-style checkpointed session control**, **Letta/LangGraph-style externalized memory**, **Aider/Sourcegraph-style code indexing**, and **Anthropic/vLLM-style cache-aware prefix discipline**: 
* [pi-context](https://github.com/ttttmr/pi-context) 
* [Implementation of Dynamic Context Pruning](https://github.com/earendil-works/pi/discussions/330)
* [earendil session file format](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/session-format.md)
* [Kimi: RalphFlow architecture](https://github.com/MoonshotAI/kimi-cli/pull/1960)
* [pi-context-prune](https://github.com/championswimmer/pi-context-prune)
* [opencode-dynamic-context-pruning](https://github.com/Opencode-DCP/opencode-dynamic-context-pruning/blob/master/README.md)
* [open-aware](https://github.com/qodo-ai/open-aware)
* [AutoDocs](https://github.com/TrySita/AutoDocs)
* [code-review-graph](https://github.com/tirth8205/code-review-graph)
* [OpenHands: Memory Condensation](https://github.com/OpenHands/OpenHands/issues/5715)
* [qartez-mcp](https://github.com/kuberstar/qartez-mcp)

## Strategy landscape

A useful way to group the space is by the *kind* of context operation the system performs. The first family is **branching and time travel**: instead of compressing a long session into one lossy summary, the agent creates checkpoints, branches off for exploration, and later returns to an earlier point with a compact handoff note. Kimi’s `SendDMail` is explicitly documented as a delayed message for checkpoint rollback; Pi stores sessions as a tree via `id`/`parentId`; the `pi-context` plugin adds `context_checkpoint`, `context_timeline`, and `context_compact` on top of that tree. Recent papers such as **LCM** and **Contextual Memory Virtualisation** push the same idea into a more formal “lossless” or structurally lossless architecture:
* [MemGPT](https://arxiv.org/abs/2310.08560)
* [Reflexion](https://arxiv.org/abs/2303.11366)
* [Generative Agents](https://arxiv.org/abs/2304.03442)
* [HiAgent](https://arxiv.org/abs/2408.09559)
* [A-MEM](https://arxiv.org/abs/2502.12110v11)
* [Agentic Memory](https://arxiv.org/abs/2601.01885)
* [RAPTOR](https://arxiv.org/abs/2401.18059)
* [Graph RAG](https://arxiv.org/abs/2404.16130v2)
* [Hippo-Rag](https://arxiv.org/abs/2405.14831)
* [LCM](https://arxiv.org/abs/2605.04050)
* [Contextual Memory Virtualization](https://arxiv.org/abs/2602.22402)
* [SideQuest](https://arxiv.org/abs/2602.22603v2)
* [Lightweight LLM Agent Memory](https://arxiv.org/abs/2604.07798v3)
* [Memory OS of AI agent](https://arxiv.org/abs/2506.06326)
* [MIRIX](https://arxiv.org/abs/2507.07957)
* 

The second family is **externalized memory**: move durable knowledge out of the hot prompt and let the agent read/write it on demand. MemGPT introduced “virtual context management” with memory tiers; Letta turns this into persisted memory blocks and, more recently, git-backed “context repositories”; LangChain Deep Agents expose filesystem-backed memory plus durable stores; Mem0 layers conversation, session, user, and organization memory; Graphiti/Zep uses a temporal knowledge graph as the memory substrate. These systems are especially relevant when tasks span multiple sessions, users, or branches. citeturn32search0turn30view0turn30view1turn36view0turn36view2turn35view1turn35view2turn34view0turn34view2

The third family is **context quarantine through subtasking**. LangChain explicitly describes subagents as useful for “context quarantine,” Deep Agents recommends using the filesystem for large subagent outputs, and Kimi persists subagent context and wire logs under per-subagent session directories. In other words, one of the cleanest ways to keep a main thread small is not to prune it after the fact, but to **never let all intermediate detail into it in the first place**. HiAgent’s research result—that subgoal-based working-memory management improves long-horizon agent tasks—supports the same direction from the academic side. 
* [LangChain Subagents](https://docs.langchain.com/oss/python/deepagents/subagents)
* [Context Engineering in DeepAgents](https://docs.langchain.com/oss/python/deepagents/context-engineering)


The fourth family is **progressive disclosure and hierarchical compression**. Anthropic now uses the phrase “progressive disclosure” both for Skills and for tool/code execution patterns where the model reads details on demand rather than front-loading everything. RAPTOR, GraphRAG, HippoRAG, A-Mem, and Graphiti all operationalize related ideas: build higher-level structure ahead of time, retrieve at the right granularity, and let the agent navigate from summaries to specifics only when needed.
* [Letting AI Actively Manage Its Own Context](https://blog.xlab.app/p/51d26495/)
* [Mem0](https://docs.mem0.ai/introduction)
* [Mem0 Memory Types](https://docs.mem0.ai/core-concepts/memory-types)
* [MemGPT Paper](https://arxiv.org/pdf/2310.08560)
* [Zep](https://help.getzep.com/graphiti/getting-started/overview)
* [Continue.dev Codebase Documentation Awareness](https://docs.continue.dev/guides/codebase-documentation-awareness)
* [Sourcegraph Cody](https://sourcegraph.com/blog/how-cody-provides-remote-repository-context)
* [Repomix](https://repomix.com/guide/)
* [Letta Blog](https://www.letta.com/blog)
  * [Letta Code Agent](https://github.com/letta-ai/letta-code)
  * [Letta Git-based Memory for Coding Agents](https://www.letta.com/blog/context-repositories)
  * [Letta: How to Build Agents that Learn and Remember](https://www.letta.com/blog/agent-memory)
  * [Letta: Memory Blocks](https://www.letta.com/blog/memory-blocks)
  * [Letta: Benchmarking LLMs on Agentic Context Engineering](https://www.letta.com/blog/context-bench)
  * [Letta: A guide to context engineering](https://www.letta.com/blog/guide-to-context-engineering)
* [Anthropic: Effective Harnesses for Long Running Agents](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents)

The fifth family is **precise forgetting / non-destructive pruning**. Pi’s new `context` hook makes it possible to modify only the message list sent to the model while leaving original session data untouched; `pi-context-prune` prunes summarized tool outputs from future context but preserves the originals behind a query tool; OpenCode DCP does something similar with placeholders, deduplication, and error purging; and vLLM’s hybrid KV design shows why sliding-window or truncation logic has to be designed carefully if you want to preserve prefix caching. This “forget in the prompt, not in storage” principle is one of the most important implementation findings in the whole space. citeturn18view0turn21search0turn20search0turn37search7

## Projects and implementations

The table below focuses on **high-confidence, directly relevant projects** for agentic context management in code-heavy environments.

| Project | What it implements | Why it matters for your use case |
|---|---|---|
| **Pi session tree** | Sessions stored as JSONL with `id`/`parentId`, enabling branching, leaf-based navigation, migration from linear logs, and tree-aware session replay. citeturn17view0 | This is one of the clearest open implementations of a **branchable session substrate** for coding agents, which is a strong foundation for precise forgetting and task-local rollback. citeturn17view0 |
| **pi-context** | Adds `context_checkpoint`, `context_timeline`, and `context_compact`; explicitly positions itself as “agentic context management” for Pi and cites Kimi D-Mail as inspiration. citeturn22view0 | Directly relevant to your request: checkpointing, timeline inspection, compacting noisy paths into handoff summaries, then continuing from earlier anchors. citeturn22view0 |
| **Kimi Code CLI / D-Mail** | `SendDMail` is documented as a delayed message for checkpoint rollback scenarios; changelog notes CHECKPOINT messages are only included when enabled; the codebase also stores subagent prompts/wire logs/context separately. citeturn15search0turn15search1turn15search10 | Kimi is the clearest real-world example of **time-travel handoff** in an agent loop, plus context isolation via subagents. citeturn22view1turn15search0turn15search10 |
| **Kimi RalphFlow PR** | Experimental “ephemeral context” flow architecture where iterations run in isolated temporary context files and merge back only when wanted. citeturn13search2 | Not a stable public feature, but a strong signal that Kimi’s maintainers are exploring **scratch-space isolation** rather than polluting the main session. citeturn13search2 |
| **pi-context-prune** | Summarizes completed tool-call batches, removes raw tool outputs from future LLM context, and preserves originals behind a `context_tree_query` escape hatch; supports an `agentic-auto` mode where the LLM decides when to prune. citeturn21search0turn21search2 | Strong example of **non-destructive, future-context-only pruning** tailored to long coding sessions. It also explicitly discusses cache effects. citeturn21search0 |
| **OpenCode Dynamic Context Pruning** | Replaces stale conversation spans with technical summaries before sending requests, supports range/message compression, deduplication, and error-input purging, while leaving session history intact. citeturn20search0 | Useful reference for **surgical pruning policies** beyond one-shot summarization, though it is not as code-structure-aware as the best repo-indexing systems. citeturn20search0 |
| **MemGPT / Letta** | Virtual context management with memory tiers; agent-editable, persisted memory blocks; compiled prompts from DB state; shared memory blocks; newer git-backed context repositories for coding agents. citeturn32search0turn30view0turn30view1 | Best-in-class for **agent-managed external memory** and one of the strongest conceptual foundations for long-running, stateful coding agents. citeturn32search0turn30view0turn30view1 |
| **LangChain Deep Agents / LangGraph** | Filesystem-backed memory, durable stores, subagents for “context quarantine,” checkpoints for time travel/debugging, and hybrid backends that route `/memories/` to persistent storage. citeturn36view0turn36view1turn36view2turn36view3 | Very relevant if you want a **framework-native architecture** for offloading large artifacts to files while preserving resumability and subtask isolation. citeturn36view0turn36view1turn36view3 |
| **Mem0** | Layered conversation/session/user/org memory; inferred or raw memory insertion; multi-agent shared memory examples. citeturn35view0turn35view1turn35view2turn35view3 | Strong turnkey option for **persistent memory across collaborating agents**, though less codebase-structural than source-code graph systems. citeturn35view1turn35view3 |
| **Graphiti / Zep** | Temporal context graphs with dynamic updates, hybrid retrieval, and point-in-time semantics; exposed via MCP. citeturn34view0turn34view1turn34view2 | Particularly valuable when code-context must be fused with **changing facts, incidents, tickets, docs, or business state** over time. citeturn34view0turn34view2 |
| **Aider** | Repo map built from tree-sitter definitions/references and graph ranking, sized to a token budget and tailored to current chat state. citeturn27view0turn27view1turn27view2 | One of the best concrete examples of **code-aware context compression without full-file retrieval**. Excellent fit for 96k–128k models. citeturn27view0turn27view1 |
| **Continue** | Context providers for files, code, repo maps, diffs, trees, terminal, debugger, docs, MCP; newer guidance pushes agent mode toward tools, rules, and MCP instead of monolithic providers. citeturn29view0turn29view1 | Useful reference for **composable context sources** and project-local rules that keep context narrow and structured. citeturn29view0turn29view1 |
| **Sourcegraph Cody** | Codebase context retrieval across local, repo, and remote repos; built on Sourcegraph indexes; PageRank-style ranking on the source graph/code symbol graph. citeturn28view0turn28view1turn28view2 | Strong evidence that **multi-repo code intelligence** is crucial for real coding agents. Particularly important for enterprise-scale or monorepo-adjacent work. citeturn28view0turn28view2 |

A second cluster is less about memory itself and more about **code-context selection**, which is essential if the agent works on large structured repos instead of plain text corpora.

| Project | Retrieval/indexing style | Relevance |
|---|---|---|
| **Open Aware** | MCP server with semantic search (`get_context`) and deep code research across pre-indexed repositories. citeturn26view0 | Useful as an example of **agent-facing code intelligence via MCP**, especially for cross-repo questions. citeturn26view0 |
| **AutoDocs** | Tree-sitter + SCIP parsing, dependency graph construction, repository-wide dependency-aware docs/search, MCP server. citeturn25search0turn9search18 | Strong for combining **documentation synthesis and structural code retrieval**. citeturn25search0turn9search18 |
| **code-review-graph** | Persistent local code graph, incremental updates, blast-radius analysis, MCP/CLI integration, token-reduction positioning. citeturn25search1 | Probably the closest open-source example of **surgical review-time context minimization** for coding assistants. citeturn25search1 |
| **Qartez MCP** | Precomputed repo knowledge graph with symbols, imports, call edges, PageRank, blast radius, git co-change, complexity, served through MCP. citeturn25search2 | Very aligned with your “code level strategies” requirement: this is essentially **knowledge-graph indexing for code agents**. citeturn25search2 |
| **Repomix** | Packs repositories into AI-friendly XML/Markdown/JSON/plain-text; supports MCP mode. citeturn11search0turn11search12turn11search18 | Useful baseline or fallback, but by itself it is **not** advanced agentic context management; it is more of a serialization layer than a memory strategy. citeturn11search12turn11search18 |

A final note on **OpenHands**: there is active public design and issue traffic around memory condensation and context condensing, but the available evidence is mostly issue discussions rather than polished docs. I would treat it as an important signal of the problem space, not yet as a clean reference implementation. citeturn3search1turn3search3turn3search10turn3search13

## Papers and research resources

The following papers and primary research resources are the most relevant conceptual anchors for the design space you described.

| Paper / resource | Core idea | Why it matters here |
|---|---|---|
| **MemGPT** | OS-inspired virtual context management with paging between in-context and external memory tiers. citeturn32search0 | Foundational paper for **memory tiering** and self-managed context. citeturn32search0 |
| **Reflexion** | Verbal self-reflection stored in episodic memory to improve future attempts. citeturn6search0turn7search3 | Important for **reasoning-aware memory**, not just retrieval. citeturn6search0turn7search3 |
| **Generative Agents** | Memory stream + reflection + retrieval to drive long-lived agent behavior. citeturn6search1 | Still one of the clearest architectures for **dynamic memory retrieval plus reflection**. citeturn6search1 |
| **Voyager** | Skill library and iterative prompting for lifelong task acquisition. citeturn6search2 | Valuable analogy for **externalizing reusable procedures** instead of keeping them in hot context. citeturn6search2 |
| **HiAgent** | Hierarchical working-memory management via subgoals; summarize prior subgoals and retain only current-relevant action/observation history. citeturn33search0 | Very close to your requirement for **hierarchical task decomposition plus precise forgetting**. citeturn33search0 |
| **A-Mem** | Dynamic, Zettelkasten-inspired agentic memory with indexing/linking. citeturn32search3 | Good fit for **agent-curated note graphs** rather than raw transcript memory. citeturn32search3 |
| **AgeMem** | Exposes memory operations as tools inside the agent policy; unified long- and short-term memory management. citeturn7search0turn7search4 | One of the strongest recent papers for making memory decisions part of the agent’s action space. citeturn7search0turn7search4 |
| **RAPTOR** | Recursive clustering/abstractive summaries organized as a retrieval tree. citeturn5search0turn5search8 | Useful for **hierarchical compression and progressive disclosure** over long docs and code-related corpora. citeturn5search0turn5search8 |
| **GraphRAG** | Graph-based retrieval that scales to broad, query-focused summarization over large corpora. citeturn4search6turn4search2 | Strong conceptual fit for **documentation and architectural retrieval** where entity relationships matter. citeturn4search6turn4search2 |
| **HippoRAG** | Long-term memory retrieval using a knowledge graph plus Personalized PageRank. citeturn4search7turn4search23 | Especially relevant to your interest in **knowledge-graph indexing** and efficient multi-hop retrieval. citeturn4search7turn4search23 |
| **LCM** | Recursive context compression + recursive task partitioning with lossless retrievability via immutable store plus active context. citeturn5search1turn5search5 | The cleanest recent paper directly about **lossless context management** for coding agents. citeturn5search1turn5search5 |
| **Contextual Memory Virtualisation** | DAG-based state management and structurally lossless trimming that removes mechanical bloat while preserving verbatim user/assistant content. citeturn5search9turn5search13 | Strong conceptual support for **non-destructive trimming** and provenance-preserving condensation. citeturn5search9turn5search13 |
| **SideQuest** | Model-driven KV-cache compression performed in a parallel auxiliary thread so cache-management tokens do not pollute the main reasoning trace. citeturn5search2 | Probably the most directly relevant research for **KV-aware long-horizon agentic reasoning**. citeturn5search2 |
| **LightMem** | Lightweight memory system modularizing retrieval, writing, and consolidation with separation between online processing and offline consolidation. citeturn4search17 | Good reference for **background consolidation** instead of doing all memory work inline. citeturn4search17 |
| **MemoryOS / MemOS** | Operating-system-style hierarchical memory design with different storage units and update policies. citeturn33search1turn33search5 | Useful if you want to think of memory as a **managed systems resource**, not just a vector store. citeturn33search1turn33search5 |
| **MIRIX / MemMachine** | Newer systems emphasizing modular memory types, high recall, and in MemMachine’s case, preserving full conversational episodes and contextualized retrieval. citeturn7search1turn7search5turn33search3 | Worth watching for evaluation and retrieval ideas, though they are less code-agent-specific than Aider/Pi/Kimi. citeturn7search1turn33search3 |

## Patterns that fit software projects and KV caching

For code agents, the best architecture is not “keep a giant chat and periodically summarize it.” It is **a layered working set**. Inferred from the projects above, that working set should have: a stable prefix containing system instructions, tool schemas, repo-level rules, and a compact codebase map; a volatile active slice containing the current task, current open symbols, and recent tool results; and a lossless backing store containing full session history, branch metadata, and recoverable artifacts. This is exactly the direction implied by Anthropic’s prompt-caching structure, vLLM’s prefix caching, Pi’s session tree, and Letta’s context repositories. citeturn37search2turn5search7turn17view0turn30view1

The most KV-cache-friendly design choice is to **keep the reusable prefix byte-for-byte stable** as often as possible. Anthropic recommends placing static content at the beginning of the prompt, and its cache hierarchy is ordered as `tools`, then `system`, then `messages`; vLLM’s automatic prefix caching reuses KV blocks when the same prefix is shared; and vLLM’s hybrid KV design explicitly avoids naïve sliding-window logic that would break prefix caching. In practice, this means you should not regenerate the entire repo summary, tool catalog, or memory overview every turn. Instead, keep them fixed and append small, task-local deltas. citeturn37search2turn5search3turn5search7turn37search7

For **code retrieval**, use symbol- and relation-level selection rather than file-level stuffing. Aider shows how far a token-budgeted repo map can go with only key symbols and graph-ranked files; Sourcegraph shows the value of large-scale code indexing plus PageRank-like ranking on a source graph; Continue exposes providers such as `@Code`, `@Search`, `@Tree`, and `@Repository Map`; and specialized MCP servers such as AutoDocs, code-review-graph, and Qartez push even further into AST/SCIP/graph territory. The same logic can be implemented with LSP primitives—document symbols, references, call hierarchy, semantic tokens—when you control the editor/runtime. citeturn27view0turn27view1turn28view0turn28view2turn29view1turn25search0turn25search1turn25search2turn9search0turn9search4

For **subtasks**, the cleanest pattern is “delegate, write to file/store, return only a compact synthesis.” LangChain Deep Agents says exactly this: subagents keep the main agent’s context clean, and large results should go to the filesystem for later selective reads. Kimi’s subagent storage layout and Pi’s context-transform hooks support the same practice from a different direction. This is a far better fit for coding work than letting every grep, test log, patch attempt, and failed hypothesis stay in the main prompt. citeturn36view1turn36view0turn15search10turn18view0

For **precise forgetting**, prefer *stored policies plus recoverable originals* over destructive rewriting. Pi’s `context` event, `pi-context-prune`, and OpenCode DCP all preserve the original session and only alter the future request context. LCM and Contextual Memory Virtualisation reinforce the same principle theoretically. This is the right pattern when you want to prune stale tool outputs, old exploratory branches, or superseded plans without losing provenance. citeturn18view0turn21search0turn20search0turn5search5turn5search13

A practical architecture for your target setting would therefore look like this, as an engineering synthesis from the sources above. The **hot context** should contain: stable system prompt, tool schemas, repo rules, compact repo map, current plan, recent interaction tail. The **warm layer** should contain: per-task files, subagent result files, branch summaries, decision logs, symbol references, and doc snippets. The **cold layer** should contain: full session tree, full tool outputs, code graph index, temporal memory graph, vector/BM25 stores, and optional external KV storage. Anthropic’s progressive disclosure model and vLLM’s Unified Cache Manager both support this style of tiering: the former at the prompt level, the latter at the KV-storage level. citeturn37search0turn37search9turn37search18turn37search3

## Recommended stack and design implications

If I were designing an **agentic context manager for large software projects today**, I would not copy any one project wholesale. I would combine a few specific ideas.

Use **Pi/Kimi-style checkpointed session control** for lossless branching, rollback, and handoff summaries. Pi already gives you a tree-structured session format and extensibility around context transforms; Kimi shows a more explicit “send a message to your past self” mechanism. This is the best answer I found to your requirement around splitting tasks into subtasks, cleaning up after them, and still being able to recover old reasoning when needed. citeturn17view0turn22view0turn15search0turn22view1

Use **Letta or Deep Agents-style externalized memory** for anything durable: architecture notes, coding conventions, user/team preferences, learned repair patterns, unresolved hypotheses, and reusable procedures. The most promising concrete representations are either filesystem-backed memory files or git-backed context repositories, because they are transparent, diffable, scriptable, and easy for agents to inspect incrementally. citeturn30view1turn36view0turn36view2

Use **Aider/Sourcegraph/Qartez-style code intelligence** as the default retrieval layer, not raw file reads. For a coding agent, the best “compression” is often not summarization at all—it is simply retrieving the right function signatures, callers, imports, tests, and related docs in the first place. This is where LSP, tree-sitter, SCIP, and graph ranking matter most. citeturn27view0turn27view1turn28view2turn25search0turn25search1turn25search2turn9search0turn9search4

Use **subagents as context firebreaks**. Delegate broad exploration, long test logs, web/doc research, or speculative patch attempts to subagents; force those workers to emit either file artifacts or short structured summaries; and only promote distilled findings into the coordinator’s hot context. This is one of the few strategies that both scales well and remains cache-friendly. citeturn36view1turn36view0turn15search10

Finally, adopt a **cache discipline** at the prompt-construction layer. Keep tools/system/rules/repo-map order stable; avoid rewriting old messages into a brand-new mega-summary every turn; place summaries or branch handoffs in warm storage rather than replacing the shared prefix; and, if you self-host inference, consider external KV storage or automatic prefix caching support from vLLM/UCM. citeturn37search2turn5search3turn5search7turn37search3

The main limitation in the current ecosystem is that the most interesting ideas are split across different projects. Pi/Kimi are strong on session control but not the strongest on code intelligence; Aider/Sourcegraph are excellent on code selection but weaker on long-lived agent memory; Letta and Deep Agents are strong on memory architecture but not specifically optimized for code-symbol retrieval; Graphiti is strong on temporal graph memory but not a turn-key coding harness. Also, some promising systems remain immature or partly documented through issues and experiments rather than stable public APIs, especially OpenHands-style condensation and some newer plugin ecosystems. citeturn22view0turn15search0turn27view0turn28view0turn30view1turn36view2turn34view0turn3search1turn3search3

Open questions remain. There is still limited public benchmarking on **coding-specific long-horizon context management under strict KV-cache constraints**; many performance claims from newer tooling are self-reported; and there is not yet a widely accepted benchmark that jointly measures code retrieval quality, branch/rewind fidelity, cache preservation, and recovery of pruned state after hierarchical task decomposition. The underlying ingredients are now visible, but the “one best system” is still emerging. citeturn37search1turn5search2turn8search15turn33search3turn8search12
