package internal

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v84/github"
)

type GitHubClient struct {
	client *github.Client
}

func NewGitHubClient(config *Config) *GitHubClient {
	transport, err := ghinstallation.New(http.DefaultTransport, config.GitHubAppID, config.GitHubInstallationID, []byte(config.GitHubAppPrivateKey))
	if err != nil {
		log.Fatalf("Failed to create GitHub client: %v", err)
	}
	return &GitHubClient{
		client: github.NewClient(&http.Client{Transport: transport}),
	}
}

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
