package internal

import (
	"testing"
)

func TestNewGitHubClient(t *testing.T) {
	t.Run("creates client with valid config", func(t *testing.T) {
		cfg := &Config{
			GitHubAppID:          12345,
			GitHubInstallationID: 67890,
			GitHubAppPrivateKey: `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA0Z3VS5JJcds3xfn/ygWyF6bfjJJpBmLFEqHJUWQpQs0EXAMPLE
-----END RSA PRIVATE KEY-----`,
		}

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("NewGitHubClient() panicked with valid config: %v", r)
			}
		}()

		client := NewGitHubClient(cfg)
		if client == nil {
			t.Error("NewGitHubClient() returned nil")
		}
		if client.client == nil {
			t.Error("NewGitHubClient() client.client is nil")
		}
	})

	t.Run("fails with invalid private key", func(t *testing.T) {
		cfg := &Config{
			GitHubAppID:          12345,
			GitHubInstallationID: 67890,
			GitHubAppPrivateKey:  "invalid-key",
		}

		defer func() {
			if r := recover(); r == nil {
				t.Error("NewGitHubClient() should have panicked with invalid key")
			}
		}()

		NewGitHubClient(cfg)
	})
}
