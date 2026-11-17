# PaaS3.0 S-Mart 应用构建工具
本项目基于 go 1.20 开发, 支持在本地构建 SaaS

> 本项目仅支持构建基于`应用描述文件(app_desc.yaml)`的应用

# 本地开发方法
## 1. 准备依赖环境
- [安装 Docker](https://docs.docker.com/engine/install/)

## 2. 构建程序和基础镜像
```bash
make build-cli # x86 架构
make build-cli-arm64 # arm64 架构
```

## 3. 构建 S-Mart 应用

`bksmartpacker` 将使用 `docker` 构建应用(支持多模块), 并输出构建产物(镜像层)

```bash
❯ ./bin/bksmartpacker-linux -h
      --builder-image string     cnb builder 镜像 (default "mirrors.tencent.com/bkpaas/bk-builder-heroku-bionic:v1.0.0")
      --dest string              s-mart 包的输出路径, 支持相对路径(相对于工作目录)或绝对路径; 
                                  (s-mart 包是 gzip 压缩的 tarball, 建议以 .tar.gz 或 .tgz 作为命名后缀) (default "./s-mart.tar.gz")
      --disable-collect-static   (Python)禁用静态文件收集 (default true)
  -h, --help                     Show help
      --src string               应用源码的路径, 支持相对路径(相对于工作目录)或绝对路径 
                                  应用源码目录需要包含应用描述文件(app_desc.yaml)
      --cache-registry string    缓存镜像仓库. 格式如: mirrors.tencent.com/bkapps
      --registry-auth string     镜像仓库凭证信息. 格式如: '{"mirrors.tencent.com": "Basic xxx"}'
      --buildpack-cfg string     buildpack 配置文件路径, 格式如: conf/buildpack.yaml
```

其中, **--src** 需要是`应用描述文件(app_desc.yaml)`所在的目录。
