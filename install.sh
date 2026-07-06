#!/bin/sh
set -eu

repo="kokoichi206/cal-slotter"
install_dir="${SLOTTER_INSTALL_DIR:-${HOME}/.local/bin}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in
  darwin | linux) ;;
  *)
    echo "unsupported OS: $os" >&2
    exit 1
    ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64 | amd64)
    arch="amd64"
    ;;
  arm64 | aarch64)
    arch="arm64"
    ;;
  *)
    echo "unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required" >&2
  exit 1
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT INT TERM

release_json="$tmp_dir/release.json"
curl -fsSL \
  -H "Accept: application/vnd.github+json" \
  "https://api.github.com/repos/${repo}/releases/latest" \
  -o "$release_json"

tag="$(sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' "$release_json" | head -n 1)"
if [ -z "$tag" ]; then
  echo "failed to resolve latest release tag" >&2
  exit 1
fi

version="${tag#v}"
asset="slotter_${version}_${os}_${arch}.tar.gz"
url="https://github.com/${repo}/releases/download/${tag}/${asset}"

curl -fsSL "$url" -o "$tmp_dir/$asset"
tar -xzf "$tmp_dir/$asset" -C "$tmp_dir" slotter

mkdir -p "$install_dir"
chmod +x "$tmp_dir/slotter"
mv "$tmp_dir/slotter" "$install_dir/slotter"

echo "slotter ${tag} installed to ${install_dir}/slotter"
