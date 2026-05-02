package internal

import (
	"os"
	"testing"
	"time"
)

func TestRequireEnv(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr bool
	}{
		{
			name:    "returns value when env var is set",
			key:     "TEST_VAR",
			value:   "test-value",
			wantErr: false,
		},
		{
			name:    "returns error when env var is empty",
			key:     "TEST_VAR",
			value:   "",
			wantErr: true,
		},
		{
			name:    "returns error when env var is not set",
			key:     "NONEXISTENT_VAR",
			value:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != "" {
				os.Setenv(tt.key, tt.value)
				defer os.Unsetenv(tt.key)
			} else if tt.key == "TEST_VAR" {
				os.Unsetenv(tt.key)
			}

			got, err := requireEnv(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("requireEnv() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.value {
				t.Errorf("requireEnv() = %v, want %v", got, tt.value)
			}
		})
	}
}

func TestLoadConfig_Success(t *testing.T) {
	setRequiredEnvVars := func() {
		os.Setenv("WEB_HOOK_SECRET", "test-secret")
		os.Setenv("GITHUB_APP_ID", "12345")
		os.Setenv("GITHUB_APP_PRIVATE_KEY", "test-private-key")
		os.Setenv("GITHUB_INSTALLATION_ID", "67890")
		os.Setenv("DO_TOKEN", "test-do-token")
		os.Setenv("DO_RUNNER_SNAPSHOT_ID", "123456")
	}

	clearEnvVars := func() {
		os.Unsetenv("WEB_HOOK_SECRET")
		os.Unsetenv("PORT")
		os.Unsetenv("GITHUB_APP_ID")
		os.Unsetenv("GITHUB_APP_PRIVATE_KEY")
		os.Unsetenv("GITHUB_INSTALLATION_ID")
		os.Unsetenv("GITHUB_RUNNER_GROUP_ID")
		os.Unsetenv("GITHUB_RUNNER_LABELS")
		os.Unsetenv("DO_TOKEN")
		os.Unsetenv("DO_RUNNER_SNAPSHOT_ID")
		os.Unsetenv("DO_RUNNER_IMAGE_SLUG")
		os.Unsetenv("RUNNER_TTL")
		os.Unsetenv("RUNNER_VERSION")
	}

	t.Run("loads config with all required vars and defaults", func(t *testing.T) {
		clearEnvVars()
		setRequiredEnvVars()
		defer clearEnvVars()

		cfg := LoadConfig()

		if cfg.WebHookSecret != "test-secret" {
			t.Errorf("WebHookSecret = %v, want test-secret", cfg.WebHookSecret)
		}
		if cfg.Port != "8080" {
			t.Errorf("Port = %v, want 8080", cfg.Port)
		}
		if cfg.GitHubAppID != 12345 {
			t.Errorf("GitHubAppID = %v, want 12345", cfg.GitHubAppID)
		}
		if cfg.GitHubAppPrivateKey != "test-private-key" {
			t.Errorf("GitHubAppPrivateKey = %v, want test-private-key", cfg.GitHubAppPrivateKey)
		}
		if cfg.GitHubInstallationID != 67890 {
			t.Errorf("GitHubInstallationID = %v, want 67890", cfg.GitHubInstallationID)
		}
		if cfg.GitHubRunnerGroupID != 1 {
			t.Errorf("GitHubRunnerGroupID = %v, want 1", cfg.GitHubRunnerGroupID)
		}
		if len(cfg.GitHubRunnerLabels) != 3 {
			t.Errorf("GitHubRunnerLabels length = %v, want 3", len(cfg.GitHubRunnerLabels))
		}
		if cfg.GitHubRunnerLabels[0] != "self-hosted" || cfg.GitHubRunnerLabels[1] != "linux" || cfg.GitHubRunnerLabels[2] != "x64" {
			t.Errorf("GitHubRunnerLabels = %v, want [self-hosted linux x64]", cfg.GitHubRunnerLabels)
		}
		if cfg.DOToken != "test-do-token" {
			t.Errorf("DOToken = %v, want test-do-token", cfg.DOToken)
		}
		if cfg.DORunnerSnapshotID == nil || *cfg.DORunnerSnapshotID != 123456 {
			t.Errorf("DORunnerSnapshotID = %v, want 123456", cfg.DORunnerSnapshotID)
		}
		if cfg.DORunnerImageSlug != nil {
			t.Errorf("DORunnerImageSlug = %v, want nil", cfg.DORunnerImageSlug)
		}
		if cfg.RunnerTTL != time.Hour {
			t.Errorf("RunnerTTL = %v, want 1h", cfg.RunnerTTL)
		}
		if cfg.RunnerVersion != "2.334.0" {
			t.Errorf("RunnerVersion = %v, want 2.334.0", cfg.RunnerVersion)
		}
	})

	t.Run("loads config with custom optional values", func(t *testing.T) {
		clearEnvVars()
		setRequiredEnvVars()
		os.Setenv("PORT", "9090")
		os.Setenv("GITHUB_RUNNER_GROUP_ID", "42")
		os.Setenv("GITHUB_RUNNER_LABELS", "custom,labels,test")
		os.Setenv("RUNNER_TTL", "2h30m")
		os.Setenv("RUNNER_VERSION", "2.400.0")
		defer clearEnvVars()

		cfg := LoadConfig()

		if cfg.Port != "9090" {
			t.Errorf("Port = %v, want 9090", cfg.Port)
		}
		if cfg.GitHubRunnerGroupID != 42 {
			t.Errorf("GitHubRunnerGroupID = %v, want 42", cfg.GitHubRunnerGroupID)
		}
		if len(cfg.GitHubRunnerLabels) != 3 {
			t.Errorf("GitHubRunnerLabels length = %v, want 3", len(cfg.GitHubRunnerLabels))
		}
		if cfg.GitHubRunnerLabels[0] != "custom" || cfg.GitHubRunnerLabels[1] != "labels" || cfg.GitHubRunnerLabels[2] != "test" {
			t.Errorf("GitHubRunnerLabels = %v, want [custom labels test]", cfg.GitHubRunnerLabels)
		}
		if cfg.RunnerTTL != 2*time.Hour+30*time.Minute {
			t.Errorf("RunnerTTL = %v, want 2h30m", cfg.RunnerTTL)
		}
		if cfg.RunnerVersion != "2.400.0" {
			t.Errorf("RunnerVersion = %v, want 2.400.0", cfg.RunnerVersion)
		}
	})

	t.Run("loads config with DO_RUNNER_IMAGE_SLUG instead of snapshot", func(t *testing.T) {
		clearEnvVars()
		setRequiredEnvVars()
		os.Unsetenv("DO_RUNNER_SNAPSHOT_ID")
		os.Setenv("DO_RUNNER_IMAGE_SLUG", "ubuntu-22-04-x64")
		defer clearEnvVars()

		cfg := LoadConfig()

		if cfg.DORunnerSnapshotID != nil {
			t.Errorf("DORunnerSnapshotID = %v, want nil", cfg.DORunnerSnapshotID)
		}
		if cfg.DORunnerImageSlug == nil || *cfg.DORunnerImageSlug != "ubuntu-22-04-x64" {
			t.Errorf("DORunnerImageSlug = %v, want ubuntu-22-04-x64", cfg.DORunnerImageSlug)
		}
	})

	t.Run("trims whitespace from runner labels", func(t *testing.T) {
		clearEnvVars()
		setRequiredEnvVars()
		os.Setenv("GITHUB_RUNNER_LABELS", " label1 , label2 , label3 ")
		defer clearEnvVars()

		cfg := LoadConfig()

		if cfg.GitHubRunnerLabels[0] != "label1" || cfg.GitHubRunnerLabels[1] != "label2" || cfg.GitHubRunnerLabels[2] != "label3" {
			t.Errorf("GitHubRunnerLabels = %v, want [label1 label2 label3]", cfg.GitHubRunnerLabels)
		}
	})
}

