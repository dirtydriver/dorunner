#!/bin/bash
# Rust toolchain (rustup + cargo + rustfmt + clippy + cargo-audit) installed
# system-wide, plus Zig for cross-compilation (Lambda artifacts).
set -euxo pipefail

: "${ZIG_VERSION:=0.14.1}"

# --- Rust -------------------------------------------------------------------
export CARGO_HOME=/opt/rust/cargo
export RUSTUP_HOME=/opt/rust/rustup
# NOTE: rustup-init's --component flag takes a single value; passing multiple
# space-separated values makes it treat the extras as unexpected positional
# args. Install the minimal profile first, then add components via rustup.
curl --proto '=https' --tlsv1.2 -fsSL https://sh.rustup.rs \
  | sh -s -- -y --no-modify-path --profile minimal

# Make rustup/cargo available for the rest of this script.
export PATH="/opt/rust/cargo/bin:${PATH}"

# Add the components GitHub ships (rustfmt + clippy).
rustup component add rustfmt clippy

{
  echo 'export CARGO_HOME=/opt/rust/cargo'
  echo 'export RUSTUP_HOME=/opt/rust/rustup'
  echo 'export PATH=$PATH:/opt/rust/cargo/bin'
} > /etc/profile.d/rust.sh
chmod 644 /etc/profile.d/rust.sh

ln -sf /opt/rust/cargo/bin/* /usr/local/bin/

# Security auditing tool used in CI.
cargo install cargo-audit --locked
ln -sf /opt/rust/cargo/bin/* /usr/local/bin/

rustc --version
cargo --version
cargo fmt --version
cargo audit --version

# Make the shared cargo registry writable for the runner user.
chmod -R a+rwX /opt/rust/cargo/registry 2>/dev/null || true

# --- Zig --------------------------------------------------------------------
# Zig changed its archive naming: newer releases use "zig-x86_64-linux-<ver>"
# (arch first) while older ones used "zig-linux-x86_64-<ver>". Try the new
# scheme first and fall back to the old one.
ZIG_BASE="https://ziglang.org/download/${ZIG_VERSION}"
if ! curl -fsSL -o /tmp/zig.tar.xz "${ZIG_BASE}/zig-x86_64-linux-${ZIG_VERSION}.tar.xz"; then
  curl -fsSL -o /tmp/zig.tar.xz "${ZIG_BASE}/zig-linux-x86_64-${ZIG_VERSION}.tar.xz"
fi
mkdir -p /opt/zig
tar -xJf /tmp/zig.tar.xz -C /opt/zig --strip-components=1
rm -f /tmp/zig.tar.xz
ln -sf /opt/zig/zig /usr/local/bin/zig
zig version
