#!/usr/bin/env bash
# Prepare the k9o.dev/sdk-go/barbican nested module for the GitHub mirror by injecting the keep-core engine's
# RELEASE static archive + a build-tagged cgo LDFLAGS shim per platform, data-driven off the bundle manifest's
# `native_static_libs` (same pattern as ../kibo/gen-cgo-shims.sh).
#
#   gen-cgo-shims.sh <barbican-module-dir> <artifacts-dir> [<trust-kat.json>]
#
# Expects  <artifacts-dir>/manifest.json  (with .native_static_libs[<triple>] = [ "-l..." ... ])  and
#          <artifacts-dir>/native/staticlib/<rust-triple>/libk9o_keep_core_ffi_uniffi.a
# built WITHOUT --features test-trust-root. If a trust-kat.json is given, every archive is scanned with
# .gitlab/assert-no-test-root.sh semantics (the 32 test-root public-key bytes must be absent) and the script
# FAILS if one embeds it. If the bundle is absent the module is REMOVED from the mirror tree: never publish a
# barbican module that cannot cgo-link.
set -eu

MOD="${1:?usage: gen-cgo-shims.sh <barbican-module-dir> <artifacts-dir> [trust-kat.json]}"
ART="${2:?usage: gen-cgo-shims.sh <barbican-module-dir> <artifacts-dir> [trust-kat.json]}"
KAT="${3:-}"
MANIFEST="$ART/manifest.json"
PKG="$MOD/k9o_keep_core_ffi_uniffi"
LIB=libk9o_keep_core_ffi_uniffi.a

if [ ! -d "$ART/native/staticlib" ] || [ ! -f "$MANIFEST" ] || \
   [ "$(jq -r 'has("native_static_libs")' "$MANIFEST")" != "true" ]; then
  echo "gen-cgo-shims(barbican): bundle static archives / native_static_libs absent: removing barbican module from the go mirror."
  rm -rf "$MOD"
  exit 0
fi

# goos goarch rust-target-triple
MAP="darwin arm64 aarch64-apple-darwin
darwin amd64 x86_64-apple-darwin
linux amd64 x86_64-unknown-linux-gnu
linux arm64 aarch64-unknown-linux-gnu
windows amd64 x86_64-pc-windows-msvc"

emitted=0
while read -r goos goarch triple; do
  [ -n "$goos" ] || continue
  a="$ART/native/staticlib/$triple/$LIB"
  [ -f "$a" ] || continue
  if [ -n "$KAT" ]; then
    "$(dirname "$0")/../../../.gitlab/assert-no-test-root.sh" "$KAT" "$a" || { echo "gen-cgo-shims(barbican): $a embeds the TEST trust root"; exit 1; }
  fi
  libs="$(jq -r --arg t "$triple" '.native_static_libs[$t] // [] | join(" ")' "$MANIFEST")"
  [ -n "$libs" ] || { echo "  WARN: no native_static_libs for $triple; skipping $goos/$goarch"; continue; }

  mkdir -p "$PKG/lib/${goos}_${goarch}"
  cp "$a" "$PKG/lib/${goos}_${goarch}/$LIB"

  cat > "$PKG/cgo_${goos}_${goarch}.go" <<EOT
//go:build ${goos} && ${goarch}

package k9o_keep_core_ffi_uniffi

// #cgo LDFLAGS: \${SRCDIR}/lib/${goos}_${goarch}/$LIB ${libs}
import "C"
EOT
  echo "  cgo shim + static archive: ${goos}/${goarch} (${triple}) LDFLAGS=[${libs}]"
  emitted=$((emitted + 1))
done <<< "$MAP"

if [ "$emitted" -eq 0 ]; then
  echo "gen-cgo-shims(barbican): no platform static archives matched: removing barbican module from the go mirror."
  rm -rf "$MOD"
  exit 0
fi
rm -f "$MOD/gen-cgo-shims.sh"
echo "gen-cgo-shims(barbican): prepared $emitted platform(s) for the go barbican mirror."
