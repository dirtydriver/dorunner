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
  - `Administration` → Read & Write (to generate JIT runner configs)
- A **DigitalOcean personal access token** with Droplet read/write scope.
- A publicly reachable HTTPS endpoint to receive GitHub webhooks (e.g. behind a reverse proxy or load balancer).

## Installation Guide

### Step 1: Create a GitHub App

1. Navigate to your GitHub organization or personal account settings
2. Go to **Settings** → **Developer settings** → **GitHub Apps** → **New GitHub App**
3. Fill in the required fields:
   - **GitHub App name**: Choose a unique name (e.g., `dorunner-myorg`)
   - **Homepage URL**: Your organization URL or repository URL
   - **Webhook URL**: Leave empty (webhooks will be configured per-repository in Step 3.5)
4. Set **Repository permissions**:
   - **Administration**: Read & Write (required to generate JIT runner configs)
5. **Do not** subscribe to any events in the App settings (webhooks are configured per-repository)
6. Set **Where can this GitHub App be installed?**:
   - Choose based on your needs (only this account or any account)
7. Click **Create GitHub App**
8. After creation, note down the **App ID** (this is your `GITHUB_APP_ID`)
9. Generate a webhook secret and save it securely — this will be your `WEB_HOOK_SECRET`:
   ```bash
   openssl rand -hex 32
   ```

### Step 2: Generate GitHub App Private Key

1. On your GitHub App's settings page, scroll to **Private keys**
2. Click **Generate a private key**
3. A `.pem` file will be downloaded — **save this securely and never commit it to git**
4. Add `*.pem` to your `.gitignore` if storing locally
5. The contents of this file will be your `GITHUB_APP_PRIVATE_KEY`

**Note on PEM format**: When passing the private key as an environment variable, you need to preserve newlines:
- In shell: `GITHUB_APP_PRIVATE_KEY="$(cat private-key.pem)"`
- In Docker: Use `-e GITHUB_APP_PRIVATE_KEY="$(cat private-key.pem)"`
- In DigitalOcean App Platform: Paste the raw PEM contents including `-----BEGIN RSA PRIVATE KEY-----` and `-----END RSA PRIVATE KEY-----` lines

### Step 3: Install the GitHub App

1. On your GitHub App's settings page, click **Install App** in the left sidebar
2. Select the organization or account where you want to install it
3. Choose **All repositories** or **Only select repositories** based on your needs
4. Click **Install**
5. After installation, note the installation ID from the URL:
   - **Personal account**: `https://github.com/settings/installations/12345678`
   - **Organization**: `https://github.com/organizations/your-org/settings/installations/12345678`
   - The number `12345678` is your `GITHUB_INSTALLATION_ID`

### Step 3.5: Configure Repository Webhook

1. Navigate to the repository where you want to use self-hosted runners
2. Go to **Settings** → **Webhooks** → **Add webhook**
3. Configure the webhook:
   - **Payload URL**: Your public dorunner endpoint (e.g., `https://dorunner.example.com/webhook`)
   - **Content type**: `application/json`
   - **Secret**: Paste the webhook secret you generated in Step 1 point 9 (your `WEB_HOOK_SECRET`)
   - **Which events**: Select **Let me select individual events**, then check only **Workflow jobs**
   - **Active**: ✅ Checked
4. Click **Add webhook**
5. Repeat for each repository that needs self-hosted runners

### Step 4: Create DigitalOcean API Token

