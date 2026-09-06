# 快速开始

本指南从一台尚未运行 Aktion 的 Docker 主机开始，最终让 GitHub Actions 作业在 Aktion runner 上执行。完成后，`aktion status` 应显示健康的 listener，GitHub Actions 也应能将作业分配给 `home-docker`。

## 准备运行目录

主机需要 Docker Engine 或 Docker Desktop，并且必须支持 **Linux 容器**；同时需要 Docker Compose v2，即可使用 `docker compose` 命令。Aktion 主程序支持 Windows、Linux 和 macOS，listener 与 runner 当前为 Linux 容器。

可运行的 Aktion 目录至少包含：

```text
aktion.exe                 # Windows; usually named aktion on Linux/macOS
config.json
Dockerfile
packaging/
  start-listener.sh
dist/
  aktion-listener-linux-amd64   # or linux-arm64, matching the Docker host
```

GitHub App 的 PEM 不应包含在分发包中，也不应提交到版本控制。以下示例将其保存为 `secrets/github-app.pem`。

!!! warning "保护私钥"
    `privateKeyFile` 指向的 PEM 可用于以该 GitHub App 的身份认证。不要将它提交到仓库，也不要共享包含该文件的运行目录。

先检查 Docker：

=== "Windows"

    ```powershell
    docker version
    docker compose version
    ```

=== "Linux / macOS"

    ```bash
    docker version
    docker compose version
    ```

两条命令都应成功。如果 Docker Desktop 当前使用 Windows 容器，请先切换到 Linux 容器。

## 创建 GitHub App

Aktion 仅支持 GitHub App 认证，不支持 PAT。

1. 创建 GitHub App，将仓库的 **Administration** 权限设为 **Read and write**。
2. Aktion 不需要 webhook。若该 App 仅用于 Aktion，可关闭 webhook。
3. 将 App 安装到需要使用这台 runner 主机的仓库。
4. 记录 App 的 **Client ID** 和数字形式的 **Installation ID**。
5. 生成私钥，并将 PEM 文件保存在 Aktion 主机上。

属于同一个 App installation 的仓库可以共用 `app.installationId`。如果某个仓库属于其他 installation，可在该仓库的配置项中覆盖此值。

## 创建最小配置

创建 `config.json`：

```json
{
  "app": {
    "clientId": "Iv23liXXXXXXXXXXXXXXXXXX",
    "installationId": 12345678,
    "privateKeyFile": "./secrets/github-app.pem"
  },
  "defaults": {
    "scaleSetName": "home-docker",
    "minRunners": 0,
    "maxRunners": 1,
    "logLevel": "info"
  },
  "repositories": [
    {
      "id": "my-repo",
      "url": "https://github.com/OWNER/REPO",
      "enabled": true
    }
  ]
}
```

将 PEM 保存到 `./secrets/github-app.pem`。相对路径以 Aktion 的配置根目录为基准；默认配置根目录是当前工作目录。

`minRunners: 0` 表示没有作业时该仓库可以缩容到零个 runner。`maxRunners: 1` 表示该仓库同时最多运行一个 runner。此上限按仓库计算，不是整台主机的总上限。

## 启动前验证

=== "Windows"

    ```powershell
    .\aktion.exe doctor
    ```

=== "Linux / macOS"

    ```bash
    ./aktion doctor
    ```

`doctor` 会检查 JSON 配置、私钥、Docker Engine、Docker Compose、GitHub App 身份、installation 权限，以及 App 是否实际安装到了每个已启用的仓库。

如有失败项，先按提示修复。全部检查通过时，末尾会显示 **All systems operational**。

## 启动 listener

=== "Windows"

    ```powershell
    .\aktion.exe up
    .\aktion.exe status
    ```

=== "Linux / macOS"

    ```bash
    ./aktion up
    ./aktion status
    ```

首次运行 `up` 时，Aktion 可能需要拉取基础 image、解析 Dev Container Feature，并构建本地 runner image。随后会为每个已启用的仓库启动一个 listener 容器。

`status` 中应显示该仓库的 listener 为 `RUNNING`。此时尚未分配作业，因此 runner 数量为 0 属于正常状态。

## 运行第一个工作流

默认 scale set 名称为 `home-docker`。在已配置的仓库中创建或修改工作流：

```yaml
name: Aktion smoke test

on:
  workflow_dispatch:

jobs:
  test:
    runs-on: home-docker
    steps:
      - uses: actions/checkout@v7
      - run: uname -a
```

手动触发该工作流，然后运行：

```text
aktion status
```

作业开始后，应能短暂看到一个临时 runner 和一个正在运行的作业。作业结束后，该 runner 容器会被删除。如果作业持续等待 runner，请参阅[工作流一直等待 runner](troubleshooting.md)。

## 后续配置

第一个仓库运行正常后，可继续配置：

- [添加更多仓库](guides/repositories.md)
- [为不同仓库设置不同的并发上限](guides/capacity.md)
- [为 runner 添加 Node、Go、Rust 等 Feature](guides/runner-environment.md)
- [查看状态、日志以及重启和清理命令](operations.md)
