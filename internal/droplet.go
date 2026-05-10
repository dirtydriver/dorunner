package internal

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/digitalocean/godo"
)

//go:embed userdata/bootstrap-packer.sh
var bootstrapPacker string

//go:embed userdata/bootstrap-stock.sh
var bootstrapStock string

// DOClient wraps the DigitalOcean godo client and maintains a cached list of
// account snapshots so droplet creation can resolve snapshot names to IDs
// without a live API call on every request.
type DOClient struct {
	client        *godo.Client
	mu            sync.RWMutex
	snapshots     []godo.Snapshot
	runnerVersion string
}

// DropletConfig holds the resolved parameters for a new droplet, derived from
// the GitHub Actions runner labels attached to a workflow job.
type DropletConfig struct {
	Region        string
	Size          string
	Image         godo.DropletCreateImage
	BootStrapType string // "packer" or "stock"
}

// BootstrapData is the template context injected into the cloud-init user-data
// scripts at droplet creation time.
type BootstrapData struct {
	JITConfig     string // base64-encoded just-in-time runner config from GitHub
	RunnerVersion string // actions/runner release tag, e.g. "2.334.0"
}

// NewDOClient creates a DOClient authenticated with the token in cfg, then
// eagerly fetches the account's droplet snapshots so they are available for
// the first CreateDroplet call. Exits the process if the initial fetch fails.
func NewDOClient(cfg *Config) *DOClient {

	client := &DOClient{
		client:        godo.NewFromToken(cfg.DOToken),
		runnerVersion: cfg.RunnerVersion,
	}

	if err := client.fetchSnapshot(context.Background()); err != nil {
		log.Fatalf("failed to fetch snapshot at startup: %v", err)
	}

	return client
}

// CreateDroplet provisions an ephemeral DigitalOcean droplet for a single
// GitHub Actions job. It resolves the droplet parameters from the job's runner
// labels (region, size, image/snapshot), renders the appropriate cloud-init
// bootstrap script with the JIT config, then calls the Droplets.Create API.
// The droplet is tagged "github-runner" so the cleanup cron can find it later.
func (d *DOClient) CreateDroplet(ctx context.Context, runnerName string, jitConfig string, labels []string) error {
	cfg, err := d.parseLabels(labels)
	if err != nil {
		return fmt.Errorf("failed to parse labels: %w", err)
	}

	var script string

	switch cfg.BootStrapType {
	case "packer":
		script = bootstrapPacker
	case "stock":
		script = bootstrapStock
	default:
		return fmt.Errorf("invalid bootstrap type: %s", cfg.BootStrapType)
	}

	userData, err := renderUserData(script, BootstrapData{
		JITConfig:     jitConfig,
		RunnerVersion: d.runnerVersion,
	})
	if err != nil {
		return fmt.Errorf("failed to render user data: %w", err)
	}
	_, _, err = d.client.Droplets.Create(ctx, &godo.DropletCreateRequest{
		Name:     runnerName,
		Region:   cfg.Region,
		Size:     cfg.Size,
		Image:    cfg.Image,
		Tags:     []string{"github-runner"},
		UserData: userData,
	})
	if err != nil {
		return fmt.Errorf("failed to create droplet: %w", err)
	}
	return nil
}

