// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	commandAuth            = "auth"
	authDeviceCodeURL      = "https://github.com/login/device/code"
	authOAuthTokenURL      = "https://github.com/login/oauth/access_token"
	authDefaultAppID       = "Iv1.b507a08c87ecfe98"
	authDevicePollInterval = 5 * time.Second
)

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

func runAuthCopilot(cmd *cobra.Command, _ []string) error {
	appID := resolveCopilotAppID()

	tokenFile, err := copilotTokenFilePath()
	if err != nil {
		return err
	}

	// Check for an existing token and offer to re-auth.
	if data, err := os.ReadFile(tokenFile); err == nil && strings.TrimSpace(string(data)) != "" {
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Fprintf(cmd.OutOrStdout(), "Already authenticated. Use --force to re-authenticate.\n")
			return nil
		}
	}

	token, err := runCopilotDeviceFlow(cmd.Context(), cmd, appID)
	if err != nil {
		return err
	}

	if err := persistCopilotToken(tokenFile, token); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nAuthenticated successfully. Token saved to %s\n", tokenFile)
	return nil
}

// resolveCopilotAppID reads the app_id from the Starlark config if a github-copilot
// provider is defined, otherwise falls back to the default Neovim app ID.
func resolveCopilotAppID() string {
	container, workspaceRoot, err := app.NewAppContainerForStarlark()
	if err != nil {
		return authDefaultAppID
	}
	defer container.ShutdownService.Shutdown()

	rt := starlarkpkg.NewRuntime(workspaceRoot)
	loader := starlarkpkg.NewLoaderService(rt)
	if err := loader.LoadAll(); err != nil {
		return authDefaultAppID
	}

	for _, p := range rt.Providers() {
		if p.Type == string(provider.GitHubCopilot) && p.AppID != "" {
			return p.AppID
		}
	}
	return authDefaultAppID
}

// copilotTokenFilePath returns the path to the persisted GitHub OAuth token.
func copilotTokenFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "meowg1k", "copilot_token"), nil
}

// runCopilotDeviceFlow performs the full GitHub OAuth device authorization grant.
func runCopilotDeviceFlow(ctx context.Context, cmd *cobra.Command, appID string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	// Step 1: request a device code.
	reqBody, err := json.Marshal(map[string]string{
		"client_id": appID,
		"scope":     "read:user",
	})
	if err != nil {
		return "", fmt.Errorf("failed to build device code request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authDeviceCodeURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create device code request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("device code request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // best-effort close in defer

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read device code response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("device code request returned status %d: %s", resp.StatusCode, string(body))
	}

	var deviceResp struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		Interval        int    `json:"interval"`
	}
	if err := json.Unmarshal(body, &deviceResp); err != nil {
		return "", fmt.Errorf("failed to parse device code response: %w", err)
	}

	// Step 2: prompt the user.
	fmt.Fprintf(cmd.OutOrStdout(), "\nGitHub Copilot Authentication\n")
	fmt.Fprintf(cmd.OutOrStdout(), "1. Visit:      %s\n", deviceResp.VerificationURI)
	fmt.Fprintf(cmd.OutOrStdout(), "2. Enter code: %s\n\n", deviceResp.UserCode)
	fmt.Fprintf(cmd.OutOrStdout(), "Waiting for authorization...\n")

	// Step 3: poll until the user authorizes.
	pollInterval := authDevicePollInterval
	if deviceResp.Interval > 0 {
		pollInterval = time.Duration(deviceResp.Interval) * time.Second
	}

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("authentication cancelled: %w", ctx.Err())
		case <-time.After(pollInterval):
		}

		token, pending, pollErr := pollCopilotToken(ctx, client, appID, deviceResp.DeviceCode)
		if pollErr != nil {
			return "", pollErr
		}
		if pending {
			continue
		}
		return token, nil
	}
}

// pollCopilotToken sends one poll to the OAuth token endpoint.
// Returns (token, false, nil) on success, ("", true, nil) when still pending,
// and ("", false, err) on a hard failure.
func pollCopilotToken(ctx context.Context, client *http.Client, appID, deviceCode string) (string, bool, error) {
	body, err := json.Marshal(map[string]string{
		"client_id":   appID,
		"device_code": deviceCode,
		"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
	})
	if err != nil {
		return "", false, fmt.Errorf("failed to build poll request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authOAuthTokenURL, bytes.NewReader(body))
	if err != nil {
		return "", false, fmt.Errorf("failed to create poll request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("poll request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // best-effort close in defer

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

	token, _ := result["access_token"].(string) //nolint:errcheck // type assertion; empty string handled below
	if token == "" {
		return "", true, nil
	}
	return token, false, nil
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
