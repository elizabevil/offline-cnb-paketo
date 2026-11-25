#!/usr/bin/env python3
"""D6 真实测试包：base 镜像 + 每模块 Procfile/metadata 薄层。

用法:
  python3 script/tools/d6_thin_package.py input-s-mart.tar.gz --out out.tar.gz
"""
import argparse
import gzip
import hashlib
import io
import json
import os
import re
import shutil
import tarfile
import tempfile

import yaml


def sha(b):
    return hashlib.sha256(b).hexdigest()


def put_blob(d, data):
    h = sha(data)
    open(os.path.join(d, "blobs", "sha256", h), "wb").write(data)
    return "sha256:" + h


def make_layer(module, processes, base_metadata):
    prefix = base_metadata.split("[[processes]]")[0]
    lines = [prefix.rstrip(), ""]
    for name, cmd in processes:
        lines += [
            "[[processes]]",
            f'  type = "{name}"',
            f'  command = [{json.dumps(cmd)}]',
            "  direct = false",
            '  buildpack-id = "paketo-buildpacks/procfile"',
            "",
        ]
    metadata = "\n".join(lines)
    procfile = "\n".join(f"{n}: {c}" for n, c in processes) + "\n"

    raw = io.BytesIO()
    with tarfile.open(fileobj=raw, mode="w") as t:
        def add(name, data, mode=0o644):
            ti = tarfile.TarInfo(name); ti.size = len(data); ti.mode = mode
            t.addfile(ti, io.BytesIO(data))
        add("app/Procfile", procfile.encode())
        add("layers/config/metadata.toml", metadata.encode())
        seen = set()
        for n, _ in processes:
            if n in seen: continue
            seen.add(n)
            ti = tarfile.TarInfo(f"cnb/process/{n}")
            ti.type = tarfile.SYMTYPE
            ti.linkname = "/cnb/lifecycle/launcher"
            ti.mode = 0o777
            t.addfile(ti)
    raw_b = raw.getvalue()
    gz_b = gzip.compress(raw_b, 6)
    return gz_b, raw_b


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("package")
    ap.add_argument("--out", required=True)
    args = ap.parse_args()
    work = tempfile.mkdtemp(prefix="d6thin-")
    try:
        with tarfile.open(args.package, "r:gz") as t:
            t.extractall(work)
        desc = yaml.safe_load(open(os.path.join(work, "app_desc.yaml"), encoding="utf-8"))
        app = (desc.get("app") or {})
        app_code = app.get("bkAppCode", "")
        version = desc.get("appVersion", "")
        modules = desc["modules"]
        base_name = modules[0]["name"]
        base_tgz = os.path.join(work, base_name + ".tgz")
        layout = tempfile.mkdtemp(prefix="d6thin-layout-")
        with tarfile.open(base_tgz) as t:
            t.extractall(layout)
        idx = json.load(open(os.path.join(layout, "index.json")))
        base_entry = idx["manifests"][0]
        base_manifest = json.load(open(os.path.join(layout, "blobs", "sha256", base_entry["digest"].split(":")[1])))
        base_cfg_digest = base_manifest["config"]["digest"]
        base_cfg = json.load(open(os.path.join(layout, "blobs", "sha256", base_cfg_digest.split(":")[1])))
        base_meta = ""
        for l in base_manifest["layers"]:
            p = os.path.join(layout, "blobs", "sha256", l["digest"].split(":")[1])
            with tarfile.open(p) as t:
                for n in t.getnames():
                    if n.endswith("layers/config/metadata.toml"):
                        base_meta = t.extractfile(n).read().decode(); break
            if base_meta: break
        entries, dman = [], []
        for m in modules:
            name = m["name"]
            procs = [(p["name"], p["procCommand"]) for p in m["spec"].get("processes", [])]
            gz, raw = make_layer(name, procs, base_meta)
            gz_d = put_blob(layout, gz)
            raw_d = "sha256:" + sha(raw)
            cfg = json.loads(json.dumps(base_cfg))
            cfg.setdefault("rootfs", {}).setdefault("diff_ids", []).append(raw_d)
            cfg.setdefault("history", []).append({"created": "1970-01-01T00:00:00Z", "created_by": "d6-thin"})
            cfg_b = json.dumps(cfg, separators=(",", ":")).encode()
            cfg_d = put_blob(layout, cfg_b)
            man = json.loads(json.dumps(base_manifest))
            man["config"]["digest"] = cfg_d
            man["config"]["size"] = len(cfg_b)
            man["layers"].append({"mediaType": "application/vnd.oci.image.layer.v1.tar+gzip", "digest": gz_d, "size": len(gz)})
            man_b = json.dumps(man, separators=(",", ":")).encode()
            man_d = put_blob(layout, man_b)
            tag = f"{app_code}/{name}:{version}"
            entries.append({"mediaType": base_entry["mediaType"], "digest": man_d, "size": len(man_b),
                            "annotations": {"io.containerd.image.name": "docker.io/" + tag,
                                            "org.opencontainers.image.ref.name": version}})
            dman.append({"Config": "blobs/sha256/" + cfg_d.split(":")[1], "RepoTags": [tag],
                         "Layers": ["blobs/sha256/" + l["digest"].split(":")[1] for l in man["layers"]]})
        idx["manifests"] = entries
        json.dump(idx, open(os.path.join(layout, "index.json"), "w"))
        json.dump(dman, open(os.path.join(layout, "manifest.json"), "w"), separators=(",", ":"))
        json.dump({f"{app_code}/{m['name']}": {version: e["digest"].split(":")[1]}
                   for m, e in zip(modules, entries)}, open(os.path.join(layout, "repositories"), "w"))
        # one tgz, hardlinked for every module
        multi = os.path.join(work, "_multi.tgz")
        with tarfile.open(multi, "w") as t:
            for n in sorted(os.listdir(layout)):
                t.add(os.path.join(layout, n), arcname=n, recursive=True)
        for m in modules:
            target = os.path.join(work, m["name"] + ".tgz")
            if os.path.exists(target): os.remove(target)
            os.link(multi, target)
        os.remove(multi)
        with tarfile.open(args.out, "w:gz") as t:
            for n in sorted(os.listdir(work)):
                t.add(os.path.join(work, n), arcname=n, recursive=False)
        print("out:", args.out, f"{os.path.getsize(args.out)/1048576:.1f} MB")
    finally:
        shutil.rmtree(work, ignore_errors=True)


if __name__ == "__main__":
    main()
