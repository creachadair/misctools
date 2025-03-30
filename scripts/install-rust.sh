#!/usr/bin/env bash
#
# Install a Rust toolchain.
# Assumes Linux, macOS, or similar.
#
set -euo pipefail

base=/usr/local/rust
if [[ ! -d "$base" ]] ; then
    sudo mkdir -p "$base"
    sudo chown "$(id -u):$(id -g)" "$base"
fi
export RUSTUP_HOME="${base}/rustup"
export CARGO_HOME="${base}/cargo"
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs |
    bash -s -- --no-modify-path -y
