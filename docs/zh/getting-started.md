# 快速开始

这一页从一个尚未运行 Aktion 的 Docker 主机开始，目标是让一个 GitHub Actions workflow 真正跑到你的自托管 runner 上。完成后，你应该能在 `aktion status` 中看到健康的 listener，并在 GitHub Actions 中看到 job 被 `home-docker` 接走。

## 准备运行目录

主机需要 Docker Engine 或 Docker Desktop，并能运行 **Linux containers**；同时需要 Docker Compose v2，即 `docker compose` 命令可用。Manager 支持 Windows、Linux 和 macOS，但 listener 与 runner 当前是 Linux container。

一个可运行的 Aktion 目录至少需要：

```text
aktion.exe                 # Windows；Linux/macOS 通常命名为 aktion
config.json
Dockerfile
packaging/
  start-listener.sh
dist/
  aktion-listener-linux-amd64   # 或 linux-arm64，与 Docker host 架构匹配
```

GitHub App 的 PEM 不应包含在分发包或版本控制中。后面会把它放到例如 `secrets/github-app.pem`。

!!! warning "保护 private key"
    `privateKeyFile` 指向的 PEM 可以代表 GitHub App 进行认证。不要提交到 repository，也不要把包含 private key 的运行目录公开分享。

先确认 Docker：

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

两条命令都应成功。如果 Docker Desktop 正在使用 Windows containers，请先切换到 Linux containers。

## 创建 GitHub App

Aktion 只使用 GitHub App 认证，不提供 PAT 模式。为 Aktion 创建一个 GitHub App，并完成以下设置：

1. 在 repository permissions 中把 **Administration** 设为 **Read and write**。
2. Aktion 不需要 webhook；如果这个 App 只用于 Aktion，可以关闭 webhook。
3. 把 App 安装到你希望由这台主机执行 Actions 的 repository。
4. 记录 **Client ID** 和安装对应的数字 **Installation ID**。
5. 为 App 生成 private key，并保存为 PEM 文件。

如果多个 repository 属于同一次 App installation，可以共用 `app.installationId`。需要不同 installation 的 repository 可以在自己的配置项中覆盖它。

## 写入最小配置

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

把 PEM 放到 `./secrets/github-app.pem`。相对路径以 Aktion 的配置根目录为基准；默认就是你运行 Aktion 时的当前目录。

`minRunners: 0` 允许没有 job 时缩到零；`maxRunners: 1` 表示这个 repository 同时最多拥有一个 runner。这里的 `maxRunners` 是**每个 repository 的限制**，不是整台主机的总上限。

## 在启动前检查

=== "Windows"

    ```powershell
    .\aktion.exe doctor
    ```

=== "Linux / macOS"

    ```bash
    ./aktion doctor
    ```

`doctor` 会检查配置 JSON、private key、Docker Engine、Docker Compose、GitHub App 身份、installation 权限，以及 App 是否真的安装到了每个已启用的 repository。

先把所有失败项修好。成功时最后会看到 **All systems operational**。

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

第一次 `up` 可能需要拉取基础 image、解析 Dev Container Features 并构建本地 runner image。之后 Aktion 会启动每个 repository 的 listener container。

在 `status` 中，对刚才的 repository 应看到 listener 为 `RUNNING`。此时还没有 job，所以 runner 数量为 0 是正常的。

## 让 workflow 使用 Aktion

默认 scale set 名称是 `home-docker`。在刚刚配置的 repository 中创建或修改 workflow：

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

手动触发这个 workflow。随后再次运行：

```text
aktion status
```

你应该短暂看到一个 ephemeral runner 和一个 active job；job 完成后，该 runner container 会被销毁。如果 workflow 一直显示等待 runner，转到[Workflow 一直等待 runner](troubleshooting.md)。

## 下一步

一个 repository 跑通以后，不需要重新理解系统就可以继续扩展：

- [添加更多 repository](guides/repositories.md)
- [为不同 repository 设置不同并发](guides/capacity.md)
- [增加 Node、Go、Rust 等 runner Feature](guides/runner-environment.md)
- [了解日常状态、日志和重启命令](operations.md)
