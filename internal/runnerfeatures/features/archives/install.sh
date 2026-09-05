#!/bin/sh
set -eu
apt-get update
apt-get install -y --no-install-recommends unzip zip xz-utils bzip2 zstd file
rm -rf /var/lib/apt/lists/*
