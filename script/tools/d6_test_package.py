#!/usr/bin/env python3
"""D6 能力验证包：一个 tgz 内放多个 image manifest，共享 layer。

只用于验证部署平台是否接受多镜像归档；不是生产 D6 实现。
用法:
  python3 script/tools/d6_test_package.py input-s-mart.tar.gz --out out.tar.gz
"""
import argparse
import hashlib
import json
import os
import shutil
import tarfile
import tempfile

import yaml


def blob(d, data):
    h = hashlib.sha256(data).hexdigest()
    with open(os.path.join(d, "blobs", "sha256", h), "wb") as f:
        f.write(data)
    return "sha256:" + h


def build_layout(tgz, modules, app_code, version):
    tmp = tempfile.mkdtemp(prefix="d6-")
    with tarfile.open(tgz) as t:
        t.extractall(tmp)
    idx = json.load(open(os.path.join(tmp, "index.json")))
    base = idx["manifests"][0]
    md = base["digest"]
    manifest = json.load(open(os.path.join(tmp, "blobs", "sha256", md.split(":")[1])))
    config = json.load(open(os.path.join(tmp, "blobs", "sha256", manifest["config"]["digest"].split(":")[1])))
    layers = ["blobs/sha256/" + l["digest"].split(":")[1] for l in manifest["layers"]]
    entries, outer = [], []
    for name in modules:
        cfg = json.loads(json.dumps(config))
        cfg.setdefault("config", {}).setdefault("Env", []).append("MODULE=" + name)
        cb = json.dumps(cfg, separators=(",", ":")).encode()
        cd = blob(tmp, cb)
        mm = json.loads(json.dumps(manifest))
        mm["config"]["digest"] = cd
        mm["config"]["size"] = len(cb)
        mb = json.dumps(mm, separators=(",", ":")).encode()
        mdd = blob(tmp, mb)
        tag = f"{app_code}/{name}:{version}"
        entries.append({
            "mediaType": base["mediaType"], "digest": mdd, "size": len(mb),
            "annotations": {
                "io.containerd.image.name": "docker.io/" + tag,
                "org.opencontainers.image.ref.name": version,
            },
        })
        outer.append({
            "Config": "blobs/sha256/" + cd.split(":")[1],
            "RepoTags": [tag],
            "Layers": layers,
        })
    idx["manifests"] = entries
    json.dump(idx, open(os.path.join(tmp, "index.json"), "w"))
    json.dump(outer, open(os.path.join(tmp, "manifest.json"), "w"), separators=(",", ":"))
    json.dump({f"{app_code}/{n}": {version: e["digest"].split(":")[1]}
               for n, e in zip(modules, entries)},
              open(os.path.join(tmp, "repositories"), "w"))
    return tmp


def pack_dir(src, dest):
    with tarfile.open(dest, "w") as t:
        for name in sorted(os.listdir(src)):
            t.add(os.path.join(src, name), arcname=name, recursive=True)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("package")
    ap.add_argument("--out", required=True)
    args = ap.parse_args()
    work = tempfile.mkdtemp(prefix="d6-pkg-")
    try:
        with tarfile.open(args.package, "r:gz") as t:
            t.extractall(work)
        desc = yaml.safe_load(open(os.path.join(work, "app_desc.yaml"), encoding="utf-8"))
        app_code = (desc.get("app") or {}).get("bkAppCode", "")
        version = desc.get("appVersion", "")
        modules = [m["name"] for m in desc["modules"]]
        canonical = os.path.join(work, modules[0] + ".tgz")
        layout = build_layout(canonical, modules, app_code, version)
        try:
            pack_dir(layout, canonical)
        finally:
            shutil.rmtree(layout, ignore_errors=True)
        for name in modules[1:]:
            target = os.path.join(work, name + ".tgz")
            if os.path.exists(target):
                os.remove(target)
            os.link(canonical, target)
        with tarfile.open(args.out, "w:gz") as t:
            for name in sorted(os.listdir(work)):
                t.add(os.path.join(work, name), arcname=name, recursive=False)
        print("out:", args.out, f"{os.path.getsize(args.out)/1048576:.1f} MB")
    finally:
        shutil.rmtree(work, ignore_errors=True)


if __name__ == "__main__":
    main()
