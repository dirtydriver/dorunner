# dorunner

Self-hosted GitHub Actions runner controller that provisions ephemeral DigitalOcean Droplets per job and destroys them automatically when the workflow completes.

## How it works

1. A workflow job is queued on GitHub → GitHub sends a `workflow_job` webhook event with action `queued`.
2. dorunner fetches a just-in-time (JIT) runner registration token from the GitHub API.
3. A new DigitalOcean Droplet is created with cloud-init user-data that installs and registers the runner using the JIT token.
4. The runner picks up the job, executes it, and exits.
5. GitHub sends a `workflow_job` event with action `completed` → dorunner deletes the Droplet.
6. A background cleanup cron deletes any Droplet older than `RUNNER_TTL` (safety net for missed events).

## Prerequisites

- A **GitHub App** installed on the target organisation or repository with permissions:
  - `Actions` → Read & Write (to generate JIT configs)
  - Subscribe to the `workflow_job` event
- A **DigitalOcean personal access token** with Droplet read/write scope.
- A publicly reachable HTTPS endpoint to receive GitHub webhooks (e.g. behind a reverse proxy or load balancer).

## Configuration

All configuration is supplied via environment variables.

| Variable | Required | Default | Description |
|---|---|---|---|
| `WEB_HOOK_SECRET` | yes | — | GitHub webhook HMAC-SHA256 secret |
| `GITHUB_APP_ID` | yes | — | GitHub App numeric ID |
| `GITHUB_APP_PRIVATE_KEY` | yes | — | PEM-encoded RSA private key for the App |
| `GITHUB_INSTALLATION_ID` | yes | — | Installation ID of the App on your org/repo |
| `GITHUB_RUNNER_GROUP_ID` | no | `1` | Runner group to register new runners into |
| `GITHUB_RUNNER_LABELS` | no | `self-hosted,linux,x64` | Comma-separated labels applied to every runner |
| `DO_TOKEN` | yes | — | DigitalOcean personal access token |
| `DO_RUNNER_SNAPSHOT_ID` | yes* | — | Snapshot ID to use for Packer-built images |
| `DO_RUNNER_IMAGE_SLUG` | yes* | — | DigitalOcean image slug for stock images |
| `RUNNER_TTL` | no | `1h` | Maximum Droplet lifetime; older Droplets are force-deleted |
| `RUNNER_VERSION` | no | `2.334.0` | `actions/runner` release version to install on stock images |
| `PORT` | no | `8080` | HTTP port to listen on |

\* At least one of `DO_RUNNER_SNAPSHOT_ID` or `DO_RUNNER_IMAGE_SLUG` must be set.

## Runner labels

Workflow jobs select Droplet parameters by adding structured labels of the form `key/value` alongside the standard runner labels:

| Label | Example | Description |
|---|---|---|
| `region/<slug>` | `region/fra1` | DigitalOcean region slug (default: `fra1`) |
| `size/<slug>` | `size/s-2vcpu-4gb` | Droplet size slug (default: `s-1vcpu-2gb`) |
| `image/<slug>` | `image/ubuntu-22-04-x64` | Stock DigitalOcean image slug → uses **stock** bootstrap |
| `snapshot/<name>` | `snapshot/my-runner-image` | Packer snapshot name → uses **packer** bootstrap |

If neither `image` nor `snapshot` is provided, the Droplet defaults to `ubuntu-22-04-x64` with stock bootstrap.

### Example workflow

```yaml
jobs:
  build:
    runs-on:
      - self-hosted
      - linux
      - x64
      - region/fra1
      - size/s-2vcpu-4gb
      - snapshot/my-runner-image
```

## Bootstrap types

### stock

Used when the Droplet is created from a plain OS image. The bootstrap script:

1. Creates a `runner` user.
2. Downloads and extracts the `actions/runner` release specified by `RUNNER_VERSION`.
3. Registers and starts the runner with the JIT config.

### packer

Used when the Droplet is created from a pre-baked Packer snapshot that already has the runner binaries installed at `/home/runner/actions-runner`. The bootstrap script only registers and starts the runner — no download step needed, so startup is faster.

## CI/CD

The project uses GitHub Actions for continuous integration and deployment:

- **Build job**: Runs on every push/PR to `main`
  - Builds with Go 1.26
  - Runs tests (`go test ./...`)
  - Performs vulnerability scanning with `govulncheck`
- **Docker job**: Runs only on version tags (`v*`)
  - Builds and pushes Docker images to GitHub Container Registry (`ghcr.io/dirtydriver/dorunner`)
  - Tags: semantic versioning (`v1.2.3`, `v1.2`, `v1`) and SHA

## Running

### Docker

Pull from GitHub Container Registry:

```bash
docker run -d \
  -e WEB_HOOK_SECRET=... \
  -e GITHUB_APP_ID=... \
  -e GITHUB_APP_PRIVATE_KEY="$(cat private-key.pem)" \
  -e GITHUB_INSTALLATION_ID=... \
  -e DO_TOKEN=... \
  -e DO_RUNNER_IMAGE_SLUG=ubuntu-22-04-x64 \
  -p 8080:8080 \
  ghcr.io/dirtydriver/dorunner:latest
```

Or build locally:

```bash
docker build -t dorunner .

docker run -d \
  -e WEB_HOOK_SECRET=... \
  -e GITHUB_APP_ID=... \
  -e GITHUB_APP_PRIVATE_KEY="$(cat private-key.pem)" \
  -e GITHUB_INSTALLATION_ID=... \
  -e DO_TOKEN=... \
  -e DO_RUNNER_IMAGE_SLUG=ubuntu-22-04-x64 \
  -p 8080:8080 \
  dorunner
```

The image is built on `gcr.io/distroless/base:nonroot` — it contains only the statically linked binary and runs as a non-root user.

### Local

```bash
go build -o dorunner .
export WEB_HOOK_SECRET=...
# set remaining env vars
./dorunner
```

## Webhook endpoint

```
POST /webhook
```

dorunner validates the `X-Hub-Signature-256` header on every request. Configure this URL as the webhook endpoint in your GitHub App settings, selecting the `workflow_job` event.
