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

如果 Windows/macOS/Linux Docker host 上有本地 HTTP proxy，例如 3081：

```json
"proxy": {
  "http": "http://host.docker.internal:3081",
  "https": "http://host.docker.internal:3081",
  "noProxy": "localhost,127.0.0.1"
}
```

Aktion 会增加 `host-gateway` 映射，因此普通 Linux Docker Engine 也可以使用同一个 `host.docker.internal` 名称。

## 自举 CI

正常 CI 使用：

```yaml
runs-on: home-docker
```

也就是说 **Aktion builds Aktion**。同时保留一个仅手动触发的 `Bootstrap` workflow 使用 `ubuntu-latest`，用于 self-hosted runner 自己坏掉时恢复。
