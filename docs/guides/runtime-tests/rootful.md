# Rootful 测试矩阵

> Outer root = root。Inner root / Builder 单独列出。

## Host

| ID | Runtime | Platform | Inner root | Builder | Host mapping | Export | Status | Artifact | Reason |
|---|---|---|---|---|---|---|---|---|---|
| H-HOSTDOCKER-AMD64-ROOT-ROOT-DAEMON | host-docker | amd64 | root | root | root | daemon | pass | `s-mart-host-daemon-amd64.tar.gz` | — |
| H-HOSTDOCKER-ARM64-ROOT-ROOT-DAEMON | host-docker | arm64 | root | root | root | daemon | fail | — | singleton |
| H-HOSTDOCKER-AMD64-ROOT-NONROOT-DAEMON | host-docker | amd64 | root | non-root | debi | daemon | fail | — | singleton |
| H-HOSTDOCKER-ARM64-ROOT-NONROOT-DAEMON | host-docker | arm64 | root | non-root | debi | daemon | not-run | — | 未执行 |

## Managed

| ID | Runtime | Platform | Outer | Inner | Inner root | Builder | Host mapping | Export | Status | Artifact | Reason |
|---|---|---|---|---|---|---|---|---|---|---|---|
| M-DOCKER-ROOTFUL-PIND-AMD64-ROOT-ROOT-ROOT-LAYOUT | pind | amd64 | docker | pind | root | root | root | layout | pass | `s-mart-docker-outer-pind-layout-amd64.tar.gz` | — |
| M-DOCKER-ROOTFUL-PIND-AMD64-ROOT-ROOT-ROOT-DAEMON | pind | amd64 | docker | pind | root | root | root | daemon | pass | `s-mart-docker-outer-pind-daemon-amd64.tar.gz` | — |
| M-DOCKER-ROOTFUL-DIND-AMD64-ROOT-ROOT-ROOT-LAYOUT | dind | amd64 | docker | dind | root | root | root | layout | pass | `s-mart-docker-outer-dind-layout-amd64.tar.gz` | — |
| M-DOCKER-ROOTFUL-DIND-AMD64-ROOT-ROOT-ROOT-DAEMON | dind | amd64 | docker | dind | root | root | root | daemon | pass | `s-mart-docker-outer-dind-daemon-amd64.tar.gz` | — |
| M-DOCKER-ROOTFUL-ROOTLESS-DIND-AMD64-ROOT-NON-ROOT-ROOT-DAEMON | dind | amd64 | docker | dind | non-root | root | root | daemon | pass | `s-mart-rootful-outer-rootless-inner-daemon-amd64.tar.gz` | — |
| M-DOCKER-ROOTFUL-ROOTLESS-DIND-AMD64-ROOT-NON-ROOT-NONROOT-DAEMON | dind | amd64 | docker | dind | non-root | non-root | debi | daemon | pass | `s-mart-rootful-outer-rootless-inner-nonroot-daemon-amd64.tar.gz` | — |
| M-DOCKER-ROOTFUL-PIND-ARM64-ROOT-ROOT-ROOT-LAYOUT | pind | arm64 | docker | pind | root | root | root | layout | blocked | — | sidecar 退出 |
| M-DOCKER-ROOTFUL-PIND-ARM64-ROOT-ROOT-ROOT-DAEMON | pind | arm64 | docker | pind | root | root | root | daemon | blocked | — | sidecar 退出 |
| M-DOCKER-ROOTFUL-DIND-ARM64-ROOT-ROOT-ROOT-LAYOUT | dind | arm64 | docker | dind | root | root | root | layout | pass | `s-mart-docker-outer-dind-layout-arm64.tar.gz` | — |
| M-DOCKER-ROOTFUL-DIND-ARM64-ROOT-ROOT-ROOT-DAEMON | dind | arm64 | docker | dind | root | root | root | daemon | fail | — | singleton |

