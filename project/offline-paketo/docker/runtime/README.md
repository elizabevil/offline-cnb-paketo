# 通用 runtime 镜像

runtime-pind / runtime-dind 由两个项目共用，不再放在 BK 项目内。

## 文件

```text
runtime-pind.dockerfile
runtime-pind-entrypoint.sh
runtime-dind.dockerfile
runtime-dind-entrypoint.sh
```

## 构建上下文

外部构建必须提供：

```text
--build-context bundle=<repo>/bundle
--build-context runtime=<repo>/project/offline-paketo/docker/runtime
```

镜像内不包含项目专属逻辑，只启动：

```text
pind  → podman system service
dind  → dockerd
```

## 消费

```text
bundle/<platform>/images/runtime-pind.tar
bundle/<platform>/images/runtime-dind.tar
```
