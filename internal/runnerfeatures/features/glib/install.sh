#!/bin/sh
set -eu
apt-get update
apt-get install -y --no-install-recommends libglib2.0-dev
rm -rf /var/lib/apt/lists/*
