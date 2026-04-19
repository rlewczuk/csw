package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/spf13/cobra"
)

var (
	providerAuthPort         = 1455
	providerAuthCallbackPath = "/auth/callback"
	providerAuthTimeout      = 5 * time.Minute
	providerAuthExtraParams  = map[string]string{
		"id_token_add_organizations": "true",
		"codex_cli_simplified_flow":  "true",
		"originator":                 "opencode",
	}
)

func providerAuthCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "auth <provider-name>",
		Short: "Authenticate a provider via browser OAuth2 flow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			providerName := args[0]

			store, closeFn, err := findWritableStoreForProvider(providerName)
			if err != nil {
				return err
			}
			if store == nil {
				return fmt.Errorf("providerAuthCommand() [provider_auth.go]: provider not found: %s", providerName)
			}
			if closeFn != nil {
				defer closeFn()
			}

			configs, err := store.GetModelProviderConfigs()
			if err != nil {
				return fmt.Errorf("providerAuthCommand() [provider_auth.go]: failed to get provider configs: %w", err)
			}

			config, exists := configs[providerName]
			if !exists {
				return fmt.Errorf("providerAuthCommand() [provider_auth.go]: provider not found: %s", providerName)
			}

			config.APIKey = ""
			config.RefreshToken = ""
			if err := models.SaveProviderConfigToUserModelsDir(config); err != nil {
				return fmt.Errorf("providerAuthCommand() [provider_auth.go]: failed to clear previous provider auth data: %w", err)
			}

			pkce, err := models.GenerateOAuthPKCECodes()
			if err != nil {
				return fmt.Errorf("providerAuthCommand() [provider_auth.go]: failed to generate PKCE codes: %w", err)
			}

			state, err := models.GenerateOAuthState()
			if err != nil {
				return fmt.Errorf("providerAuthCommand() [provider_auth.go]: failed to generate OAuth state: %w", err)
			}

			redirectURI := providerAuthRedirectURI()
			authURL, err := models.BuildAuthorizationURL(
				config,
				redirectURI,
				state,
				pkce.Challenge,
				models.DefaultOAuthScope,
				providerAuthExtraParams,
			)
			if err != nil {
				return fmt.Errorf("providerAuthCommand() [provider_auth.go]: failed to build authorization URL: %w", err)
			}

			fmt.Printf("Open this link in your browser to authenticate provider '%s':\n%s\n", providerName, authURL)
			fmt.Printf("Waiting for callback on %s ...\n", redirectURI)

			ctx, cancel := context.WithTimeout(context.Background(), providerAuthTimeout)
			defer cancel()

			callback, err := models.WaitForOAuthCallback(ctx, providerAuthListenAddress(), providerAuthCallbackPath)
			if err != nil {
				return fmt.Errorf("providerAuthCommand() [provider_auth.go]: failed waiting for OAuth callback: %w", err)
			}

			if callback.State != state {
				return fmt.Errorf("providerAuthCommand() [provider_auth.go]: invalid state returned by callback")
			}

			httpClient := &http.Client{Timeout: 30 * time.Second}
			if config.RequestTimeout > 0 {
				httpClient.Timeout = config.RequestTimeout
			}

			tokenResp, err := models.ExchangeAuthorizationCode(config, httpClient, callback.Code, redirectURI, pkce.Verifier)
			if err != nil {
				return fmt.Errorf("providerAuthCommand() [provider_auth.go]: failed to exchange authorization code: %w", err)
			}

			config.AuthMode = conf.AuthModeOAuth2
			config.APIKey = tokenResp.AccessToken
			if tokenResp.RefreshToken != "" {
				config.RefreshToken = tokenResp.RefreshToken
			}

			if err := models.SaveProviderConfigToUserModelsDir(config); err != nil {
				return fmt.Errorf("providerAuthCommand() [provider_auth.go]: failed to save provider config: %w", err)
			}

			fmt.Printf("Provider '%s' authenticated successfully\n", providerName)
			return nil
		},
	}
}

func providerAuthListenAddress() string {
	return fmt.Sprintf("127.0.0.1:%d", providerAuthPort)
}

func providerAuthRedirectURI() string {
	return fmt.Sprintf("http://localhost:%d%s", providerAuthPort, providerAuthCallbackPath)
}
