package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/spf13/cobra"
)

func providerListCommand(useJSON *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all providers from all configuration paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := GetCompositeConfigStore()
			if err != nil {
				return err
			}

			configs, err := store.GetModelProviderConfigs()
			if err != nil {
				return fmt.Errorf("providerListCommand() [provider_list.go]: failed to get provider configs: %w", err)
			}

			if *useJSON {
				return outputJSON(configs)
			}

			return outputProviderList(configs)
		},
	}
}

func outputProviderList(configs map[string]*conf.ModelProviderConfig) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "NAME\tTYPE\tDESCRIPTION")
	for name, config := range configs {
		description := config.Description
		if description == "" {
			description = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", name, config.Type, description)
	}

	return nil
}
