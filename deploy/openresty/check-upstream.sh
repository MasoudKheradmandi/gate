#!/bin/sh
# Compares the versions pinned in build.sh with the newest upstream releases (same branch:
# OpenSSL stays on the 3.5 LTS line). Prints one line per component; exits 1 if any is
# out of date so CI can open an issue. Uses only `git ls-remote` (no API tokens).
set -eu
here=$(dirname "$0")
pinned() { sed -n "s/^$1=//p" "$here/build.sh"; }

latest() { # repo tag-prefix version-regex
  git ls-remote --tags --refs "https://github.com/$1" \
    | sed -n "s#.*refs/tags/$2##p" | grep -E "^$3\$" | sort -V | tail -1
}

stale=0
check() { # name pinned latest
  if [ -z "$3" ]; then echo "?  $1 $2 (could not determine upstream)"; stale=1
  elif [ "$2" = "$3" ]; then echo "ok $1 $2"
  else echo "!! $1 $2 -> $3"; stale=1; fi
}

check openresty "$(pinned OPENRESTY_VERSION)" "$(latest openresty/openresty v '[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+')"
check openssl   "$(pinned OPENSSL_VERSION)"   "$(latest openssl/openssl openssl- '3\.5\.[0-9]+')"
check pcre2     "$(pinned PCRE2_VERSION)"     "$(latest PCRE2Project/pcre2 pcre2- '10\.[0-9]+')"
check zlib      "$(pinned ZLIB_VERSION)"      "$(latest madler/zlib v '1\.[0-9]+(\.[0-9]+)*')"
exit $stale
