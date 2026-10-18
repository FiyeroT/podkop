#!/usr/bin/env python3
"""The build matrix of the package podkop: which architectures, in which image openwrt/sdk.

podkop is built for every architecture (pkgarch) the release of OpenWrt has a package
sing-box for: the architectures where OpenWrt builds Go programs. ipk in the SDK of
--ipk-release (opkg), apk in the SDK of --apk-release (apk).

An SDK is made for a target/subtarget: for each pkgarch the first target (alphabetically)
whose packages have that architecture and that has an image openwrt/sdk is taken.

  sdk/gen-matrix.py --ipk-release 24.10.8 --apk-release 25.12.5 [--only x86_64,mipsel_24kc] > matrix.json
"""
import argparse
import json
import re
import sys
import urllib.request
from concurrent.futures import ThreadPoolExecutor

DL = "https://downloads.openwrt.org"
HUB = "https://hub.docker.com/v2/repositories/openwrt/sdk/tags/"


def get(url, tries=3):
    for i in range(tries):
        try:
            with urllib.request.urlopen(url, timeout=30) as r:
                return r.read().decode()
        except Exception:
            if i == tries - 1:
                raise


def subdirs(url):
    return sorted(set(re.findall(r'href="([a-z0-9_.-]+)/"', get(url))) - {"..", "."})


def image_exists(tag):
    try:
        urllib.request.urlopen(HUB + tag, timeout=30)
        return True
    except Exception:
        return False


def pkgarch_targets(release):
    """pkgarch -> [(target, subtarget)], from profiles.json of each subtarget"""
    root = f"{DL}/releases/{release}/targets"
    pairs = [(t, s) for t in subdirs(root + "/") for s in subdirs(f"{root}/{t}/")]

    def arch(pair):
        # some targets (malta) have no profiles.json: then the architecture comes from the
        # index of the target's packages (ipk: Packages, apk: index.json)
        t, s = pair
        d = f"{root}/{t}/{s}"
        try:
            return pair, json.loads(get(f"{d}/profiles.json"))["arch_packages"]
        except Exception:
            pass
        try:
            return pair, json.loads(get(f"{d}/packages/index.json"))["architecture"]
        except Exception:
            pass
        try:
            m = re.search(r"^Architecture: (\S+)", get(f"{d}/packages/Packages"), re.M)
            return pair, m.group(1) if m else None
        except Exception:
            return pair, None

    out = {}
    with ThreadPoolExecutor(16) as ex:
        for pair, a in ex.map(arch, pairs):
            if a:
                out.setdefault(a, []).append(pair)
    return {a: sorted(v) for a, v in out.items()}


def singbox_arches(release):
    root = f"{DL}/releases/{release}/packages"

    def has(a):
        return a, 'href="sing-box' in get(f"{root}/{a}/packages/")

    with ThreadPoolExecutor(16) as ex:
        return {a for a, ok in ex.map(has, subdirs(root + "/")) if ok}


def pick(release, arches):
    targets = pkgarch_targets(release)
    rows = []
    for a in sorted(arches):
        for t, s in targets.get(a, []):
            tag = f"{t}-{s}-v{release}"
            if image_exists(tag):
                rows.append({"pkgarch": a, "release": release, "image": f"openwrt/sdk:{tag}"})
                break
        else:
            print(f"warning: no SDK image for {a} in {release}", file=sys.stderr)
    return rows


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--ipk-release", required=True)
    ap.add_argument("--apk-release", required=True)
    ap.add_argument("--only", help="comma-separated pkgarchs (for test runs)")
    args = ap.parse_args()
    only = set(args.only.split(",")) if args.only else None

    matrix = []
    for fmt, release in (("ipk", args.ipk_release), ("apk", args.apk_release)):
        arches = singbox_arches(release)
        if only:
            arches &= only
        matrix += [dict(r, format=fmt) for r in pick(release, arches)]
    if not matrix:
        sys.exit("no architecture to build")
    json.dump({"include": matrix}, sys.stdout, indent=1)
    print(f"{len(matrix)} jobs", file=sys.stderr)


if __name__ == "__main__":
    main()
