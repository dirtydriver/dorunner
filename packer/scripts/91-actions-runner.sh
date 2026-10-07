#!/bin/bash
# Download and extract the GitHub Actions runner.
# NOTE: No configure/run here — JIT config is injected at Droplet boot time.
set -euxo pipefail

: "${RUNNER_VERSION:?RUNNER_VERSION must be set}"

sudo -u runner mkdir -p /home/runner/actions-runner
cd /home/runner/actions-runner

sudo -u runner curl -fsSL \
  -o "actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz" \
  "https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz"

sudo -u runner tar xzf "actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz"
rm -f "actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz"

# Install runner system dependencies (dotnet deps, libicu, etc.).
/home/runner/actions-runner/bin/installdependencies.sh
