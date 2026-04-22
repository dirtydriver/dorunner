package internal

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/digitalocean/godo"
)

type DOClient struct {
	client    *godo.Client
	mu        sync.RWMutex
	snapshots []godo.Snapshot
}

type DropletConfig struct {
	Region string
	Size   string
	Image  godo.DropletCreateImage
}

func NewDOClient(cfg *Config) *DOClient {

	client := &DOClient{
		client: godo.NewFromToken(cfg.DOToken),
	}

	if err := client.fetchSnapshot(context.Background()); err != nil {
		log.Fatalf("failed to fetch snapshot at startup: %v", err)
	}

	return client
}

func (d *DOClient) CreateDroplet(ctx context.Context, runnerName string, jitConfig string, labels []string) error {
	cfg, err := d.parseLabels(labels)
	if err != nil {
		return fmt.Errorf("failed to parse labels: %w", err)
	}
	_, _, err = d.client.Droplets.Create(ctx, &godo.DropletCreateRequest{
		Name:     runnerName,
		Region:   cfg.Region,
		Size:     cfg.Size,
		Image:    cfg.Image,
		Tags:     []string{"github-runner"},
		UserData: fmt.Sprintf("#!/bin/bash\nJIT_CONFIG=\"%s\"", jitConfig),
	})
	if err != nil {
		return fmt.Errorf("failed to create droplet: %w", err)
	}
	return nil
}

func (d *DOClient) getDropletId(ctx context.Context, runnerName string) (int, error) {
	id, _, err := d.client.Droplets.ListByName(ctx, runnerName, nil)

	if err != nil {
		return 0, fmt.Errorf("failed to get droplet id for %s: %w", runnerName, err)
	}

	if len(id) == 0 {
		return 0, fmt.Errorf("droplet %s not found", runnerName)
	}

	return id[0].ID, nil
}

func (d *DOClient) DeleteDroplet(ctx context.Context, runnerName string) error {
	runnerID, err := d.getDropletId(ctx, runnerName)
	if err != nil {
		return fmt.Errorf("failed to get droplet id for %s: %w", runnerName, err)
	}
	_, err = d.client.Droplets.Delete(ctx, runnerID)
	if err != nil {
		return fmt.Errorf("failed to delete droplet %d: %w", runnerID, err)
	}
	return nil
}

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
		log.Printf("image is not specified, using default: %s", doConfig.Image.Slug)
	}

	return doConfig, nil
}
