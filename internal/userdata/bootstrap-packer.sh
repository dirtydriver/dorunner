#!/bin/bash
set -e

JIT_CONFIG="{{ .JITConfig }}"

cd /home/runner/actions-runner

export ACTIONS_RUNNER_INPUT_JITCONFIG="$JIT_CONFIG"
sudo -u runner --preserve-env=ACTIONS_RUNNER_INPUT_JITCONFIG /home/runner/actions-runner/run.sh