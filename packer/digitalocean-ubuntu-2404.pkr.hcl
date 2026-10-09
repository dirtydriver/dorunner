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
  default = "s-2vcpu-4gb"
}

variable "image_name" {
  type    = string
  default = "ubuntu-24-04-x64"
}

variable "runner_version" {
  type    = string
  default = "2.337.0"
}

variable "node_major" {
  type    = string
  default = "22"
}

variable "zig_version" {
  type    = string
  default = "0.14.1"
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

  # Wait for cloud-init to finish before provisioning.
  provisioner "shell" {
    inline = ["cloud-init status --wait"]
  }

  # Base packages: build-essential, git, curl, ca-certificates, jq, yq,
  # unzip/zip and the shell baseline (bash/coreutils/grep/sed/findutils/gawk).
  provisioner "shell" {
    script           = "scripts/00-base-packages.sh"
    environment_vars = ["DEBIAN_FRONTEND=noninteractive"]
  }

  # Node.js (full host install via NodeSource).
  provisioner "shell" {
    script = "scripts/10-node.sh"
    environment_vars = [
      "DEBIAN_FRONTEND=noninteractive",
      "NODE_MAJOR=${var.node_major}",
    ]
  }

  # Rust (rustup + cargo + cargo-audit + rustfmt) and Zig cross-build toolchain.
  provisioner "shell" {
    script           = "scripts/15-rust.sh"
    environment_vars = ["ZIG_VERSION=${var.zig_version}"]
  }

  # Cloud / deploy CLIs: AWS CLI v2, Azure CLI, GitHub CLI.
  provisioner "shell" {
    script           = "scripts/20-cloud-clis.sh"
    environment_vars = ["DEBIAN_FRONTEND=noninteractive"]
  }

  # Docker Engine (CLI, containerd, Buildx, Compose).
  provisioner "shell" {
    script           = "scripts/21-docker.sh"
    environment_vars = ["DEBIAN_FRONTEND=noninteractive"]
  }

  # Runner user with passwordless sudo, added to docker group.
  provisioner "shell" {
    script = "scripts/90-runner-user.sh"
  }

  # GitHub Actions runner binaries (JIT config injected at boot, not here).
  provisioner "shell" {
    script           = "scripts/91-actions-runner.sh"
    environment_vars = ["RUNNER_VERSION=${var.runner_version}"]
  }

  # Final cleanup to shrink the snapshot.
  provisioner "shell" {
    script           = "scripts/99-cleanup.sh"
    environment_vars = ["DEBIAN_FRONTEND=noninteractive"]
  }

  post-processor "manifest" {
    output     = "manifest.json"
    strip_path = true
  }
}
