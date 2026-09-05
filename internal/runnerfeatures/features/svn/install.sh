#!/bin/sh
set -eu
apt-get update
apt-get install -y --no-install-recommends subversion
rm -rf /var/lib/apt/lists/*
