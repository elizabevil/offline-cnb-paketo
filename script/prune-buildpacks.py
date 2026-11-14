#!/usr/bin/env python3
"""按 order 裁剪 bundle/buildpacks，只保留指定语言及其依赖。

用法: script/prune-buildpacks.py [--dry-run] [语言…]
默认保留 go、nodejs、python（python 的 order 已包含 pip 与 poetry 路径）。
lifecycle 等组件不在此范围——它们继续由离线物料提供。
"""
import os
import re
import shutil
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
BP = os.path.join(ROOT, "bundle", "buildpacks")
LIB = os.path.join(BP, "buildpacks")
ORDER = os.path.join(BP, "order.toml")

DEFAULT_KEEP = ["go", "nodejs", "python"]


def dir_of(bp_id: str) -> str:
    leaf = bp_id.replace("paketo-buildpacks/", "")
    return os.path.join(LIB, "paketo-buildpacks_" + leaf)


def versions_of(bp_id: str):
    d = dir_of(bp_id)
    if not os.path.isdir(d):
        return []
    return sorted(v for v in os.listdir(d) if os.path.isdir(os.path.join(d, v)))


def order_groups(bp_dir: str):
    """读取 meta buildpack 的 order，返回 [(id, version)]。"""
    toml_path = os.path.join(bp_dir, "buildpack.toml")
    try:
        text = open(toml_path, encoding="utf-8").read()
    except OSError:
        return []
    groups = []
    cur = {}
    for line in text.splitlines():
        m = re.match(r'\s*id = "([^"]+)"', line)
        if m:
            cur["id"] = m.group(1)
        m = re.match(r'\s*version = "([^"]+)"', line)
        if m and "id" in cur:
            cur["version"] = m.group(1)
            groups.append((cur["id"], cur["version"]))
            cur = {}
    return groups


def collect(bp_id, version, keep: set):
    d = os.path.join(dir_of(bp_id), version)
    if not os.path.isdir(d):
        return
    if d in keep:
        return
    keep.add(d)
    for cid, cver in order_groups(d):
        collect(cid, cver, keep)


def main():
    args = [a for a in sys.argv[1:] if not a.startswith("-")]
    dry = "--dry-run" in sys.argv
    langs = args or DEFAULT_KEEP

    if not os.path.isdir(LIB):
        sys.exit(f"未找到 {LIB}（先运行 just extract-builder）")

    keep = set()
    for lang in langs:
        vs = versions_of(f"paketo-buildpacks/{lang}")
        if not vs:
            sys.exit(f"未找到语言 buildpack: {lang}")
        for v in vs:
            collect(f"paketo-buildpacks/{lang}", v, keep)

    print(f"保留 {len(keep)} 个 buildpack（语言: {', '.join(langs)}）")

    # 重写 order.toml：只保留被选语言的 order 块
    text = open(ORDER, encoding="utf-8").read()
    blocks = re.split(r'(?m)^(?=\[\[order\]\])', text)
    kept_blocks = []
    for b in blocks:
        m = re.search(r'id = "paketo-buildpacks/([^"]+)"', b)
        if m and m.group(1) in langs:
            kept_blocks.append(b.strip() + "\n")
    if dry:
        print(f"order.toml 将保留 {len(kept_blocks)} 个 order 块")
    else:
        open(ORDER, "w", encoding="utf-8").write("\n".join(kept_blocks))
        print(f"order.toml 已重写（{len(kept_blocks)} 个 order）")

    # 删除未保留目录
    removed = 0
    for name in sorted(os.listdir(LIB)):
        p = os.path.join(LIB, name)
        if not os.path.isdir(p):
            continue
        for ver in os.listdir(p):
            vp = os.path.join(p, ver)
            if vp not in keep:
                if dry:
                    print("  将删除", os.path.relpath(vp, LIB))
                else:
                    shutil.rmtree(vp, ignore_errors=True)
                removed += 1
        if not dry and not os.listdir(p):
            os.rmdir(p)

    size = shutil.disk_usage(LIB)
    print(f"{'将删除' if dry else '已删除'} {removed} 个 buildpack 目录")
    print("提示：Dockerfile 需保留 assets 提供的 lifecycle")


if __name__ == "__main__":
    main()
