#!/bin/bash
# Base packages required by the runner and the CI shell scripts.
#   - Version-control & shell baseline: git, bash, coreutils, grep, sed,
#     findutils, gawk, unzip, zip, curl, ca-certificates
#   - Build toolchain (host compilation): build-essential (gcc/cc + binutils/ld)
#   - CLI helpers used in workflows: jq, yq
set -euxo pipefail

apt-get update -y

# Everything except yq comes from apt. yq is installed as a static binary
# below (workflows expect the mikefarah/Go yq, not the apt kislyuk/Python one).
apt-get install -y --no-install-recommends \
  build-essential \
  ca-certificates \
  coreutils \
  curl \
  findutils \
  gawk \
  git \
  gnupg \
  grep \
  jq \
  libssl-dev \
  pkg-config \
  sed \
  unzip \
  xz-utils \
  zip

# git >= 2.18 is required by actions/checkout@v7 with recursive submodules.
git --version

# yq (mikefarah, Go) — the version workflows use to read openapi.yaml.
YQ_VERSION="v4.44.3"
curl -fsSL "https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/yq_linux_amd64" \
  -o /usr/local/bin/yq
chmod +x /usr/local/bin/yq
yq --version
