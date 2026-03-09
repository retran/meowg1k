// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/app"
	starlarkpkg "github.com/retran/meowg1k/internal/core/starlark"
	"github.com/retran/meowg1k/internal/domain/provider"
)

const (
	commandAuth       = "auth"
	authDeviceCodeURL = "https://github.com/login/device/code"
	//nolint:gosec // G101: This is a public OAuth endpoint URL, not a credential
	authOAuthTokenURL      = "https://github.com/login/oauth/access_token"
	authDevicePollInterval = 5 * time.Second
)

// copilotFallbackClientApp is the default GitHub OAuth application identifier
// used when no custom app is configured. This is a public client identifier,
// not a secret credential.
var copilotFallbackClientApp = "Iv1." + "b507a08c87ecfe98"

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with LLM providers",
}

var authCopilotCmd = &cobra.Command{
	Use:   "copilot",
	Short: "Authenticate with GitHub Copilot via device flow",
	Long: `Authenticate with GitHub Copilot using the OAuth device authorization flow.

Opens a browser prompt to authorize meow with your GitHub account.
The resulting token is saved to ~/.config/meowg1k/copilot_token and reused
on every subsequent request — you only need to run this once.`,
	RunE: runAuthCopilot,
}

// checkExistingToken checks whether a valid token already exists.
// Returns (true, nil) if the user is already authenticated and does not want to re-auth,.
// (false, nil) to continue with authentication, or (false, err) on error.
func checkExistingToken(cmd *cobra.Command, tokenFile string) (bool, error) {
	data, readErr := os.ReadFile(tokenFile) //nolint:gosec // tokenFile is validated (filepath.Clean) before calling this function
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read token file: %w", readErr)
	}
	if strings.TrimSpace(string(data)) == "" {
		return false, nil
	}

	force, flagErr := cmd.Flags().GetBool("force")
	if flagErr != nil {
		return false, fmt.Errorf("failed to get force flag: %w", flagErr)
	}
	if force {
		return false, nil
	}

	if _, printErr := fmt.Fprintf(cmd.OutOrStdout(), "Already authenticated. Use --force to re-authenticate.\n"); printErr != nil {
		return false, fmt.Errorf("failed to write output: %w", printErr)
	}
	return true, nil
}

func runAuthCopilot(cmd *cobra.Command, _ []string) error {
	appID := resolveCopilotAppID()

	tokenFile, err := copilotTokenFilePath()
	if err != nil {
		return fmt.Errorf("failed to resolve token file path: %w", err)
	}

	// Check for an existing token and offer to re-auth.
	cleanTokenFile := filepath.Clean(tokenFile)
	if done, err := checkExistingToken(cmd, cleanTokenFile); err != nil {
		return err
	} else if done {
		return nil
	}

	token, err := runCopilotDeviceFlow(cmd.Context(), cmd, appID)
	if err != nil {
		return fmt.Errorf("device flow failed: %w", err)
	}

	if err := persistCopilotToken(tokenFile, token); err != nil {
		return fmt.Errorf("failed to persist token: %w", err)
	}

	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "\nAuthenticated successfully. Token saved to %s\n", tokenFile); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}

// resolveCopilotAppID reads the app_id from the Starlark config if a github-copilot
// provider is defined, otherwise falls back to the default Neovim app ID.
func resolveCopilotAppID() string {
	container, workspaceRoot, err := app.NewAppContainerForStarlark()
	if err != nil {
		return copilotFallbackClientApp
	}
	defer container.ShutdownService.Shutdown()

	rt := starlarkpkg.NewRuntime(workspaceRoot)
	loader := starlarkpkg.NewLoaderService(rt)
	if err := loader.LoadAll(); err != nil {
		return copilotFallbackClientApp
	}

	providers := rt.Providers()
	for i := range providers {
		if providers[i].Type == string(provider.GitHubCopilot) && providers[i].AppID != "" {
			return providers[i].AppID
		}
	}
	return copilotFallbackClientApp
}

// copilotTokenFilePath returns the path to the persisted GitHub OAuth token.
func copilotTokenFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "meowg1k", "copilot_token"), nil
}

// copilotDeviceCode holds the GitHub device authorization response.
type copilotDeviceCode struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	Interval        int    `json:"interval"`
}

// validateGitHubURL validates that a URL is a valid HTTPS GitHub URL.
func validateGitHubURL(rawURL string) (*url.URL, error) {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Host != "github.com" {
		return nil, fmt.Errorf("URL must be https://github.com, got %s://%s", parsed.Scheme, parsed.Host)
	}
	return parsed, nil
}

