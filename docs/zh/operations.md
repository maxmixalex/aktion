# 运行和维护 Aktion

日常管理 Aktion 不需要编辑生成的 Compose 文件。期望的运行配置保存在 `config.json` 中；启动、状态检查、日志和故障恢复均通过 `aktion` 命令完成。

## 启动、停止和重启

```text
aktion up
aktion down
aktion restart
```

`up` 会解析配置和 runner Feature，按需构建本地 runner image，然后为所有已启用的仓库启动 listener。

`down` 会停止并删除 listener，不会删除 `config.json`、私钥或已经构建的 Docker image。

`restart` 会重新生成配置，先构建 runner 和 listener 镜像；任一构建失败时保留现有 listener 运行。全部构建成功后，才停止当前 listener 并使用已构建镜像重新启动。修改仓库、容量、Feature、代理或其他运行配置后，应执行该命令。

## 查看整体状态

```text
aktion status
```

不带命令运行 `aktion` 时也会执行 `status`。状态页包含：

- 健康的 listener 数量；
- 当前临时 runner 数量；
- 正在执行的作业；
- GitHub 已分配但仍在等待 runner 的作业；
- 各仓库的 listener 和 runner 状态。

如果 listener 不是 `RUNNING`，状态页会尽量显示近期相关错误。需要完整上下文时，再查看日志。

## 查看仓库、runner 和资源使用情况

```text
aktion repos
aktion runners
aktion stats
```

`repos` 用于确认哪些仓库实际处于启用状态，以及它们解析后的容量。`enabled: false` 的配置项不会参与运行配置。

`runners` 会列出当前可见的临时 runner 容器。如果 listener 日志中包含足够信息，还会显示各 runner 正在执行的工作流作业。

`stats` 显示 Aktion listener 和 runner 的一次性 Docker 资源快照，包括 CPU、内存和网络 I/O。它不是持续监控工具，但可用于判断 `maxRunners` 是否设置过高。

## 跟踪日志

查看所有 listener：

```text
aktion logs
```

只查看一个仓库：

```text
aktion logs my-repo
```

命令会先显示最近 200 行日志，然后持续跟踪输出。按终端通常的方式退出日志跟踪（一般为 Ctrl+C）不会停止 listener。

## 查看 GitHub 端的 scale set

```text
aktion scalesets
```

该命令会直接查询 GitHub Actions 服务，检查每个已启用仓库的 scale set。正常运行的仓库通常应显示 `FOUND`。

如果 listener 退出时未能删除远端 scale set，下次启动可能出现 `RunnerScaleSetExistsException`。此时应执行 cleanup，而不是修改生成的状态文件。

## 清理遗留的 scale set { #cleanup-stale-scale-set }

先确认没有需要继续执行的作业：

```text
aktion cleanup
```

Aktion 会先停止本地 listener，删除已配置仓库的远端 scale set；全部清理成功后，再重新启动 listener。

默认情况下，只要检测到正在运行的临时 runner，cleanup 就会拒绝继续。

!!! danger "cleanup --force 可能中断正在执行的作业"
    只有确认这些作业可以被终止时，才执行：

    ```text
    aktion cleanup --force
    ```

如果需要删除远端状态，但暂时保持 listener 停止：

```text
aktion cleanup --no-start
```

如果一个或多个仓库清理失败，Aktion 会保持 listener 停止，避免反复重启。修复命令显示的 GitHub App、权限或网络问题后，再重新执行 cleanup 或运行 `aktion up`。

## 检查最终配置

配置包含继承、preset 或 Feature 时，可先查看单个仓库解析后的配置：

```text
aktion config resolve my-repo
```

生成 Compose 文件并由 Docker Compose 验证：

```text
aktion config
```

仅生成 `compose.generated.yaml`：

```text
aktion render
```

`compose.generated.yaml` 是 Aktion 生成的状态文件。不要将手工修改视为持久配置；`render`、`up` 或 `restart` 都可能覆盖该文件。
