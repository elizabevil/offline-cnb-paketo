# Non-root 测试矩阵

> Outer root = non-root。Inner root / Builder 单独列出。

## Host

| ID | Runtime | Platform | Inner root | Builder | Host mapping | Export | Status | Artifact | Reason |
|---|---|---|---|---|---|---|---|---|---|
| H-HOSTDOCKER-AMD64-ROOTLESS-ROOT-DAEMON | host-docker | amd64 | non-root | root | debi | daemon | pass | `s-mart-host-daemon-rootless-amd64.tar.gz` | — |
| H-HOSTDOCKER-ARM64-ROOTLESS-ROOT-DAEMON | host-docker | arm64 | non-root | root | debi | daemon | fail | — | singleton |
| H-HOSTDOCKER-AMD64-ROOTLESS-NONROOT-DAEMON | host-docker | amd64 | non-root | non-root | debi | daemon | fail | — | singleton |
| H-HOSTDOCKER-ARM64-ROOTLESS-NONROOT-DAEMON | host-docker | arm64 | non-root | non-root | debi | daemon | not-run | — | 未执行 |
| H-HOSTPODMAN-AMD64-ROOTLESS-ROOT-LAYOUT | host-podman | amd64 | non-root | root | debi | layout | pass | `s-mart-host-layout-amd64.tar.gz` | — |
| H-HOSTPODMAN-ARM64-ROOTLESS-ROOT-LAYOUT | host-podman | arm64 | non-root | root | debi | layout | pass | `s-mart-host-layout-arm64.tar.gz` | — |
| H-HOSTPODMAN-AMD64-ROOTLESS-NONROOT-LAYOUT | host-podman | amd64 | non-root | non-root | debi | layout | pass | `s-mart-host-nonroot-layout-amd64.tar.gz` | — |
| H-HOSTPODMAN-ARM64-ROOTLESS-NONROOT-LAYOUT | host-podman | arm64 | non-root | non-root | debi | layout | pass | `s-mart-host-nonroot-layout-arm64.tar.gz` | — |

## Managed

| ID | Runtime | Platform | Outer | Inner | Inner root | Builder | Host mapping | Export | Status | Artifact | Reason |
|---|---|---|---|---|---|---|---|---|---|---|---|
| M-PODMAN-PIND-AMD64-ROOTLESS-ROOTLESS-NONROOT-LAYOUT | pind | amd64 | podman | pind | non-root | non-root | debi | layout | pass | `s-mart-pind-layout-amd64.tar.gz` | — |
| M-PODMAN-PIND-AMD64-ROOTLESS-ROOTLESS-NONROOT-DAEMON | pind | amd64 | podman | pind | non-root | non-root | debi | daemon | pass | `s-mart-pind-daemon-amd64.tar.gz` | — |
| M-PODMAN-PIND-ARM64-ROOTLESS-ROOTLESS-NONROOT-LAYOUT | pind | arm64 | podman | pind | non-root | non-root | debi | layout | blocked | — | qemu robust lock |
| M-PODMAN-PIND-ARM64-ROOTLESS-ROOTLESS-NONROOT-DAEMON | pind | arm64 | podman | pind | non-root | non-root | debi | daemon | blocked | — | qemu robust lock |
| M-PODMAN-DIND-AMD64-ROOTLESS-ROOTLESS-NONROOT-LAYOUT | dind | amd64 | podman | dind | non-root | non-root | debi | layout | pass | `s-mart-dind-layout-amd64.tar.gz` | — |
| M-PODMAN-DIND-AMD64-ROOTLESS-ROOTLESS-NONROOT-DAEMON | dind | amd64 | podman | dind | non-root | non-root | debi | daemon | pass | `s-mart-dind-daemon-amd64.tar.gz` | — |
| M-PODMAN-DIND-ARM64-ROOTLESS-ROOTLESS-NONROOT-LAYOUT | dind | arm64 | podman | dind | non-root | non-root | debi | layout | pass | `s-mart-dind-layout-arm64.tar.gz` | — |
| M-PODMAN-DIND-ARM64-ROOTLESS-ROOTLESS-NONROOT-DAEMON | dind | arm64 | podman | dind | non-root | non-root | debi | daemon | pass | `s-mart-dind-daemon-arm64.tar.gz` | — |
| M-DOCKER-ROOTLESS-PIND-AMD64-ROOTLESS-ROOTLESS-ROOT-LAYOUT | pind | amd64 | docker | pind | non-root | root | debi | layout | pass | `s-mart-rootless-docker-pind-layout-amd64.tar.gz` | — |
| M-DOCKER-ROOTLESS-PIND-AMD64-ROOTLESS-ROOTLESS-ROOT-DAEMON | pind | amd64 | docker | pind | non-root | root | debi | daemon | pass | `s-mart-rootless-docker-pind-daemon-amd64.tar.gz` | — |
| M-DOCKER-ROOTLESS-DIND-AMD64-ROOTLESS-ROOTLESS-ROOT-LAYOUT | dind | amd64 | docker | dind | non-root | root | debi | layout | pass | `s-mart-rootless-docker-dind-layout-amd64.tar.gz` | — |
| M-DOCKER-ROOTLESS-DIND-AMD64-ROOTLESS-ROOTLESS-ROOT-DAEMON | dind | amd64 | docker | dind | non-root | root | debi | daemon | pass | `s-mart-rootless-docker-dind-daemon-amd64.tar.gz` | — |
| M-DOCKER-ROOTLESS-PIND-ARM64-ROOTLESS-ROOTLESS-ROOT-LAYOUT | pind | arm64 | docker | pind | non-root | root | debi | layout | blocked | — | sidecar 退出 |
| M-DOCKER-ROOTLESS-PIND-ARM64-ROOTLESS-ROOTLESS-ROOT-DAEMON | pind | arm64 | docker | pind | non-root | root | debi | daemon | blocked | — | sidecar 退出 |
| M-DOCKER-ROOTLESS-DIND-ARM64-ROOTLESS-ROOTLESS-ROOT-LAYOUT | dind | arm64 | docker | dind | non-root | root | debi | layout | pass | `s-mart-rootless-docker-dind-layout-arm64.tar.gz` | — |
| M-DOCKER-ROOTLESS-DIND-ARM64-ROOTLESS-ROOTLESS-ROOT-DAEMON | dind | arm64 | docker | dind | non-root | root | debi | daemon | fail | — | singleton |

