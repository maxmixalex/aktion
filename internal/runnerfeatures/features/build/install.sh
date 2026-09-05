#!/bin/sh
set -eu
apt-get update
apt-get install -y --no-install-recommends build-essential pkg-config cmake file
rm -rf /var/lib/apt/lists/*
