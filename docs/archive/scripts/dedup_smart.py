#!/usr/bin/env python3
"""D1: 把 s-mart 中相同源码的模块合并为单镜像多 tag，并用硬链接去重。

用法:
  python3 script/tools/dedup_smart.py dist/linux-amd64/s-mart.tar.gz \
      --out dist/linux-amd64/s-mart-d1.tar.gz
"""
import argparse
import gzip
import io
import json
import os
import shutil
import tarfile
import tempfile


def load_json(path):
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def dump_json(path, data):
    with open(path, "w", encoding="utf-8") as f:
        json.dump(data, f, separators=(",", ":"))


def parse_app_desc(path):
    import yaml
    with open(path, "r", encoding="utf-8") as f:
        return yaml.safe_load(f)


def group_modules(desc):
    groups = {}
    for m in desc.get("modules", []):
        spec = m.get("spec") or {}
        build = spec.get("build") or {}
        bps = tuple(sorted(
            (bp.get("name", ""), bp.get("version", ""))
            for bp in build.get("buildpacks", [])
        ))
        key = (m.get("sourceDir", "."), m.get("language", ""), bps)
        groups.setdefault(key, []).append(m)
    return [g for g in groups.values() if len(g) > 1]


def retag_layout(tgz_path, tags):
    """给 OCI layout tar 增加多个 RepoTags，返回临时目录路径。"""
    tmp = tempfile.mkdtemp(prefix="d1-layout-")
    with tarfile.open(tgz_path) as t:
        t.extractall(tmp)
    manifest_path = os.path.join(tmp, "manifest.json")
    index_path = os.path.join(tmp, "index.json")
    repos_path = os.path.join(tmp, "repositories")
    manifest = load_json(manifest_path)
    index = load_json(index_path)
    repos = load_json(repos_path) if os.path.exists(repos_path) else {}
    base = index["manifests"][0]
    digest = base["digest"]
    manifest[0]["RepoTags"] = tags
    dump_json(manifest_path, manifest)
    entries = []
    for tag in tags:
        repo, ver = tag.rsplit(":", 1)
        e = json.loads(json.dumps(base))
        e.setdefault("annotations", {})
        e["annotations"]["io.containerd.image.name"] = "docker.io/" + tag
        e["annotations"]["org.opencontainers.image.ref.name"] = ver
        entries.append(e)
        repos.setdefault(repo, {})[ver] = digest.split(":")[1]
    index["manifests"] = entries
    dump_json(index_path, index)
    dump_json(repos_path, repos)
    return tmp


def pack_layout(src_dir, dest):
    with tarfile.open(dest, "w") as t:
        for root, dirs, files in os.walk(src_dir):
            dirs.sort()
            for name in sorted(files):
                p = os.path.join(root, name)
                arc = os.path.relpath(p, src_dir)
                t.add(p, arcname=arc, recursive=False)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("package")
    ap.add_argument("--out", required=True)
    args = ap.parse_args()

    work = tempfile.mkdtemp(prefix="d1-pkg-")
    try:
        with tarfile.open(args.package, "r:gz") as t:
            t.extractall(work)
        desc = parse_app_desc(os.path.join(work, "app_desc.yaml"))
        app = desc.get("app", {})
        app_code = app.get("bkAppCode") or app.get("code") or ""
        version = desc.get("appVersion", "")
        groups = group_modules(desc)
        if not groups:
            print("no reusable module group")
            return
        for group in groups:
            names = [m["name"] for m in group]
            first = names[0]
            canonical = os.path.join(work, first + ".tgz")
            if not os.path.exists(canonical):
                print("skip group, missing", canonical)
                continue
            tags = [f"{app_code}/{n}:{version}" for n in names]
            layout = retag_layout(canonical, tags)
            try:
                pack_layout(layout, canonical)
            finally:
                shutil.rmtree(layout, ignore_errors=True)
            for n in names[1:]:
                target = os.path.join(work, n + ".tgz")
                if os.path.exists(target):
                    os.remove(target)
                os.link(canonical, target)
            print("merged", ",".join(names), "->", canonical, f"{os.path.getsize(canonical)/1048576:.1f} MB")
        with tarfile.open(args.out, "w:gz") as t:
            for name in sorted(os.listdir(work)):
                t.add(os.path.join(work, name), arcname=name, recursive=False)
        print("out:", args.out, f"{os.path.getsize(args.out)/1048576:.1f} MB")
    finally:
        shutil.rmtree(work, ignore_errors=True)


if __name__ == "__main__":
    main()
