package internal

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	WebHookSecret        string
	Port                 string
	GitHubAppID          int64
	GitHubAppPrivateKey  string
	GitHubInstallationID int64
	GitHubRunnerGroupID  int64
	GitHubRunnerLabels   []string
	DOToken              string
	DORunnerSnapshotID   *int
	DORunnerImageSlug    *string
	RunnerTTL            time.Duration
	RunnerVersion        string
}

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

func requireEnv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("environment variable %s is required", key)
	}
	return value, nil
}
