#!/bin/bash
# Final cleanup to reduce snapshot size and remove build-time state.
set -euxo pipefail

apt-get autoremove -y || true
apt-get clean || true

rm -rf /var/lib/apt/lists/*
rm -rf /tmp/* /var/tmp/* || true
rm -rf /root/.cache /home/runner/.cache 2>/dev/null || true

# Truncate logs and machine-id so clones get a fresh identity.
find /var/log -type f -exec truncate -s 0 {} + 2>/dev/null || true
: > /etc/machine-id || true

# Clear shell history.
rm -f /root/.bash_history /home/runner/.bash_history 2>/dev/null || true

sync
