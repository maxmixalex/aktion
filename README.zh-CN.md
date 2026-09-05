# Aktion

**不用 Kubernetes 的自托管 GitHub Actions。**

Aktion 是一个面向 Docker host 的轻量 GitHub Runner Scale Set manager：每个仓库保留一个很轻的 listener，GitHub 分配任务时从 0 创建临时 Linux runner container，一次 job 完成后立即销毁。

> **目前只支持 Linux runner。** Aktion manager 本身不限定 Windows：Windows、Linux、macOS 上只要 Docker CLI 能连接到 Linux Docker Engine/Desktop 都可以运行。

## 主要能力

- scale-to-zero 临时 Linux runners
- GitHub App 认证
- 一个配置管理多个 private repo
- 每个 repo 单独覆盖 `minRunners` / `maxRunners`
- Rich 风格终端状态：repo → runner → job，并支持 OSC 8 可点击链接
- stale scale set 诊断与 `cleanup`
- HTTP/HTTPS proxy 透传到 listener 和真正执行 job 的 runner
- 可组合的 runner image features
- 不需要 Kubernetes / ARC cluster

Aktion 底层继续使用 GitHub 官方 [`actions/scaleset`](https://github.com/actions/scaleset) 实现 Scale Set 协议。

## Manager 与 listener

```text
aktion                Windows / Linux / macOS
  └─ 控制 Docker host
       └─ aktion-listener        Linux container
            └─ ephemeral runner  Linux container
```

因此发布物会包含跨平台的 `aktion` manager，以及预编译的 Linux amd64/arm64 `aktion-listener`。终端用户不需要在 Docker build 时现场跑 `go get/go build`。

## 使用

复制：

```text
config.example.json → config.json
```

把 GitHub App PEM 放到 `secrets/`，然后：

```text
aktion doctor
aktion up
aktion status
```

配置结构见 `config.example.json`。repo 中的 `minRunners/maxRunners` 会覆盖 defaults。

## Runner Features

Aktion 不要求所有仓库共享一个越来越大的 runner 镜像，而是提供类似 Dev Container Features 的可组合 runner feature。每个内置 feature 都由 metadata + 幂等 `install.sh` 组成，支持 `dependsOn` 硬依赖和 `installsAfter` 软排序。

默认 feature 集：

```json
"runnerFeatures": ["build", "network", "python", "archives", "native"]
```

它覆盖通用 C/C++/autotools/CMake/Ninja 构建工具、网络/源码工具、Python、压缩工具和常用 native headers。额外可选：`glib`、`gtk4`、`ncurses`、`lua54`。

仓库可以完整覆盖 defaults：

```json
{
  "id": "hpclang",
  "url": "https://github.com/zyf722/hpclang",
  "enabled": true,
  "runnerFeatures": [
    "build", "network", "python", "archives", "native",
    "glib", "gtk4", "ncurses", "lua54"
  ]
}
```

省略 `runnerFeatures` 表示继承；显式 `[]` 表示完全不加 Aktion feature，直接使用 base runner image。 `runnerImage` 可以理解为 feature 链的 base image。

Aktion 会解析依赖后，为每个 feature 生成一个独立 Docker stage/layer，并根据 base image + feature 内容生成稳定的 `local/aktion-runner:<hash>` tag。多个仓库采用同一组合时共用镜像和 Docker cache。 `aktion features` 可列出内置 feature。

目前内置 feature 使用 `apt` 安装，因此预期 base image 属于 Debian/Ubuntu 系。

## Proxy

如果 Windows/macOS/Linux Docker host 上有本地 HTTP proxy，例如 3081：

```json
"proxy": {
  "http": "http://host.docker.internal:3081",
  "https": "http://host.docker.internal:3081",
  "noProxy": "localhost,127.0.0.1"
}
```

Aktion 会增加 `host-gateway` 映射，因此普通 Linux Docker Engine 也可以使用同一个 `host.docker.internal` 名称。

## CLI

```text
aktion status
aktion repos
aktion runners
aktion stats
aktion doctor
aktion features
aktion logs [repo-id]
aktion scalesets
aktion cleanup
aktion up
aktion down
aktion restart
```

## 自举 CI

正常 CI 使用：

```yaml
runs-on: home-docker
```

也就是说 **Aktion builds Aktion**。同时保留一个仅手动触发的 `Bootstrap` workflow 使用 `ubuntu-latest`，用于 self-hosted runner 自己坏掉时恢复。
