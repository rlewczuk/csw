package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/spf13/cobra"
)

func providerShowCommand(useJSON *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "show <provider-name>",
		Short: "Show details of a provider from composite configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			providerName := args[0]

			store, err := GetCompositeConfigStore()
			if err != nil {
				return err
			}

			configs := store.ModelProviderConfigs

			config, exists := configs[providerName]
			if !exists {
				return fmt.Errorf("providerShowCommand() [provider_show.go]: provider not found: %s", providerName)
			}

			if *useJSON {
				return outputJSON(config)
			}

			return outputProviderDetails(config)
		},
	}
}

func outputProviderDetails(config *conf.ModelProviderConfig) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "PROPERTY\tVALUE")
	fmt.Fprintf(w, "Name\t%s\n", config.Name)
	fmt.Fprintf(w, "Type\t%s\n", config.Type)
	if config.Description != "" {
		fmt.Fprintf(w, "Description\t%s\n", config.Description)
	}
	fmt.Fprintf(w, "URL\t%s\n", config.URL)
	if config.AuthURL != "" {
		fmt.Fprintf(w, "Auth URL\t%s\n", config.AuthURL)
	}
	if config.APIKey != "" {
		fmt.Fprintf(w, "API Key\t%s\n", maskAPIKey(config.APIKey))
	}
	if config.ConnectTimeout > 0 {
		fmt.Fprintf(w, "Connect Timeout\t%s\n", config.GetConnectTimeoutDuration())
	}
	if config.RequestTimeout > 0 {
		fmt.Fprintf(w, "Request Timeout\t%s\n", config.GetRequestTimeoutDuration())
	}
	if len(config.Headers) > 0 {
		for name, value := range config.Headers {
			fmt.Fprintf(w, "Header %s\t%s\n", name, maskHeaderValue(name, value))
		}
	}

	return nil
}
