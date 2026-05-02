#!/bin/bash
set -e
JIT_CONFIG="{{ .JITConfig }}"
RUNNER_VERSION="{{ .RunnerVersion }}"



# Create a user
useradd -m -s /bin/bash runner
# Create a folder
sudo -u runner mkdir -p /home/runner/actions-runner
cd /home/runner/actions-runner
# Download the latest run ner package
sudo -u runner curl -O -L https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz
# Extract the installer
sudo -u runner tar xzf ./actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz


sudo -u runner ./config.sh --jitconfig "$JIT_CONFIG" --unattended
sudo -u runner ./run.sh
