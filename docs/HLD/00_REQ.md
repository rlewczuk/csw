
# Product Description

Codesnort SWE is an advanced software development agent designed to perform complex tasks on big codebases. It supports various styles of structured AI assisted software development including but not confined to SDD (spec driven development). 

Functional Requirements:

* **F.1** working on tasks of arbitrary complexity on all development phases (analysis, specification, design, planning, implementation, testing, debugging, deployment), including:
  * intelligent splitting of tasks into smaller ones, 
  * spanning big task onto multiple sessions, 
  * using appropriate tools and roles to perform various parts of task (eg. architect, coder, tester etc.);
  * intelligently parallelizing certain subtasks or steps if possible;
  * working interactively with developer on certain steps if needed;
  * developer should be able to intercept and impact planned tasks (eg. skipping steps, manually adding steps, marking steps done, retrying steps etc.);
* **F.2** supporting all roles in software development that AI can do: architect, developer, documenter, tester etc.
  * various roles can have not only various system prompts but also various tool sets available and various permissions (eg. writing only .md files for architect, or accessing only files in given path);
* **F.3** enforcing specified rules when designing or developing components, features or functionalities;
* **F.4** providing predefined custom actions that are mapped as tools and can be used by Model or user (eg. build app, run all tests, run specific test, clean build artifacts, run app, restart app etc.)
* **F.5** providing predefined recipes for performing certain tasks or steps, consisting of prompts, templates and other paremeters;
  * for example SDD process should be implemented as set of recipes, not hardcoded;
* **F.6** providing predefined blueprints -- sets of files, rules, supporting prompts etc. for quick set up of a development environment for certain software stack enforcing certain policies:
  * blueprints should be composable, eg. developer should be able to choose base software stack, design language, organizational policies etc.
* **F.7** agent should integrate with all popular IDEs (Jetbrains, VSCode, Zed) as well as work standalone (by providing TUI or WebUI); also integration via established protocols such as ACP (Agent Client Protocol) should be supported;
  * user interface should be friendly for developer to issue more complicated tasks or decisions (better than small chat box);
* **F.8** agent should provide a set of tools for Model to use and decide which one should be available in certain context:
  * internal tools (managing files locally, managing tasks and overall workflow etc.);
  * tools available from IDEs with which it integrates (via ACP or MCP for Jetbrains);
  * third party tools via MCP protocol;
  * secure execution of third party tools (eg. in isolated containers);
  * intelligent selection which tools to expose to an Model for given subtask in given context;
* **F.9** agent should be able to access various Model providers, including OpenAI, Anthropic, Google, local models (Ollama, LMStudio, vllm, sglang), deepseek, qwen, minimax:
  * various models from various providers should be used depending on role and context;
* **F.10** agent should provide secure way for build and execution of developed application and tests:
  * agent should provide minimum surface for each task, eg. build does not need to see any files containing credentials;
* **F.11** agent should provide way to run applications or build on remote environment (eg. k8 cluster);
* **F.12** agent should maintain view of application structure so that it can navigate and operate over it:
  * existing tools should be used if possible (eg. LSPs, securely executed);
  * AI assistent navigation can be used in addition to existing tools;
* **F.13** agent should provide means to roll back performed steps if process goes wrong way (checkpoints);
  * use existing tools to implement checkpoints (i.e. git);
* **F.14** agen should be able to work in non-interactive mode, so it can be called from CI or to resolve issues in automated way;

# Glossary

## Action

Predefined command mapped as a tool (for example for building application or running tests). It relieves Model from generating command which make it more predictable and safer.

## Blueprint

Blueprint is a set of commands, files, scripts, rules, actions, prompts, recipes, templates, technology stack dependencies and their integration, together making a framework (or part of) for a software application being developed. Blueprints allow agent to avoid guessing and unnecessary work related to set up all components of a technology stack, then guiding agent how to work on a project and enforcing certain architectural decisions.

## Recipe

Recipe is a set of prompts, templates and other components bound to a command available via UI. Can contain subsequent prompts using results of previous ones.

## Role

Role determines what kind of task agent will perform in a given context. It determines system prompt, rules, available tools and privileges.

## Session

Session is a context initiated by developer (or agent) in which given part of task is executed. Session is associated with Model context, consisting of system message, user and assistant messages, where new conversation is added at the end and context being periodically pruned.

## Task

Task is a set of operations, steps, subtasks etc. realizing a functionality (or subset of) as envisioned by product developer. Tasks have hierarchical structure of arbitrary depth, each node of it being managed by agent (i.e. having unique ID, description, status, attributes, parallelism), so that agent can decide which one can be executed next.

## Version Control System (VCS)

