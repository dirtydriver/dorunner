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
  token         = var.do_token
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

  # Create runner user with passwordless sudo
  provisioner "shell" {
    inline = [
      "useradd -m -s /bin/bash runner",
      "echo 'runner ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/runner",
      "chmod 440 /etc/sudoers.d/runner",
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
      # Verify checksum (GitHub provides SHA256 checksums for each release)
      "echo 'Verifying runner package integrity...'",
      "sudo -u runner curl -fsSL -o actions-runner-linux-x64-${var.runner_version}.tar.gz.sha256 https://github.com/actions/runner/releases/download/v${var.runner_version}/actions-runner-linux-x64-${var.runner_version}.tar.gz.sha256",
      "sudo -u runner sha256sum -c actions-runner-linux-x64-${var.runner_version}.tar.gz.sha256",
      # Extract
      "sudo -u runner tar xzf actions-runner-linux-x64-${var.runner_version}.tar.gz",
      # Cleanup
      "rm -f actions-runner-linux-x64-${var.runner_version}.tar.gz actions-runner-linux-x64-${var.runner_version}.tar.gz.sha256",
      # Install runner system dependencies (dotnet, libicu, etc.)
      "/home/runner/actions-runner/bin/installdependencies.sh",
    ]
  }

  post-processor "manifest" {
    output     = "manifest.json"
    strip_path = true
  }
}
