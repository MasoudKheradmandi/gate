#!/bin/sh
# Prepares test fixtures for the development stack (run once in deploy/dev):
#   - pebble-ca.pem      : CA of the Pebble test ACME server (trusted by corvus via LEGO_CA_CERTIFICATES)
#   - import/npm/...     : a small Nginx Proxy Manager installation (SQLite + certificate files)
set -eu
cd "$(dirname "$0")"

PEBBLE_IMAGE=$(awk '/image: ghcr.io\/letsencrypt\/pebble/ {print $2}' docker-compose.yml)
docker pull -q "$PEBBLE_IMAGE" >/dev/null
cid=$(docker create "$PEBBLE_IMAGE")
docker cp "$cid:/test/certs/pebble.minica.pem" pebble-ca.pem
docker rm "$cid" >/dev/null

rm -rf import && mkdir -p import/npm/data/custom_ssl/npm-2 import/npm/letsencrypt/live/npm-1
mkcert() { # dir domain
  openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes -days 60 -subj "/CN=$2" \
    -addext "subjectAltName=DNS:$2" -keyout "$1/privkey.pem" -out "$1/fullchain.pem" 2>/dev/null
}
mkcert import/npm/letsencrypt/live/npm-1 npm-shop.corvus.test
mkcert import/npm/data/custom_ssl/npm-2 npm-custom.corvus.test

python3 - <<'PY'
import sqlite3
db = sqlite3.connect("import/npm/data/database.sqlite")
db.executescript("""
CREATE TABLE user (id INTEGER PRIMARY KEY, email TEXT, is_deleted INTEGER DEFAULT 0);
CREATE TABLE certificate (id INTEGER PRIMARY KEY, owner_user_id INTEGER, provider TEXT, nice_name TEXT, domain_names TEXT, meta TEXT, is_deleted INTEGER DEFAULT 0);
CREATE TABLE access_list (id INTEGER PRIMARY KEY, name TEXT, satisfy_any INTEGER, pass_auth INTEGER DEFAULT 1, is_deleted INTEGER DEFAULT 0);
CREATE TABLE access_list_client (id INTEGER PRIMARY KEY, access_list_id INTEGER, address TEXT, directive TEXT);
CREATE TABLE access_list_auth (id INTEGER PRIMARY KEY, access_list_id INTEGER, username TEXT, password TEXT);
CREATE TABLE proxy_host (id INTEGER PRIMARY KEY, domain_names TEXT, forward_scheme TEXT, forward_host TEXT, forward_port INTEGER,
  access_list_id INTEGER DEFAULT 0, certificate_id INTEGER DEFAULT 0, ssl_forced INTEGER, block_exploits INTEGER, allow_websocket_upgrade INTEGER,
  http2_support INTEGER, enabled INTEGER, advanced_config TEXT, locations TEXT, is_deleted INTEGER DEFAULT 0);
CREATE TABLE stream (id INTEGER PRIMARY KEY, is_deleted INTEGER DEFAULT 0);
CREATE TABLE redirection_host (id INTEGER PRIMARY KEY, is_deleted INTEGER DEFAULT 0);
INSERT INTO user VALUES (1, 'ops@corvus.test', 0);
INSERT INTO certificate VALUES (1, 1, 'letsencrypt', 'npm shop', '["npm-shop.corvus.test"]', '{}', 0);
INSERT INTO certificate VALUES (2, 1, 'other', 'npm custom', '["npm-custom.corvus.test"]', '{}', 0);
INSERT INTO certificate VALUES (3, 1, 'other', 'deleted', '["gone.corvus.test"]', '{}', 1);
INSERT INTO access_list VALUES (1, 'office', 1, 1, 0);
INSERT INTO access_list_client VALUES (1, 1, '10.99.0.0/16', 'allow');
INSERT INTO access_list_auth VALUES (1, 1, 'alice', 'alice-secret-1');
INSERT INTO proxy_host VALUES (1, '["npm-shop.corvus.test"]', 'http', 'demo', 80, 0, 1, 1, 1, 1, 1, 1, 'client_max_body_size 20m;', '[]', 0);
INSERT INTO proxy_host VALUES (2, '["npm-custom.corvus.test"]', 'http', 'demo', 80, 1, 2, 0, 0, 0, 0, 1, '', '[]', 0);
INSERT INTO proxy_host VALUES (3, '["npm-legacy.corvus.test"]', 'http', 'demo', 80, 0, 0, 0, 0, 0, 0, 1, 'proxy_pass $forward_scheme://$server:$port;', '[{"path":"/x"}]', 0);
INSERT INTO proxy_host VALUES (4, '["deleted.corvus.test"]', 'http', 'demo', 80, 0, 0, 0, 0, 0, 0, 1, '', '[]', 1);
INSERT INTO stream VALUES (1, 0);
""")
db.commit()
PY
chmod -R a+rX import
echo "fixtures ready"
