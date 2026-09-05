#!/bin/sh
set -eu
apt-get update
apt-get install -y --no-install-recommends liblua5.4-dev
rm -rf /var/lib/apt/lists/*
