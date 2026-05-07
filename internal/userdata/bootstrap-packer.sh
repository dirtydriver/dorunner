#!/bin/bash
set -e

JIT_CONFIG="{{ .JITConfig }}"

cd /home/runner/actions-runner
sudo -u runner ./config.sh --jitconfig "$JIT_CONFIG" --unattended
sudo -u runner ./run.sh
