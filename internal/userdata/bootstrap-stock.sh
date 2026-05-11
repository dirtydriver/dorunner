#!/bin/bash
set -e

JIT_CONFIG="{{ .JITConfig }}"
RUNNER_VERSION="{{ .RunnerVersion }}"

# Create runner user
useradd -m -s /bin/bash runner

# Create folder and download runner
sudo -u runner mkdir -p /home/runner/actions-runner
cd /home/runner/actions-runner

# allow runner user to use sudo without password
echo "runner ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/runner
chmod 440 /etc/sudoers.d/runner

# Download and extract
sudo -u runner curl -o actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz -L \
  https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz
sudo -u runner tar xzf ./actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz

# Configure and start via environment variable
export ACTIONS_RUNNER_INPUT_JITCONFIG="$JIT_CONFIG"
sudo -u runner --preserve-env=ACTIONS_RUNNER_INPUT_JITCONFIG /home/runner/actions-runner/run.sh