// renderUserData executes script as a Go text/template with data as context,
// returning the rendered cloud-init user-data string.
func renderUserData(script string, data BootstrapData) (string, error) {

	tmpl, err := template.New("bootstrap").Parse(script)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// getDropletId resolves a droplet name to its numeric DigitalOcean ID.
func (d *DOClient) getDropletId(ctx context.Context, runnerName string) (int, error) {
	id, _, err := d.client.Droplets.ListByName(ctx, runnerName, nil)

	if err != nil {
		return 0, fmt.Errorf("failed to get droplet id for %s: %w", runnerName, err)
	}

	if len(id) == 0 {
		return 0, nil
	}

	return id[0].ID, nil
}

// getDropletsByTag returns all droplets carrying the given tag.
func (d *DOClient) getDropletsByTag(ctx context.Context, tag string) ([]godo.Droplet, error) {
	droplets, _, err := d.client.Droplets.ListByTag(ctx, tag, nil)

	if err != nil {
		return nil, fmt.Errorf("failed to get droplets by tag %s: %w", tag, err)
	}

	return droplets, nil
}

// deleteOrphanedDroplets deletes every "github-runner"-tagged droplet whose
// age exceeds ttl. This guards against droplets that were never cleaned up by
// the normal "completed" webhook path (e.g. the job was cancelled or the
// server restarted before receiving the event).
func (d *DOClient) deleteOrphanedDroplets(ctx context.Context, ttl time.Duration) error {
	droplets, err := d.getDropletsByTag(ctx, "github-runner")
	if err != nil {
		return fmt.Errorf("failed to get droplets by tag: %w", err)
	}

	for _, droplet := range droplets {
		created, err := time.Parse(time.RFC3339, droplet.Created)
		if err != nil {
			log.Printf("failed to parse droplet creation time for %s: %v", droplet.Name, err)
			continue
		}

		age := time.Since(created)

		if age > ttl {
			log.Printf("Droplet %s is older than %v, deleting", droplet.Name, ttl)
			_, err := d.client.Droplets.Delete(ctx, droplet.ID)
			if err != nil {
				log.Printf("failed to delete droplet %d: %v", droplet.ID, err)
			}
		}
	}

	return nil
}

// StartCleanupCron runs deleteOrphanedDroplets on a recurring interval until
// ctx is cancelled. Intended to be called in a goroutine from main.
func (d *DOClient) StartCleanupCron(ctx context.Context, interval, ttl time.Duration) {

	ticker := time.NewTicker(interval)

	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := d.deleteOrphanedDroplets(ctx, ttl); err != nil {
				log.Printf("failed to delete orphaned droplets: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}

}

// DeleteDroplet destroys the droplet with the given name. Called by the
// webhook handler when a workflow job reaches the "completed" state.
func (d *DOClient) DeleteDroplet(ctx context.Context, runnerName string) error {
	runnerID, err := d.getDropletId(ctx, runnerName)
	if err != nil {
		return fmt.Errorf("failed to get droplet id for %s: %w", runnerName, err)
	}
	if runnerID == 0 {
		log.Printf("droplet %s already deleted, skipping", runnerName)
		return nil
	}
	_, err = d.client.Droplets.Delete(ctx, runnerID)
	if err != nil {
		return fmt.Errorf("failed to delete droplet %d: %w", runnerID, err)
	}
	return nil
}

// fetchSnapshot refreshes the in-memory snapshot cache from the DigitalOcean
// API. It holds the write lock for the duration of the update so concurrent
// label parsing sees a consistent snapshot list.
func (d *DOClient) fetchSnapshot(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	snapshots, _, err := d.client.Snapshots.ListDroplet(ctx, &godo.ListOptions{})

	if err != nil {
		return fmt.Errorf("failed to list snapshots: %w", err)
	}

	d.snapshots = snapshots

	return nil
}

// StartSnapshotRefresher periodically refreshes the snapshot cache so that
// newly published Packer images are picked up without a restart. Runs until
// ctx is cancelled; call in a goroutine from main.
func (d *DOClient) StartSnapshotRefresher(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)

	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := d.fetchSnapshot(ctx); err != nil {
				log.Printf("failed to fetch snapshot: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

// parseLabels converts GitHub Actions runner labels of the form "key/value"
// into a DropletConfig. Recognised keys: region, size, image (stock slug),
// snapshot (Packer image name resolved against the cached snapshot list).
// Missing region, size, or image fall back to documented defaults.
func (d *DOClient) parseLabels(labels []string) (*DropletConfig, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	doConfig := &DropletConfig{}
	for _, label := range labels {
		parts := strings.SplitN(label, "/", 2)

		if len(parts) != 2 {
			continue
		}
		prefix, value := parts[0], parts[1]

		switch prefix {
		case "region":
			doConfig.Region = value
		case "size":
			doConfig.Size = value
		case "image":
			doConfig.Image = godo.DropletCreateImage{Slug: value}
			doConfig.BootStrapType = "stock"
		case "snapshot":

			var snapshotID int
			var err error
			for _, snapshot := range d.snapshots {
				if snapshot.Name == value {
					snapshotID, err = strconv.Atoi(snapshot.ID)
					if err != nil {
						return nil, fmt.Errorf("failed to convert snapshot ID to int: %w", err)
					}
					break
				}
			}

			if snapshotID == 0 {
				return nil, fmt.Errorf("snapshot %s not found", value)
			}
			doConfig.Image = godo.DropletCreateImage{ID: snapshotID}
			doConfig.BootStrapType = "packer"
		}
	}

	if doConfig.Size == "" {
		doConfig.Size = "s-1vcpu-2gb"
		log.Printf("size is not specified, using default: %s", doConfig.Size)
	}

	if doConfig.Region == "" {
		doConfig.Region = "fra1"
		log.Printf("region is not specified, using default: %s", doConfig.Region)
	}

	if doConfig.Image.Slug == "" && doConfig.Image.ID == 0 {
		doConfig.Image = godo.DropletCreateImage{Slug: "ubuntu-22-04-x64"}
		doConfig.BootStrapType = "stock"
		log.Printf("image is not specified, using default: %s", doConfig.Image.Slug)
	}

	return doConfig, nil
}
