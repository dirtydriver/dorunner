package internal

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration loaded from environment variables.
// Required variables cause a fatal log if absent; optional ones have documented
// defaults.
type Config struct {
	WebHookSecret        string        // WEB_HOOK_SECRET — GitHub webhook HMAC secret
	Port                 string        // PORT (default: 8080)
	GitHubAppID          int64         // GITHUB_APP_ID
	GitHubAppPrivateKey  string        // GITHUB_APP_PRIVATE_KEY — PEM-encoded RSA key
	GitHubInstallationID int64         // GITHUB_INSTALLATION_ID
	GitHubRunnerGroupID  int64         // GITHUB_RUNNER_GROUP_ID (default: 1)
	GitHubRunnerLabels   []string      // GITHUB_RUNNER_LABELS (default: self-hosted,linux,x64)
	DOToken              string        // DO_TOKEN — DigitalOcean personal access token
	DORunnerSnapshotID   *int          // DO_RUNNER_SNAPSHOT_ID (optional, mutually exclusive with DORunnerImageSlug)
	DORunnerImageSlug    *string       // DO_RUNNER_IMAGE_SLUG (optional, mutually exclusive with DORunnerSnapshotID)
	RunnerTTL            time.Duration // RUNNER_TTL (default: 1h) — max droplet lifetime before forced cleanup
	RunnerVersion        string        // RUNNER_VERSION (default: 2.334.0) — actions/runner release to install
}

// LoadConfig reads all configuration from environment variables, applies
// defaults for optional fields, and exits the process on any missing required
// value or parse error.
func LoadConfig() *Config {

	must := func(key string) string {
		val, err := requireEnv(key)
		if err != nil {
			log.Fatalf("Failed to load required environment variable %s: %v", key, err)
		}
		return val
	}

	optional := func(key string, defaultval string) string {
		val := os.Getenv(key)
		if val == "" {
			return defaultval
		}
		return val
	}

	appid, err := strconv.ParseInt(must("GITHUB_APP_ID"), 10, 64)
	if err != nil {
		log.Fatalf("Failed to parse GITHUB_APP_ID: %v", err)
	}

	installationID, err := strconv.ParseInt(must("GITHUB_INSTALLATION_ID"), 10, 64)
	if err != nil {
		log.Fatalf("Failed to parse GITHUB_INSTALLATION_ID: %v", err)
	}

	groupid, err := strconv.ParseInt(optional("GITHUB_RUNNER_GROUP_ID", "1"), 10, 64)
	if err != nil {
		log.Fatalf("Failed to parse GITHUB_RUNNER_GROUP_ID: %v", err)
	}

	runnerTTL, err := time.ParseDuration(optional("RUNNER_TTL", "1h"))
	if err != nil {
		log.Fatalf("Failed to parse RUNNER_TTL: %v", err)
	}

	dorunnerSnapshotIDStr := optional("DO_RUNNER_SNAPSHOT_ID", "")
	var dorunnerSnapshotID *int
	if dorunnerSnapshotIDStr != "" {
		snapshotID, err := strconv.Atoi(dorunnerSnapshotIDStr)
		if err != nil {
			log.Fatalf("Failed to parse DO_RUNNER_SNAPSHOT_ID: %v", err)
		}
		dorunnerSnapshotID = &snapshotID
	}

	dorunnerImageSlug := os.Getenv("DO_RUNNER_IMAGE_SLUG")
	var dorunnerImageSlugPtr *string
	if dorunnerImageSlug != "" {
		dorunnerImageSlugPtr = &dorunnerImageSlug
	}

	rawLabels := strings.Split(optional("GITHUB_RUNNER_LABELS", "self-hosted,linux,x64"), ",")

	labels := make([]string, len(rawLabels))

	for i, label := range rawLabels {
		labels[i] = strings.TrimSpace(label)
	}

	if len(labels) == 0 || len(labels) == 1 && labels[0] == "" {
		log.Fatalf("GITHUB_RUNNER_LABELS must not be empty")
	}

	cfg := &Config{
		WebHookSecret:        must("WEB_HOOK_SECRET"),
		Port:                 optional("PORT", "8080"),
		GitHubAppID:          appid,
		GitHubAppPrivateKey:  must("GITHUB_APP_PRIVATE_KEY"),
		GitHubInstallationID: installationID,
		GitHubRunnerGroupID:  groupid,
		GitHubRunnerLabels:   labels,
		DOToken:              must("DO_TOKEN"),
		DORunnerSnapshotID:   dorunnerSnapshotID,
		DORunnerImageSlug:    dorunnerImageSlugPtr,
		RunnerTTL:            runnerTTL,
		RunnerVersion:        optional("RUNNER_VERSION", "2.334.0"),
	}

	if cfg.DORunnerSnapshotID == nil && cfg.DORunnerImageSlug == nil {
		log.Fatalf("Either DO_RUNNER_SNAPSHOT_ID or DO_RUNNER_IMAGE_SLUG must be set")
	}

	return cfg
}

// requireEnv returns the value of the named environment variable or an error
// if it is unset or empty.
func requireEnv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("environment variable %s is required", key)
	}
	return value, nil
}
