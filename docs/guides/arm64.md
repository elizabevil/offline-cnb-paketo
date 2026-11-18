# arm64 验证指南

## 一、架构边界

| 组件 | 是否区分架构 | 说明 |
|---|---|---|
| `bundle/linux-amd64/`、`bundle/linux-arm64/` | 是 | 二进制物料按平台分目录，由 `materials sync --platform` 落盘 |
| `buildpacks/paketo/` | **否** | 全部是 `/bin/sh` 脚本，无 ELF，架构中立 |
| `project/*/docker/*.dockerfile` | 否 | 通过 `BUNDLE_ARCH` 引用平台目录，镜像本身按宿主架构构建 |
| `script/bin/cnb-<os>-<arch>` | 是 | `just tools` 按宿主自动选择，可用 `TARGET=` 覆盖 |

结论：只需要准备 `bundle/linux-arm64`，buildpack 目录无需拆分。

## 二、前置：固定 arm64 物料哈希

`assets.lock.toml` 中 arm64 条目已有 URL，但 `sha256` 为空（`materials sync` 不会自动补）。
在 arm64 机器上执行：

```bash
# 逐个下载后回填 sha256（一次性）
script/bin/cnb-linux-arm64 materials list --platform linux-arm64
# 按输出逐个下载 url，计算 sha256，写入对应 [asset.platform.linux-arm64] 的 sha256
```

`cnb materials pin` 只能由已下载的归档反推哈希并打印，不写回 lock。

## 三、在 arm64 主机验证

```bash
git clone https://gitee.com/elizabevil/Learn-cnb-paketo.git
cd Learn-cnb-paketo

just tools                              # 产出 cnb/bksmartpacker 的 arm64 二进制
script/bin/cnb-linux-arm64 materials verify --platform linux-arm64

cd project/offline-paketo-bk
CNB_BIN=../../script/bin/cnb-linux-arm64 just build --platform linux-arm64

CNB_BIN=../../script/bin/cnb-linux-arm64 \
BPS_BIN=../../script/bin/bksmartpacker-linux-arm64 \
just smart /path/to/release-arm64
```

## 四、断言

```bash
W=$(mktemp -d)
tar -xzf dist/s-mart.tar.gz -C "$W" web.tgz
mkdir -p "$W/oci" && tar -xf "$W/web.tgz" -C "$W/oci"

python3 - "$W/oci" <<'PY'
import glob, json, sys
for f in glob.glob(sys.argv[1] + "/blobs/sha256/*"):
    try:
        d = json.load(open(f))
    except Exception:
        continue
    if "architecture" in d:
        print("architecture:", d["architecture"], "| os:", d.get("os"))
PY
```

期望：`architecture: arm64`。

## 五、x86 + qemu 备选

```bash
sudo podman run --privileged --rm tonistiigi/binfmt --install arm64
podman run --rm --platform linux/arm64 debian:trixie-slim uname -m   # aarch64
```

注意：bksmartpacker 启动 builder 时目前没有 `--platform` 透传，qemu 场景需要额外参数支持。

## 六、已知阻塞

| 项 | 状态 |
|---|---|
| arm64 物料 sha256 | 未固定，`materials sync --platform linux-arm64` 会失败 |
| `bundle/linux-arm64` | 尚未生成 |
| bksmartpacker 的 `--platform` | 未支持，仅影响 qemu 交叉构建 |
