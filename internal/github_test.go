package internal

import (
	"testing"
)

func TestGitHubClient(t *testing.T) {
	t.Run("GitHubClient struct can be instantiated", func(t *testing.T) {
		client := &GitHubClient{}
		if client.client != nil {
			t.Error("GitHubClient.client should be nil when not initialized")
		}
	})
}
