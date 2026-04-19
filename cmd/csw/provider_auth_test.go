package main

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderAuthCommand_Success(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "csw-provider-auth-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	tmpHome, err := os.MkdirTemp("", "csw-home-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpHome)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(oldDir)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	mock := testutil.NewMockHTTPServer()
	defer mock.Close()
	mock.AddRestResponse("/oauth/token", "POST", `{"access_token":"new-access-token","refresh_token":"new-refresh-token","expires_in":3600}`)

	providerConfig := &conf.ModelProviderConfig{
		Name:        "openai-auth",
		Type:        "openai",
		URL:         "https://chatgpt.com/backend-api/codex/responses",
		AuthURL:     mock.URL() + "/oauth/authorize",
		TokenURL:    mock.URL() + "/oauth/token",
		ClientID:    "client-id",
		AuthMode:    conf.AuthModeOAuth2,
		APIKey:      "",
		Headers:     map[string]string{"originator": "opencode"},
		QueryParams: map[string]string{},
	}
	err = models.SaveProviderConfigToModelsDir(providerConfig, filepath.Join(tmpDir, ".csw", "config", "models"))
	require.NoError(t, err)

	originalPort := providerAuthPort
	originalTimeout := providerAuthTimeout
	providerAuthPort = getFreePortNumber(t)
	providerAuthTimeout = 15 * time.Second
	defer func() {
		providerAuthPort = originalPort
		providerAuthTimeout = originalTimeout
	}()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	lineCh := make(chan string, 32)
	scanDone := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			lineCh <- scanner.Text()
		}
		close(scanDone)
		close(lineCh)
	}()

	cmd := providerAuthCommand()
	cmd.SetArgs([]string{"openai-auth"})

	errCh := make(chan error, 1)
	go func() {
		errCh <- cmd.Execute()
		_ = w.Close()
	}()

	authURL, state := readAuthURLAndStateFromOutput(t, lineCh)
	require.NotEmpty(t, authURL)
	require.NotEmpty(t, state)

	callbackURL := fmt.Sprintf("http://localhost:%d%s?code=auth-code-123&state=%s", providerAuthPort, providerAuthCallbackPath, url.QueryEscape(state))
	sendCallbackRequestWithRetryForProviderTest(t, callbackURL)

	select {
	case execErr := <-errCh:
		require.NoError(t, execErr)
	case <-time.After(20 * time.Second):
		t.Fatalf("TestProviderAuthCommand_Success() [provider_auth_test.go]: timeout waiting for auth command")
	}

	<-scanDone

	req := mock.GetRequests()
	require.Len(t, req, 1)
	assert.Equal(t, "/oauth/token", req[0].Path)
	assert.Contains(t, string(req[0].Body), "grant_type=authorization_code")
	assert.Contains(t, string(req[0].Body), "code=auth-code-123")
	assert.Contains(t, string(req[0].Body), "client_id=client-id")
	assert.Contains(t, string(req[0].Body), "code_verifier=")

	storeAfter, err := GetConfigStore(ConfigScopeGlobal)
	require.NoError(t, err)
	defer func() {
		if closer, ok := storeAfter.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	}()

	configs, err := storeAfter.GetModelProviderConfigs()
	require.NoError(t, err)
	updated, exists := configs["openai-auth"]
	require.True(t, exists)
	assert.Equal(t, conf.AuthModeOAuth2, updated.AuthMode)
	assert.Equal(t, "new-access-token", updated.APIKey)
	assert.Equal(t, "new-refresh-token", updated.RefreshToken)
}