1. Log in to your [DigitalOcean account](https://cloud.digitalocean.com/)
2. Navigate to **API** in the left sidebar (or go to https://cloud.digitalocean.com/account/api/tokens)
3. Click **Generate New Token**
4. Set the token name (e.g., `dorunner-token`)
5. Required scopes:
   - **Droplets**: Create, Read, Delete
   - **Snapshots**: Read
6. Click **Generate Token**
7. **Important**: Copy the token immediately and save it securely (this is your `DO_TOKEN`)
   - You won't be able to see it again after leaving the page

### Step 5: (Optional) Create Packer Snapshot

If you want faster runner startup times, you can pre-build a DigitalOcean snapshot with the GitHub Actions runner pre-installed.

1. Use [Packer](https://www.packer.io/) to build a custom image with the runner binaries at `/home/runner/actions-runner`
2. After building, find the snapshot in DigitalOcean:
   - Navigate to **Images** → **Snapshots**
   - Note the snapshot name (this will be used with the `snapshot/<name>` label)
   - The snapshot ID will be automatically resolved by dorunner

Alternatively, skip this step and use stock DigitalOcean images (slower startup but no pre-building required).

### Step 6: Deploy dorunner

Choose one of the deployment methods below and configure the environment variables from the previous steps.

**Required environment variables:**
```bash
WEB_HOOK_SECRET=<random-secret-generated-in-step-1-point-9>
GITHUB_APP_ID=<your-app-id-from-step-1>
GITHUB_APP_PRIVATE_KEY=<contents-of-pem-file-from-step-2>
GITHUB_INSTALLATION_ID=<installation-id-from-step-3>
DO_TOKEN=<your-do-token-from-step-4>
```

See the [Running](#running) section for deployment options.

### Step 7: Test the Setup

1. Create a test workflow in your repository:
   ```yaml
   name: Test Self-Hosted Runner
   on: [push]
   jobs:
     test:
       runs-on:
         - self-hosted
         - linux
         - size/s-1vcpu-2gb
         - region/fra1
         - image/ubuntu-22-04-x64
       steps:
         - run: echo "Hello from ephemeral runner!"
         - run: uname -a
   ```

2. Push the workflow and check:
   - GitHub Actions tab shows the job as queued
   - dorunner logs show it received the webhook
   - A new Droplet appears in your DigitalOcean dashboard with tag `github-runner`
   - The job executes successfully
   - The Droplet is automatically deleted after completion

### Troubleshooting

**Webhook not received:**
- Verify the webhook URL is publicly accessible
- Check webhook deliveries in your repository: Settings → Webhooks → Recent Deliveries
- Ensure `X-Hub-Signature-256` validation is passing

**Droplet creation fails:**
- Verify `DO_TOKEN` has write permissions
- Check DigitalOcean API rate limits
- Ensure the region/size/image labels are valid

**Runner doesn't pick up jobs:**
- Verify your workflow's `runs-on` labels match the runner configuration (labels come from the workflow_job webhook payload)
- Check the runner appears in Settings → Actions → Runners (it should show briefly while the job runs)
- Review dorunner logs for JIT config generation errors

## Configuration

All configuration is supplied via environment variables.

| Variable | Required | Default | Description |
|---|---|---|---|
| `WEB_HOOK_SECRET` | yes | — | GitHub webhook HMAC-SHA256 secret |
| `GITHUB_APP_ID` | yes | — | GitHub App numeric ID |
| `GITHUB_APP_PRIVATE_KEY` | yes | — | PEM-encoded RSA private key for the App |
| `GITHUB_INSTALLATION_ID` | yes | — | Installation ID of the App on your org/repo |
| `GITHUB_RUNNER_GROUP_ID` | no | `1` | Runner group to register new runners into |
| `DO_TOKEN` | yes | — | DigitalOcean personal access token |
| `RUNNER_TTL` | no | `1h` | Maximum Droplet lifetime; older Droplets are force-deleted |
| `RUNNER_VERSION` | no | `2.337.0` | `actions/runner` release version to install on stock images |
| `PORT` | no | `8080` | HTTP port to listen on |

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

## Building Custom Snapshots with Packer

The `packer/` directory contains a Packer configuration for building pre-configured DigitalOcean snapshots with GitHub Actions runner binaries and common CI/CD tools pre-installed. Using snapshots significantly reduces Droplet startup time compared to stock images.

### What's Included in the Snapshot

The Packer build creates a snapshot with:

- **GitHub Actions runner** binaries (version configurable via `runner_version` variable)
- **System packages**: `build-essential`, `jq`, `yq`, `jc`, `python3.12-venv`, `unzip`
- **AWS CLI v2** (commonly used in CI/CD workflows)
- **Runner user** with passwordless sudo access
- **Runner dependencies** (dotnet, libicu, etc.) pre-installed

The snapshot does **not** include runner configuration — JIT tokens are injected at Droplet creation time via cloud-init.

### Packer Variables

| Variable | Default | Description |
|---|---|---|
| `do_token` | `$DIGITALOCEAN_TOKEN` | DigitalOcean API token (required) |
| `region` | `ams3` | Region where the build Droplet will be created |
| `droplet_size` | `s-1vcpu-1gb` | Size of the build Droplet |
| `image_name` | `ubuntu-24-04-x64` | Base OS image to build from |
| `runner_version` | `2.337.0` | GitHub Actions runner version to install |
| `runner_sha256` | checksum of `2.337.0` | SHA256 of the runner tarball; must be changed together with `runner_version` (GitHub publishes it in the release notes) |

### Building a Snapshot

1. **Install Packer** (if not already installed):
   ```bash
   # macOS
   brew install packer
   
   # Linux
   wget https://releases.hashicorp.com/packer/1.10.0/packer_1.10.0_linux_amd64.zip
   unzip packer_1.10.0_linux_amd64.zip
   sudo mv packer /usr/local/bin/
   ```

2. **Set your DigitalOcean token**:
   ```bash
   export DIGITALOCEAN_TOKEN=your_do_token_here
   ```

3. **Build the snapshot** (from the repository root):
   ```bash
   cd packer
   packer init .
   packer build digitalocean-ubuntu-2404.pkr.hcl
   ```

4. **Custom build with variables**:
   ```bash
   packer build \
     -var "region=ams3" \
     -var "runner_version=2.340.0" \
     -var "runner_sha256=<sha256 from the release notes>" \
     digitalocean-ubuntu-2404.pkr.hcl
   ```

5. **Find your snapshot**:
   - The build outputs a `manifest.json` file with snapshot details
   - Snapshot name format: `ubuntu-24-04-x64-runner-2.334.0-YYYYMMDD-HHmmss`
   - View in DigitalOcean: **Images** → **Snapshots**

### Using Your Snapshot

Once the snapshot is built, reference it in your workflow labels:

```yaml
jobs:
  my-job:
    runs-on:
      - self-hosted
      - linux
      - x64
      - region/fra1
      - size/s-2vcpu-4gb
      - snapshot/ubuntu-24-04-x64-runner-2.334.0-20260517-103000
```

**Note**: The snapshot name must match exactly. You can find the exact name in the Packer build output or in your DigitalOcean dashboard under **Images** → **Snapshots**.

### Snapshot Benefits

- **Faster startup**: Runner binaries and dependencies are pre-installed (~30-60 seconds faster)
- **Consistent environment**: Same base image for all jobs
- **Custom tooling**: Add your own tools to the Packer build script
- **Reduced bandwidth**: No need to download runner tarball on every job

### Updating Snapshots

When GitHub releases a new runner version:

1. Update `runner_version` **and** `runner_sha256` in the Packer file or pass via `-var`
2. Rebuild the snapshot
3. Update your workflow labels to reference the new snapshot name
4. Delete old snapshots from DigitalOcean to avoid charges

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
  -p 8080:8080 \
  ghcr.io/dirtydriver/dorunner:v1.0.0
```

**Note**: Replace `v1.0.0` with the specific version you want to use. Check [releases](https://github.com/dirtydriver/dorunner/releases) for available versions.

Or build locally:

```bash
docker build -t dorunner .

docker run -d \
  -e WEB_HOOK_SECRET=... \
  -e GITHUB_APP_ID=... \
  -e GITHUB_APP_PRIVATE_KEY="$(cat private-key.pem)" \
  -e GITHUB_INSTALLATION_ID=... \
  -e DO_TOKEN=... \
  -p 8080:8080 \
  dorunner
```

The image is built on `gcr.io/distroless/base:nonroot` — it contains only the statically linked binary and runs as a non-root user.

### DigitalOcean App Platform (recommended)

The easiest way to deploy dorunner is via DigitalOcean App Platform:

1. Fork or push this repository to GitHub
2. Go to [DigitalOcean App Platform](https://cloud.digitalocean.com/apps) → **Create App**
3. Connect your GitHub repository
4. Set the component type to **Web Service**
5. Add all required environment variables as **encrypted secrets**
6. Deploy — App Platform will build from the Dockerfile automatically
7. Copy the public URL (e.g. `https://dorunner-xxxxx.ondigitalocean.app`) — use this as your webhook Payload URL in Step 3.5
8. Go back to **Step 3.5** and update the webhook **Payload URL** with your App Platform URL:
   ```
   https://dorunner-xxxxx.ondigitalocean.app/webhook
   ```
   If you already created the webhook with a placeholder URL, edit it:
   **Settings → Webhooks → your webhook → Edit** and update the Payload URL.

Every push to `main` triggers an automatic redeploy with zero downtime. **Note**: Your App Platform URL is stable and won't change on redeployments — you only need to update the webhook URL if you delete and recreate the app.

### Local

Copy `.env.example` to `.env` and fill in your values, then:

```bash
go build -o dorunner .
cp .env.example .env
# edit .env with your values
export $(cat .env | xargs)
./dorunner
```

## Webhook endpoint

```
POST /webhook
```

dorunner validates the `X-Hub-Signature-256` header on every request. Configure this URL as the webhook endpoint in your repository settings (see Step 3.5 in the Installation Guide).
