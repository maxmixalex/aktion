#!/bin/sh
set -eu
apt-get update
apt-get install -y --no-install-recommends \
  ca-certificates curl git git-lfs jq wget openssh-client rsync gnupg
rm -rf /var/lib/apt/lists/*