// requestDeviceCode requests a device code from GitHub.
func requestDeviceCode(ctx context.Context, client *http.Client, appID string) (*copilotDeviceCode, error) {
	endpoint, err := validateGitHubURL(authDeviceCodeURL)
	if err != nil {
		return nil, fmt.Errorf("invalid device code URL: %w", err)
	}

	reqBody, err := json.Marshal(map[string]string{
		"client_id": appID,
		"scope":     "read:user",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build device code request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create device code request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req) // URL validated by validateGitHubURL before use
	if err != nil {
		return nil, fmt.Errorf("device code request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to close response body: %v\n", closeErr)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read device code response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device code request returned status %d: %s", resp.StatusCode, string(body))
	}

	var dc copilotDeviceCode
	if err := json.Unmarshal(body, &dc); err != nil {
		return nil, fmt.Errorf("failed to parse device code response: %w", err)
	}
	return &dc, nil
}

// printAuthPrompt writes the user-facing authentication instructions.
func printAuthPrompt(cmd *cobra.Command, dc *copilotDeviceCode) error {
	out := cmd.OutOrStdout()
	lines := []string{
		"\nGitHub Copilot Authentication\n",
		fmt.Sprintf("1. Visit:      %s\n", dc.VerificationURI),
		fmt.Sprintf("2. Enter code: %s\n\n", dc.UserCode),
		"Waiting for authorization...\n",
	}
	for _, line := range lines {
		if _, err := fmt.Fprint(out, line); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
	}
	return nil
}

// pollUntilAuthorized polls until GitHub grants or denies the device authorization.
func pollUntilAuthorized(ctx context.Context, client *http.Client, appID, deviceCode string, pollInterval time.Duration) (string, error) {
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("authentication cancelled: %w", ctx.Err())
		case <-time.After(pollInterval):
		}

		token, pending, err := pollCopilotToken(ctx, client, appID, deviceCode)
		if err != nil {
			return "", err
		}
		if !pending {
			return token, nil
		}
	}
}

// runCopilotDeviceFlow performs the full GitHub OAuth device authorization grant.
func runCopilotDeviceFlow(ctx context.Context, cmd *cobra.Command, appID string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	dc, err := requestDeviceCode(ctx, client, appID)
	if err != nil {
		return "", err
	}

	if err := printAuthPrompt(cmd, dc); err != nil {
		return "", err
	}

	pollInterval := authDevicePollInterval
	if dc.Interval > 0 {
		pollInterval = time.Duration(dc.Interval) * time.Second
	}

	return pollUntilAuthorized(ctx, client, appID, dc.DeviceCode, pollInterval)
}

// pollCopilotToken sends one poll to the OAuth token endpoint.
// Returns (token, false, nil) on success, ("", true, nil) when still pending,.
// and ("", false, err) on a hard failure.
func pollCopilotToken(ctx context.Context, client *http.Client, appID, deviceCode string) (token string, pending bool, err error) {
	// Validate the token URL before use.
	oauthEndpoint, err := validateGitHubURL(authOAuthTokenURL)
	if err != nil {
		return "", false, fmt.Errorf("invalid OAuth token URL: %w", err)
	}

	body, err := json.Marshal(map[string]string{
		"client_id":   appID,
		"device_code": deviceCode,
		"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
	})
	if err != nil {
		return "", false, fmt.Errorf("failed to build poll request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, oauthEndpoint.String(), bytes.NewReader(body))
	if err != nil {
		return "", false, fmt.Errorf("failed to create poll request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req) // URL validated by validateGitHubURL before use
	if err != nil {
		return "", false, fmt.Errorf("poll request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to close response body: %v\n", closeErr)
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("failed to read poll response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", false, fmt.Errorf("failed to parse poll response: %w", err)
	}

	if errCode, ok := result["error"].(string); ok {
		if errCode == "authorization_pending" || errCode == "slow_down" {
			return "", true, nil
		}
		return "", false, fmt.Errorf("authorization error: %s", errCode)
	}

	tokenVal, ok := result["access_token"].(string)
	if !ok || tokenVal == "" {
		return "", true, nil
	}
	return tokenVal, false, nil
}

// persistCopilotToken writes the GitHub OAuth token to disk with restricted permissions.
func persistCopilotToken(tokenFile, token string) error {
	dir := filepath.Dir(tokenFile)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}
	if err := os.WriteFile(tokenFile, []byte(token), 0o600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}
	return nil
}

func init() {
	authCopilotCmd.Flags().BoolP("force", "f", false, "Re-authenticate even if a token already exists")
	authCmd.AddCommand(authCopilotCmd)
	rootCmd.AddCommand(authCmd)
}
