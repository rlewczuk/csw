package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/rlewczuk/csw/pkg/models"
	"github.com/spf13/cobra"
)

func providerModelsCommand(useJSON *bool) *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:   "models [<provider>]",
		Short: "List models for a provider or all providers",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := GetCompositeConfigStore()
			if err != nil {
				return err
			}

			configs, err := store.GetModelProviderConfigs()
			if err != nil {
				return fmt.Errorf("providerModelsCommand() [provider_models.go]: failed to get provider configs: %w", err)
			}

			var targetProviders []string
			if len(args) == 1 {
				providerName := args[0]
				if _, exists := configs[providerName]; !exists {
					fmt.Fprintf(os.Stderr, "Error: provider not found: %s\n", providerName)
					os.Exit(1)
				}
				targetProviders = []string{providerName}
			} else {
				for name := range configs {
					targetProviders = append(targetProviders, name)
				}
			}

			var modelsList []modelEntry
			for _, providerName := range targetProviders {
				config := configs[providerName]
				provider, err := models.ModelFromConfig(config)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to create provider %s: %v\n", providerName, err)
					continue
				}

				provider.SetVerbose(verbose)
				modelInfos, err := provider.ListModels()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to list models for provider %s: %v\n", providerName, err)
					continue
				}

				for _, modelInfo := range modelInfos {
					modelsList = append(modelsList, modelEntry{
						Provider: providerName,
						Model:    modelInfo.Name,
					})
				}
			}

			if *useJSON {
				return outputJSON(modelsList)
			}

			return outputModelsList(modelsList)
		},
	}

	cmd.Flags().BoolVar(&verbose, "verbose", false, "Print raw request and response headers")

	return cmd
}

func outputModelsList(modelsList []modelEntry) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "PROVIDER\tMODEL")
	for _, entry := range modelsList {
		fmt.Fprintf(w, "%s\t%s\n", entry.Provider, entry.Model)
	}

	return nil
}
