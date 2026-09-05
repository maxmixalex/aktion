#!/bin/sh
set -eu
apt-get update
apt-get install -y --no-install-recommends \
  ca-certificates wget openssh-client rsync gnupg git-lfs
rm -rf /var/lib/apt/lists/*
