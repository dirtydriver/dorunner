package internal

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v84/github"
)

// GitHubClient wraps the go-github client and provides GitHub Actions
// runner-management operations used by the webhook handler.
type GitHubClient struct {
	client *github.Client
}

// NewGitHubClient creates a GitHubClient that authenticates as a GitHub App
// installation using the app ID, installation ID, and PEM private key from cfg.
// Exits the process if the transport cannot be initialised.
func NewGitHubClient(config *Config) *GitHubClient {
	transport, err := ghinstallation.New(http.DefaultTransport, config.GitHubAppID, config.GitHubInstallationID, normalizePEM(config.GitHubAppPrivateKey))
	if err != nil {
		log.Fatalf("Failed to create GitHub client: %v", err)
	}
	return &GitHubClient{
		client: github.NewClient(&http.Client{Transport: transport}),
	}
}

// normalizePEM repairs private keys mangled by env-var handling: surrounding
// quotes, literal "\n" sequences instead of newlines, or a base64-encoded PEM.
func normalizePEM(key string) []byte {
	key = strings.TrimSpace(key)
	key = strings.Trim(key, `"'`)
	if !strings.Contains(key, "-----BEGIN") {
		if decoded, err := base64.StdEncoding.DecodeString(key); err == nil {
			key = string(decoded)
		}
	}
	key = strings.ReplaceAll(key, `\n`, "\n")
	key = strings.ReplaceAll(key, "\r\n", "\n")
	return []byte(key)
}

// FetchJITConfig generates a just-in-time runner registration token for a
// single-use ephemeral runner attached to owner/repo. The returned string is
// the base64-encoded JIT config passed directly to the runner's config.sh.
func (c *GitHubClient) FetchJITConfig(ctx context.Context, owner, repo, runnerName string, runnerGroupID int64, labels []string) (string, error) {

	jitConfig, _, err := c.client.Actions.GenerateRepoJITConfig(ctx, owner, repo, &github.GenerateJITConfigRequest{
		Name:          runnerName,
		RunnerGroupID: runnerGroupID,
		Labels:        labels,
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch JIT config: %w", err)
	}
	if jitConfig == nil {
		return "", fmt.Errorf("JIT config is nil")
	}
	if jitConfig.EncodedJITConfig == nil {
		return "", fmt.Errorf("JIT config returned nil for runner %s", runnerName)
	}

	return *jitConfig.EncodedJITConfig, nil
}
