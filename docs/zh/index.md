# Aktion

**不用 Kubernetes 的自托管 GitHub Actions runner。**

Aktion 让一台普通的 Docker 主机同时为多个 GitHub repository 提供按需 runner。每个已启用的 repository 都有自己的 GitHub Runner Scale Set listener；有 job 到达时，Aktion 创建临时 Linux runner container，job 完成后销毁。你得到的是 GitHub Actions 原生的队列和 workflow 体验，而不需要为了弹性 runner 部署 Kubernetes 或 Actions Runner Controller。

Aktion 适合希望自己掌控 runner 主机和软件环境、同时又不想长期维护一批常驻 runner 的个人或小型团队。Manager 可以运行在 Windows、Linux 和 macOS；listener 与实际执行 job 的 runner 当前使用 Linux container。

!!! note "它不是 GitHub-hosted Ubuntu 的复制品"
    默认 runner 基于 `ghcr.io/actions/actions-runner:latest`，再叠加 Aktion 的默认 Features。它不会自动拥有 GitHub-hosted `ubuntu-24.04` 镜像中的全部预装工具。项目特有的依赖仍应在 workflow 中安装，或明确加入你控制的 runner 环境。

## 从这里开始

第一次使用时，按下面的顺序完成即可：

1. 准备能运行 Linux container 的 Docker Engine 或 Docker Desktop，以及 Docker Compose v2。
2. 创建并安装一个 GitHub App，给目标 repository **Administration: Read and write** 权限。
3. 在 `config.json` 中填入 App 凭据和要接入的 repository。
4. 运行 `aktion doctor`，先把配置、Docker 和 GitHub 连接全部检查通过。
5. 运行 `aktion up`，然后把 workflow 的 `runs-on` 指向你的 scale set 名称。

[开始第一次部署 →](getting-started.md)

## 你通常会做什么

如果 Aktion 已经运行起来，最常见的工作集中在四件事上：增加 repository、控制每个 repository 的并发、调整 runner 环境，以及查看运行状态。对应的指南不会要求你理解内部 Go package 或 listener 实现。

- [添加或停用 repository](guides/repositories.md)
- [设置 runner 数量与并发](guides/capacity.md)
- [定制 runner 环境](guides/runner-environment.md)
- [通过 HTTP/HTTPS proxy 访问外部服务](guides/proxy.md)

日常查看和维护命令集中在[运行与维护](operations.md)，所有 `config.json` 字段和 CLI 命令则分别放在[配置参考](reference/configuration.md)与[CLI 参考](reference/cli.md)。

## Aktion 负责什么

| Aktion 负责 | 仍由你负责 |
| --- | --- |
| 为每个 repository 维护 scale set listener | Docker 主机本身的可用性和容量 |
| 根据 GitHub 分配的 job 创建、销毁 runner | workflow 中项目专用的依赖安装 |
| 构建 host 控制的 runner image | GitHub App 的创建、安装和 private key 保管 |
| 限制每个 repository 的 runner 数量 | 决定哪些 repository 可以使用这台主机 |
| 展示 listener、runner、job 和 Docker 状态 | 对不受信任 workflow 采用合适的隔离策略 |

想先理解 listener、runner 和 repository 之间的关系，可以阅读[Aktion 如何工作](concepts/how-it-works.md)。