#!/bin/sh
# The Go of the SDK of OpenWrt 24.10 is too old for the backend (src/backend/go.mod asks for
# Go 1.25.5): lang/golang of the packages feed is taken at a commit of openwrt-25.12
# (Go 1.26.8), as podkop-engine builds sing-box. Run in the directory of the SDK.
set -e
REPO=https://github.com/openwrt/packages.git
COMMIT=f60a090afff932579d015045dd1eff9f744fa043
tmp=$(mktemp -d)
git -C "$tmp" init -q
git -C "$tmp" remote add origin "$REPO"
git -C "$tmp" sparse-checkout set lang/golang
git -C "$tmp" fetch -q --depth 1 --filter=blob:none origin "$COMMIT"
git -C "$tmp" checkout -q FETCH_HEAD
rm -rf feeds/packages/lang/golang
cp -a "$tmp/lang/golang" feeds/packages/lang/golang
rm -rf "$tmp"
./scripts/feeds update -i packages > /dev/null
./scripts/feeds install -p packages golang > /dev/null
