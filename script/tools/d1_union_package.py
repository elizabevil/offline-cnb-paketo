#!/usr/bin/env python3
"""D1 union: 一个镜像包含所有模块的进程 metadata，四个模块 tag 指向同一镜像。

用于验证：平台只加载一个镜像时，所有 process type 是否都可解析。
"""
import argparse
import gzip
import hashlib
import io
import json
import os
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


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("package")
    ap.add_argument("--out", required=True)
    args = ap.parse_args()
    work = tempfile.mkdtemp(prefix="d1union-")
    try:
        with tarfile.open(args.package, "r:gz") as t:
            t.extractall(work)
        desc = yaml.safe_load(open(os.path.join(work, "app_desc.yaml"), encoding="utf-8"))
        app = desc.get("app") or {}
        app_code = app.get("bkAppCode", "")
        version = desc.get("appVersion", "")
        modules = desc["modules"]
        # union processes，后出现的覆盖同名（web 以 pipeline 为准，便于验证 pipeline）
        procs = {}
        # 同名进程以 pipeline 为准，便于验证 pipeline 的 web/node-timeout
        ordered = [m for m in modules if m["name"] != "pipeline"] + [m for m in modules if m["name"] == "pipeline"]
        for m in ordered:
            for p in m["spec"].get("processes", []):
                procs[p["name"]] = p["procCommand"]
        base_name = modules[0]["name"]
        base_tgz = os.path.join(work, base_name + ".tgz")
        layout = tempfile.mkdtemp(prefix="d1union-layout-")
        with tarfile.open(base_tgz) as t:
            t.extractall(layout)
        idx = json.load(open(os.path.join(layout, "index.json")))
        base_entry = idx["manifests"][0]
        base_man = json.load(open(os.path.join(layout, "blobs", "sha256", base_entry["digest"].split(":")[1])))
        base_cfg = json.load(open(os.path.join(layout, "blobs", "sha256", base_man["config"]["digest"].split(":")[1])))
        base_meta = ""
        for l in base_man["layers"]:
            with tarfile.open(os.path.join(layout, "blobs", "sha256", l["digest"].split(":")[1])) as t:
                for n in t.getnames():
                    if n.endswith("layers/config/metadata.toml"):
                        base_meta = t.extractfile(n).read().decode(); break
            if base_meta: break
        prefix = base_meta.split("[[processes]]")[0].rstrip()
        lines = [prefix, ""]
        for n, cmd in procs.items():
            lines += ["[[processes]]", f'  type = "{n}"', f'  command = [{json.dumps(cmd)}]',
                      "  direct = false", '  buildpack-id = "paketo-buildpacks/procfile"', ""]
        metadata = "\n".join(lines)
        procfile = "\n".join(f"{n}: {c}" for n, c in procs.items()) + "\n"
        raw = io.BytesIO()
        with tarfile.open(fileobj=raw, mode="w") as t:
            def add(name, data, mode=0o644):
                ti = tarfile.TarInfo(name); ti.size = len(data); ti.mode = mode
                t.addfile(ti, io.BytesIO(data))
            add("app/Procfile", procfile.encode())
            add("layers/config/metadata.toml", metadata.encode())
            for n in procs:
                ti = tarfile.TarInfo(f"cnb/process/{n}"); ti.type = tarfile.SYMTYPE
                ti.linkname = "/cnb/lifecycle/launcher"; ti.mode = 0o777
                t.addfile(ti)
        gz = gzip.compress(raw.getvalue(), 6)
        gz_d = put_blob(layout, gz); raw_d = "sha256:" + sha(raw.getvalue())
        cfg = json.loads(json.dumps(base_cfg))
        cfg.setdefault("rootfs", {}).setdefault("diff_ids", []).append(raw_d)
        cfg.setdefault("history", []).append({"created": "1970-01-01T00:00:00Z", "created_by": "d1-union"})
        cb = json.dumps(cfg, separators=(",", ":")).encode(); cd = put_blob(layout, cb)
        man = json.loads(json.dumps(base_man))
        man["config"]["digest"] = cd; man["config"]["size"] = len(cb)
        man["layers"].append({"mediaType": "application/vnd.oci.image.layer.v1.tar+gzip", "digest": gz_d, "size": len(gz)})
        mb = json.dumps(man, separators=(",", ":")).encode(); md = put_blob(layout, mb)
        tags = [f"{app_code}/{m['name']}:{version}" for m in modules]
        entries = []
        for tag in tags:
            e = json.loads(json.dumps(base_entry)); e["digest"] = md; e["size"] = len(mb)
            e["annotations"]["io.containerd.image.name"] = "docker.io/" + tag
            e["annotations"]["org.opencontainers.image.ref.name"] = version
            entries.append(e)
        idx["manifests"] = entries; json.dump(idx, open(os.path.join(layout, "index.json"), "w"))
        json.dump([{"Config": "blobs/sha256/" + cd.split(":")[1], "RepoTags": tags,
                    "Layers": ["blobs/sha256/" + l["digest"].split(":")[1] for l in man["layers"]]}],
                  open(os.path.join(layout, "manifest.json"), "w"), separators=(",", ":"))
        json.dump({f"{app_code}/{m['name']}": {version: md.split(":")[1]} for m in modules},
                  open(os.path.join(layout, "repositories"), "w"))
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
