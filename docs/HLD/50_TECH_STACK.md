# Golang Tech Stack

* Go 1.25.x;
* github.com/go-git/go-git
* github.com/spf13/cobra, github.com/spf13/viper
* github.com/stretchr/testify
* charm.land/bubbletea/v2  - TUI library - MIT
* charm.land/bubbles/v2   - components for bubbletea - MIT
* github.com/charmbracelet/catwalk - lista providerów jaką używa crush - MIT
* github.com/charmbracelet/lipgloss - styling dla tekstowych UI - MIT
* github.com/charmbracelet/ultraviolet - terminal interface primitives (podstawowe)

* container runtime management (dla różnych runtimes):
  * github.com/runc/libcontainer - 
  * containerd - 
  * github.com/docker/docker/client - 
  * podman client - 
  * ?? ciryle/go-sandbox
  * ?? seccomp
  * ?? RootlessKit
  * KataContainers
* LSP interface:
  * github.com/sourcegraph/go-lsp - ale to jest przeterminowane
  * go.lsp.dev/protocol - struktury danych gotowe, resztę trzeba sobie dopisać
  * go.bugst/lsp - ??
  * golang.org/x/tools/gopls/internal/lsprpc - klient wewnętrzny gopls
* agentic library:
  * github.com/tmc/langchaingo
  * charm.land/fantasy   - library for building AI agents - Apache 2.0
* LLM client library
  * gollm
  * lexlapax/go-llms
  * langhaingo/llms
  * go-llmbridge
* TBD - grepowanie drzewa z kodem, przeszukiwanie semantyczne;
* 




# WDR

## Compare agentic libraries

Compare charmbracelet/fantasy and langchain-go agent libraries for golang. Focus on following things:
* complexity of libraries 
* complexity of development code using those libraries;
* testability of code using those libraries;
* ability to run parallel sessions;
* ability to handle more complex tasks (i.e. splitting tasks into smaller parts);
* logging of conversation and LLM communication;
* handling LLM communication errors, including rate limiting, failover to alternative providers etc.;
* handling LLM cost tracking;

## Agentic interface

I'm looking for golang library to build agentic applications. It should be able to:
* manage conversation context (system, user and assistant messages);
* periodically prune conversation context;
* define and use tools;
* run parallel sessions;
* support either local or remote LLM providers;
* support multiple LLM providers;
* provide test mocks, so that client code can be tested easily;

## LLM interface

I'm looking for golang library to interact with LLM providers. It should be able to:
* list available models;
* query model with streaming;
* properly handle rate limiting (eg. by pausing or letting client code provide custom handler);
* (optional) gracefully handle errors (incl. retries, failover to alternative providers etc.);
* authenticate using API keys;
* configure model;
* (optional)track cost;
* support multiple providers (eg. openai, anthropic, google, local models etc.);
* provide test mocks, so that client code can be tested without actual LLM calls;


## LSP interface

I'm looking for golang library to interact with LSP servers. It should be able to:
* start LSP server for given project root;
* manage LSP server process, including stopping and restarting when needed;
* connect to LSP server;
* send requests and receive responses;
* handle notifications;
* handle errors;
* (optional) download and start LSP server - either locally or in containers;



## Wirtual Development Runtime

I'm looking for javascript/typescript library that will allow me to run and control a process sandboxed in a container. 
It should also be able to:
* control resource usage of the container (so it won't exceed specified limits); 
* process input and output (either to just receive result or emulate console);
* stop, restart and destroy container;
* map container ports to host ports;
* map local directories into container (volumes);
* (optional) provide means to map commit of a git repository as a volume inside container with ability to save changes back to host filesystem as a commit;
* (optional) limit network access of the container (eg. only allow access to localhost or specific ports or specific addresses in the internet);

Library can either rely on a popular container runtime or implement its own but it should be able to run as user, not requiring root access.


# Imagen

Generate a logo for application named `Codesnort` that is a software development agent.
It should be humorous, for example a pig snorting through heaps of code in various programming languages.
Pig should have a big nose, pretty much like in the Sourcefire Snort logo.
Be sure that logo nor mascot does not violate any trademarks or copyrights and differs ehough from Sourcefire Snort logo to not be confused with it.
Make sure pig looks funny.
