packer {
  required_plugins {
    digitalocean = {
      version = ">= 1.4.0"
      source  = "github.com/digitalocean/digitalocean"
    }
  }
}

variable "do_token" {
  type      = string
  sensitive = true
  default   = env("DIGITALOCEAN_TOKEN")
}

variable "region" {
  type    = string
  default = "ams3"
}

variable "droplet_size" {
  type    = string
  default = "s-1vcpu-1gb"
}

variable "image_name" {
  type    = string
  default = "ubuntu-24-04-x64"
}

variable "runner_version" {
  type    = string
  default = "2.334.0"
}

locals {
  timestamp  = formatdate("YYYYMMDD-HHmmss", timestamp())
  final_name = "${var.image_name}-runner-${var.runner_version}-${local.timestamp}"
}

source "digitalocean" "ubuntu_2404" {
  api_token     = var.do_token
  image         = var.image_name
  region        = var.region
  size          = var.droplet_size
  snapshot_name = local.final_name
  ssh_username  = "root"
}

build {
  name    = "github-runner-ubuntu-2404"
  sources = ["source.digitalocean.ubuntu_2404"]

  # Wait for cloud-init to finish before provisioning
  provisioner "shell" {
    inline = [
      "cloud-init status --wait"
    ]
  }

  # System packages
  provisioner "shell" {
    inline = [
      "export DEBIAN_FRONTEND=noninteractive",
      "apt-get update -y",
      "apt-get install -y build-essential jq yq jc python3.12-venv unzip",
    ]
  }

  # AWS CLI v2 (commonly used in CI/CD workflows)
  provisioner "shell" {
    inline = [
      "cd /tmp",
      "curl -fsSL 'https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip' -o awscliv2.zip",
      "unzip -q awscliv2.zip",
      "./aws/install",
      "aws --version",
      "rm -rf awscliv2.zip aws/",
    ]
  }

  # GitHub CLI (gh) for interacting with GitHub API and repositories
  provisioner "shell" {
    inline = [
      "cd /tmp",
      # Add GitHub CLI official apt repository
      "curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg",
      "chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg",
      "echo 'deb [arch=amd64 signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main' | tee /etc/apt/sources.list.d/github-cli.list > /dev/null",
      # Install gh
      "apt-get update -y",
      "apt-get install -y gh",
      "gh --version",
    ]
  }

  # Docker Engine (for containerized workflows)
  provisioner "shell" {
    inline = [
      "cd /tmp",
      # Add Docker's official GPG key
      "install -m 0755 -d /etc/apt/keyrings",
      "curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc",
      "chmod a+r /etc/apt/keyrings/docker.asc",
      # Add Docker repository
      "echo \"deb [arch=amd64 signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo \"$VERSION_CODENAME\") stable\" | tee /etc/apt/sources.list.d/docker.list > /dev/null",
      # Install Docker Engine, CLI, containerd, and plugins
      "apt-get update -y",
      "apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin",
      # Verify installation
      "docker --version",
      "docker compose version",
      # Enable Docker service
      "systemctl enable docker",
    ]
  }

  # Create runner user with passwordless sudo
  provisioner "shell" {
    inline = [
      "useradd -m -s /bin/bash runner",
      "echo 'runner ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/runner",
      "chmod 440 /etc/sudoers.d/runner",
      # Add runner to docker group for passwordless docker access
      "usermod -aG docker runner",
    ]
  }

  # Download and extract GitHub Actions runner
  # NOTE: No configure/run here — JIT config is injected at Droplet boot time
  provisioner "shell" {
    inline = [
      "sudo -u runner mkdir -p /home/runner/actions-runner",
      "cd /home/runner/actions-runner",
      # Download runner tarball
      "sudo -u runner curl -fsSL -o actions-runner-linux-x64-${var.runner_version}.tar.gz https://github.com/actions/runner/releases/download/v${var.runner_version}/actions-runner-linux-x64-${var.runner_version}.tar.gz",
      # Extract
      "sudo -u runner tar xzf actions-runner-linux-x64-${var.runner_version}.tar.gz",
      # Cleanup
      "rm -f actions-runner-linux-x64-${var.runner_version}.tar.gz",
      # Install runner system dependencies (dotnet, libicu, etc.)
      "/home/runner/actions-runner/bin/installdependencies.sh",
    ]
  }

  post-processor "manifest" {
    output     = "manifest.json"
    strip_path = true
  }
}
