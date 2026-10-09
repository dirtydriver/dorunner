#!/bin/bash
# Node.js — the runner software bundles Node, but a full host install is
# required (a stripped container is not enough). Installed via NodeSource.
set -euxo pipefail

: "${NODE_MAJOR:=22}"

curl -fsSL "https://deb.nodesource.com/setup_${NODE_MAJOR}.x" | bash -
apt-get install -y nodejs

node --version
npm --version
