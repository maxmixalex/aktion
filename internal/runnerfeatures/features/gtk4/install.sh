#!/bin/sh
set -eu
apt-get update
apt-get install -y --no-install-recommends libgtk-4-dev
rm -rf /var/lib/apt/lists/*
