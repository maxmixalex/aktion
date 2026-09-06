# Aktion

**Aktion 是一个面向 Docker 主机的轻量级 GitHub Actions runner scale-set 管理器。**

Aktion 可让一台普通的 Docker 主机按需为多个 GitHub 仓库提供 runner。每个启用的仓库都有独立的 GitHub Runner Scale Set listener。GitHub 分配作业后，Aktion 会创建临时 Linux runner 容器，并在作业结束后将其删除。这样既保留 GitHub Actions 原有的工作流和排队机制，也无需仅为弹性 runner 部署 Kubernetes 或 Actions Runner Controller。

Aktion 适合希望自行管理 runner 主机和软件环境，又不需要长期维护一组常驻 runner 的个人和小型团队。Aktion 主程序可运行在 Windows、Linux 和 macOS 上；listener 与执行作业的 runner 当前运行在 Linux 容器中。

!!! note "默认环境不同于 GitHub-hosted Ubuntu"
    默认 runner 以 `ghcr.io/actions/actions-runner:latest` 为基础，并加入 Aktion 的默认 Feature。它不会自动包含 GitHub-hosted `ubuntu-24.04` 预装的全部工具。项目专用依赖应由工作流安装，或者明确加入由主机管理的 runner 环境。

## CLI 演示

下面的 [asciinema](https://asciinema.org/) 录屏由当前 Aktion 源码实际构建后生成，展示 `aktion features` 和 `aktion help`。播放不需要 GitHub App、Docker daemon 或其他运行配置。

<div id="aktion-cli-demo" data-cast="assets/aktion-demo.cast"></div>

## 开始使用

首次部署按以下顺序完成：

1. 准备支持 Linux 容器的 Docker Engine 或 Docker Desktop，并安装 Docker Compose v2。
2. 创建 GitHub App，将目标仓库的 **Administration** 权限设为 **Read and write**，然后安装该 App。
3. 在 `config.json` 中填写 App 凭据和仓库信息。
4. 运行 `aktion doctor`。启动 listener 前，先修复配置、Docker 或 GitHub 连接问题。
5. 运行 `aktion up`，再将工作流的 `runs-on` 设置为已配置的 scale set 名称。

[运行第一个作业 →](getting-started.md)

## 常见任务

Aktion 启动后，常见管理工作主要包括添加仓库、设置各仓库的并发上限、调整 runner 环境，以及检查运行状态。

- [添加或停用仓库](guides/repositories.md)
- [配置 runner 数量和并发](guides/capacity.md)
- [配置 runner 环境](guides/runner-environment.md)
- [配置 HTTP/HTTPS 代理](guides/proxy.md)

日常命令见[运行和维护](operations.md)。`config.json` 的所有字段和 CLI 命令分别见[配置参考](reference/configuration.md)和 [CLI 参考](reference/cli.md)。

## 职责范围

| Aktion 负责 | 主机管理员负责 |
| --- | --- |
| 为每个仓库维护一个 scale set listener | 保证 Docker 主机可用，并规划主机容量 |
| 根据 GitHub 分配的作业创建和删除 runner | 在工作流中安装项目专用依赖 |
| 构建由主机配置的 runner image | 创建、安装并妥善保管 GitHub App 私钥 |
| 限制每个仓库的 runner 数量 | 决定哪些仓库可以使用这台主机 |
| 显示 listener、runner、作业和 Docker 状态 | 为不受信任的工作流选择适当的隔离方式 |

要了解仓库、listener 和 runner 之间的关系，请参阅 [Aktion 的工作方式](concepts/how-it-works.md)。
