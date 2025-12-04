# Golang Tech Stack

* Go 1.25.x;
* github.com/go-git/go-git
* github.com/spf13/cobra, github.com/spf13/viper
* github.com/stretchr/testify
* charm.land/bubbletea/v2  - TUI library
* charm.land/bubbles/v2   - components for bubbletea
* charm.land/fantasy   - library for building AI agents
* github.com/charmbracelet/crush - przykładowa aplikacja, punkt wyjścia
* github.com/charmbracelet/catwalk - lista providerów jaką używa crush
* github.com/charmbracelet/lipgloss - styling dla tekstowych UI;

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
* TBD - grepowanie drzewa z kodem, przeszukiwanie semantyczne;
* 







# WDR - LSP interface

I'm looking for golang library to interact with LSP servers. It should be able to:
* start LSP server for given project root;
* manage LSP server process, including stopping and restarting when needed;
* connect to LSP server;
* send requests and receive responses;
* handle notifications;
* handle errors;
* (optional) download and start LSP server - either locally or in containers;



# WDR - Wirtual Development Runtime

I'm looking for golang library that will allow me to run and control a process sandboxed in a container. 
It should also be able to:
* control resource usage of the container (so it won't exceed specified limits); 
* process input and output (either to just receive result or emulate console);
* stop, restart and destroy container;
* map container ports to host ports;
* map local directories into container (volumes);
* (optional) provide means to map commit of a git repository as a volume inside container with ability to save changes back to host filesystem as a commit;
* (optional) limit network access of the container (eg. only allow access to localhost or specific ports or specific addresses in the internet);

Library can either rely on a popular container runtime or implement its own but it should be able to run as user, not requiring root access.
