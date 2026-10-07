#!/bin/bash
# Create the runner user with passwordless sudo and docker group access.
set -euxo pipefail

if ! id runner >/dev/null 2>&1; then
  useradd -m -s /bin/bash runner
fi

echo 'runner ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/runner
chmod 440 /etc/sudoers.d/runner

# Passwordless docker access for the runner user.
usermod -aG docker runner || true
