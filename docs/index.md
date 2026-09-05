# Aktion

**不用 Kubernetes 的自托管 GitHub Actions。**

Aktion 在一个普通 Docker host 上运行 GitHub Runner Scale Set。每个已配置仓库保留一个轻量 listener；GitHub 分配 job 后，listener 创建临时 Linux runner container，job 完成后立即销毁。

## 适合什么场景

- 希望把一台 Windows、Linux 或 macOS 主机作为 GitHub Actions 自托管执行节点。
- Docker daemon 可以运行 Linux containers。
- 不希望为了弹性 runner 部署 Kubernetes / ARC cluster。
- 希望 runner 环境由 host 管理，而不是由目标仓库决定。

## 运行模型

```text
aktion.exe
  └─ Docker
      └─ listener（每仓库一个）
          └─ ephemeral runner（每个 job）
```

Aktion manager 可以运行在 Windows、Linux 或 macOS；当前 runner 和 listener 只支持 Linux。

## 下一步

从[安装与启动](getting-started.md)开始。首次配置前请先创建 GitHub App，并准备好它的 private key。
