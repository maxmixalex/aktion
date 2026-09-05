#!/bin/sh
set -eu
apt-get update
apt-get install -y --no-install-recommends libncurses-dev
rm -rf /var/lib/apt/lists/*
