#!/bin/sh
set -eu
apt-get update
apt-get install -y --no-install-recommends \
  build-essential pkg-config cmake ninja-build autoconf automake libtool m4 bison flex
rm -rf /var/lib/apt/lists/*