func TestLoadConfig_Failures(t *testing.T) {
	clearEnvVars := func() {
		os.Unsetenv("WEB_HOOK_SECRET")
		os.Unsetenv("PORT")
		os.Unsetenv("GITHUB_APP_ID")
		os.Unsetenv("GITHUB_APP_PRIVATE_KEY")
		os.Unsetenv("GITHUB_INSTALLATION_ID")
		os.Unsetenv("GITHUB_RUNNER_GROUP_ID")
		os.Unsetenv("GITHUB_RUNNER_LABELS")
		os.Unsetenv("DO_TOKEN")
		os.Unsetenv("DO_RUNNER_SNAPSHOT_ID")
		os.Unsetenv("DO_RUNNER_IMAGE_SLUG")
		os.Unsetenv("RUNNER_TTL")
		os.Unsetenv("RUNNER_VERSION")
	}

	setRequiredEnvVars := func() {
		os.Setenv("WEB_HOOK_SECRET", "test-secret")
		os.Setenv("GITHUB_APP_ID", "12345")
		os.Setenv("GITHUB_APP_PRIVATE_KEY", "test-private-key")
		os.Setenv("GITHUB_INSTALLATION_ID", "67890")
		os.Setenv("DO_TOKEN", "test-do-token")
		os.Setenv("DO_RUNNER_SNAPSHOT_ID", "123456")
	}

	tests := []struct {
		name  string
		setup func()
	}{
		{
			name: "fails when WEB_HOOK_SECRET is missing",
			setup: func() {
				setRequiredEnvVars()
				os.Unsetenv("WEB_HOOK_SECRET")
			},
		},
		{
			name: "fails when GITHUB_APP_ID is missing",
			setup: func() {
				setRequiredEnvVars()
				os.Unsetenv("GITHUB_APP_ID")
			},
		},
		{
			name: "fails when GITHUB_APP_ID is invalid",
			setup: func() {
				setRequiredEnvVars()
				os.Setenv("GITHUB_APP_ID", "not-a-number")
			},
		},
		{
			name: "fails when GITHUB_APP_PRIVATE_KEY is missing",
			setup: func() {
				setRequiredEnvVars()
				os.Unsetenv("GITHUB_APP_PRIVATE_KEY")
			},
		},
		{
			name: "fails when GITHUB_INSTALLATION_ID is missing",
			setup: func() {
				setRequiredEnvVars()
				os.Unsetenv("GITHUB_INSTALLATION_ID")
			},
		},
		{
			name: "fails when GITHUB_INSTALLATION_ID is invalid",
			setup: func() {
				setRequiredEnvVars()
				os.Setenv("GITHUB_INSTALLATION_ID", "invalid")
			},
		},
		{
			name: "fails when GITHUB_RUNNER_GROUP_ID is invalid",
			setup: func() {
				setRequiredEnvVars()
				os.Setenv("GITHUB_RUNNER_GROUP_ID", "not-a-number")
			},
		},
		{
			name: "fails when DO_TOKEN is missing",
			setup: func() {
				setRequiredEnvVars()
				os.Unsetenv("DO_TOKEN")
			},
		},
		{
			name: "fails when DO_RUNNER_SNAPSHOT_ID is invalid",
			setup: func() {
				setRequiredEnvVars()
				os.Setenv("DO_RUNNER_SNAPSHOT_ID", "not-a-number")
			},
		},
		{
			name: "fails when neither DO_RUNNER_SNAPSHOT_ID nor DO_RUNNER_IMAGE_SLUG is set",
			setup: func() {
				setRequiredEnvVars()
				os.Unsetenv("DO_RUNNER_SNAPSHOT_ID")
			},
		},
		{
			name: "fails when RUNNER_TTL is invalid",
			setup: func() {
				setRequiredEnvVars()
				os.Setenv("RUNNER_TTL", "invalid-duration")
			},
		},
		{
			name: "fails when GITHUB_RUNNER_LABELS is empty",
			setup: func() {
				setRequiredEnvVars()
				os.Setenv("GITHUB_RUNNER_LABELS", "")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnvVars()
			tt.setup()
			defer clearEnvVars()

			defer func() {
				if r := recover(); r == nil {
					t.Errorf("LoadConfig() did not panic as expected")
				}
			}()

			LoadConfig()
		})
	}
}
