set -u
cd /home/debi/IdeaProjects/Learn-cnb-paketo

APP=/home/debi/Desktop/cmdb-saas/release-amd64
T=/home/debi/IdeaProjects/Learn-cnb-paketo/.cache/trials
mkdir -p "$T"

echo "===== 0) 重建工具 ====="
just tools || exit 1

PB=/home/debi/IdeaProjects/Learn-cnb-paketo/project/offline-paketo-bk

echo "===== 1) layout 模式 ====="
cd "$PB"
just smart "$APP" > "$T/layout.log" 2>&1
LAYOUT_EXIT=$?
echo "layout exit=$LAYOUT_EXIT"
cp -f dist/s-mart.tar.gz "$T/layout.tar.gz" 2>/dev/null || echo "  (无产物)"

echo "===== 2) daemon 模式 ====="
BKS_EXPORT_MODE=daemon just smart "$APP" > "$T/daemon.log" 2>&1
DAEMON_EXIT=$?
echo "daemon exit=$DAEMON_EXIT"
cp -f dist/s-mart.tar.gz "$T/daemon.tar.gz" 2>/dev/null || echo "  (无产物)"

echo "===== 3) 对比 ====="
python3 - "$T" <<'PY'
import json, os, subprocess, sys, tarfile

T = sys.argv[1]

def load(pkg, out):
    os.makedirs(out, exist_ok=True)
    with tarfile.open(pkg) as tf:
        tf.extract("web.tgz", out, filter="data")
    with tarfile.open(os.path.join(out, "web.tgz")) as tf:
        tf.extractall(out, filter="data")
    return out

def report(label, pkg):
    print(f"\n== {label} ==")
    if not os.path.exists(pkg):
        print("  产物不存在")
        return
    print("  包大小: %.1f MB" % (os.path.getsize(pkg) / 1e6))
    d = load(pkg, os.path.join(T, label))
    web = os.path.join(d, "web.tgz")
    with open(web, "rb") as f:
        head = f.read(4)
    print("  web.tgz 首字节:", head, "(gzip)" if head[:2] == b"\x1f\x8b" else "(tar)")
    print("  顶层条目:", sorted(os.listdir(d))[:8])

    man_path = os.path.join(d, "manifest.json")
    if not os.path.exists(man_path):
        print("  无 manifest.json")
        return
    man = json.load(open(man_path))[0]
    print("  RepoTags:", man.get("RepoTags"), "层数:", len(man.get("Layers", [])))
    cfg_path = os.path.join(d, man["Config"])
    if not os.path.exists(cfg_path):
        print("  config 不存在:", man["Config"]); return
    cfg = json.load(open(cfg_path))
    c = cfg.get("config", {})
    env = c.get("Env", [])
    path = next((e.split("=", 1)[1] for e in env if e.startswith("PATH=")), "(无)")
    print("  created:", "有" if "created" in cfg else "缺")
    print("  history:", len(cfg.get("history", [])))
    print("  Entrypoint:", c.get("Entrypoint"), "WorkingDir:", c.get("WorkingDir"))
    print("  PATH:", path)

report("layout", os.path.join(T, "layout.tar.gz"))
report("daemon", os.path.join(T, "daemon.tar.gz"))
PY

echo
echo "===== 4) 日志里的错误（如果有）====="
grep -iE "singleton|error|failed" "$T/daemon.log" | tail -15
echo
echo "日志: $T/layout.log  $T/daemon.log"

podman pull swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/paketobuildpacks/procfile:5.15
podman pull swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/paketobuildpacks/procfile:5.15-linuxarm64