func TestProviderAuthCommand_ClearsPreviousAuthDataBeforeReauth(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "csw-provider-auth-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	tmpHome, err := os.MkdirTemp("", "csw-home-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpHome)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(oldDir)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	mock := testutil.NewMockHTTPServer()
	defer mock.Close()
	mock.AddRestResponse("/oauth/token", "POST", `{"access_token":"new-access-token","expires_in":3600}`)

	providerConfig := &conf.ModelProviderConfig{
		Name:         "openai-auth",
		Type:         "openai",
		URL:          "https://chatgpt.com/backend-api/codex/responses",
		AuthURL:      mock.URL() + "/oauth/authorize",
		TokenURL:     mock.URL() + "/oauth/token",
		ClientID:     "client-id",
		AuthMode:     conf.AuthModeOAuth2,
		APIKey:       "stale-access-token",
		RefreshToken: "stale-refresh-token",
		Headers:      map[string]string{"originator": "opencode"},
		QueryParams:  map[string]string{},
	}
	err = models.SaveProviderConfigToModelsDir(providerConfig, filepath.Join(tmpDir, ".csw", "config", "models"))
	require.NoError(t, err)

	originalPort := providerAuthPort
	originalTimeout := providerAuthTimeout
	providerAuthPort = getFreePortNumber(t)
	providerAuthTimeout = 15 * time.Second
	defer func() {
		providerAuthPort = originalPort
		providerAuthTimeout = originalTimeout
	}()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	lineCh := make(chan string, 32)
	scanDone := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			lineCh <- scanner.Text()
		}
		close(scanDone)
		close(lineCh)
	}()

	cmd := providerAuthCommand()
	cmd.SetArgs([]string{"openai-auth"})

	errCh := make(chan error, 1)
	go func() {
		errCh <- cmd.Execute()
		_ = w.Close()
	}()

	_, state := readAuthURLAndStateFromOutput(t, lineCh)
	require.NotEmpty(t, state)

	callbackURL := fmt.Sprintf("http://localhost:%d%s?code=auth-code-456&state=%s", providerAuthPort, providerAuthCallbackPath, url.QueryEscape(state))
	sendCallbackRequestWithRetryForProviderTest(t, callbackURL)

	select {
	case execErr := <-errCh:
		require.NoError(t, execErr)
	case <-time.After(20 * time.Second):
		t.Fatalf("TestProviderAuthCommand_ClearsPreviousAuthDataBeforeReauth() [provider_auth_test.go]: timeout waiting for auth command")
	}

	<-scanDone

	storeAfter, err := GetConfigStore(ConfigScopeGlobal)
	require.NoError(t, err)
	defer func() {
		if closer, ok := storeAfter.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	}()

	configs, err := storeAfter.GetModelProviderConfigs()
	require.NoError(t, err)
	updated, exists := configs["openai-auth"]
	require.True(t, exists)
	assert.Equal(t, conf.AuthModeOAuth2, updated.AuthMode)
	assert.Equal(t, "new-access-token", updated.APIKey)
	assert.Empty(t, updated.RefreshToken)
}

func readAuthURLAndStateFromOutput(t *testing.T, lineCh <-chan string) (string, string) {
	t.Helper()

	deadline := time.After(10 * time.Second)
	for {
		select {
		case line, ok := <-lineCh:
			if !ok {
				t.Fatalf("readAuthURLAndStateFromOutput() [provider_auth_test.go]: output stream closed before auth URL was printed")
			}
			if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
				parsed, err := url.Parse(line)
				require.NoError(t, err)
				state := parsed.Query().Get("state")
				return line, state
			}
		case <-deadline:
			t.Fatalf("readAuthURLAndStateFromOutput() [provider_auth_test.go]: timeout waiting for auth URL in output")
		}
	}
}

func sendCallbackRequestWithRetryForProviderTest(t *testing.T, callbackURL string) {
	t.Helper()

	var lastErr error
	for i := 0; i < 100; i++ {
		resp, err := http.Get(callbackURL)
		if err == nil {
			_ = resp.Body.Close()
			return
		}
		lastErr = err
		time.Sleep(20 * time.Millisecond)
	}

	require.NoError(t, lastErr)
}

func getFreePortNumber(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok)
	return addr.Port
}

func TestProviderAuthHelpers_Defaults(t *testing.T) {
	originalPort := providerAuthPort
	defer func() {
		providerAuthPort = originalPort
	}()

	providerAuthPort = 1455

	assert.Equal(t, "127.0.0.1:1455", providerAuthListenAddress())
	assert.Equal(t, "http://localhost:1455/auth/callback", providerAuthRedirectURI())
}
