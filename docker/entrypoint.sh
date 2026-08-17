#!/bin/sh

set -eu

mkdir -p /upgrade
chown heromail:heromail /upgrade
exec su-exec heromail "$@"
