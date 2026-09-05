#!/bin/sh
set -eu
apt-get update
apt-get install -y --no-install-recommends libssl-dev libsqlite3-dev libyaml-dev
rm -rf /var/lib/apt/lists/*
