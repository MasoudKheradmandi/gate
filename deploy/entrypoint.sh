#!/bin/sh
# Runs corvus-control as the unprivileged user 10001.
#
# Started as root (the default — e.g. `docker compose up` with a ./data bind mount that
# Docker created as root), it first makes the data directory writable for that user and
# then drops privileges. Started with `user: "10001:10001"` (hardened compose), it runs
# directly. Ports 80/443 are bindable without root because Docker sets
# net.ipv4.ip_unprivileged_port_start=0 in container network namespaces.
set -e
if [ "$(id -u)" = "0" ]; then
  data="${CORVUS_DATA_DIR:-/data}"
  mkdir -p "$data"
  if [ "$(stat -c %u "$data")" != "10001" ]; then
    echo "corvus: setting owner of $data to 10001:10001"
    chown -R 10001:10001 "$data"
  fi
  exec su-exec 10001:10001 /usr/local/bin/corvus-control "$@"
fi
exec /usr/local/bin/corvus-control "$@"
