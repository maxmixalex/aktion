# 运行与维护

日常使用 Aktion 不需要直接操作生成的 Compose 文件。配置放在 `config.json`，启动、状态、日志和修复都通过 `aktion` 命令完成。

## 启动、停止和重启

```text
aktion up
aktion down
aktion restart
```

`up` 会重新解析配置和 runner Features，按需要构建本地 runner image，然后启动所有已启用 repository 的 listener。

`down` 停止并移除 listener stack。它不会删除你的 `config.json`、private key 或已构建的 Docker image。

`restart` 等价于重新生成配置后先停止、再启动 listener。新增 repository、调整容量、Feature、proxy 或其他运行配置后，用它让修改生效。

## 查看整体状态

```text
aktion status
```

无参数运行 `aktion` 也会进入 `status`。页面会显示：

- 健康 listener 数量；
- 当前 ephemeral runner 数量；
- 正在运行的 job；
- GitHub 已分配但还在等待 runner 的 job；
- 每个 repository 的 listener 状态和 runner 使用情况。

如果 listener 不是 `RUNNING`，状态页会尽量给出最近的可用错误提示。需要完整上下文时再看日志。

## 查看 repository、runner 和资源

```text
aktion repos
aktion runners
aktion stats
```

`repos` 适合快速确认当前实际启用了哪些 repository 以及它们解析出的容量。被设为 `enabled: false` 的项不会出现在运行配置中。

`runners` 只显示当前可见的 ephemeral runner container，并在能够关联时显示正在执行的 workflow job。

`stats` 是一次性 Docker 资源快照，包含 Aktion listener 和 runner 的 CPU、内存与网络 I/O。它不是长期监控系统，但足够用来判断是否应该降低 `maxRunners`。

## 跟踪日志

查看所有 listener：

```text
aktion logs
```

只看一个 repository：

```text
aktion logs my-repo
```

日志命令会跟踪输出，并先显示最近 200 行。按终端常用方式（通常是 Ctrl+C）结束跟踪不会停止 listener。

## 检查 GitHub 侧 scale set

```text
aktion scalesets
```

这个命令直接检查每个已启用 repository 在 GitHub Actions 服务中的 scale set。正常运行时应看到 `FOUND`。

如果本地 listener 已退出，而 GitHub 侧仍残留同名 scale set，下一次启动可能遇到 `RunnerScaleSetExistsException`。这时使用下一节的 cleanup。

## 清理 stale scale set

先确认当前没有重要 job 在运行，然后：

```text
aktion cleanup
```

Aktion 会先停止本地 listener，再删除已配置 repository 的远端 scale set，最后重新启动 listener。

为避免意外中断，检测到正在运行的 ephemeral runner 时，默认会拒绝 cleanup。

!!! danger "cleanup --force 会中断活跃工作"
    只有确定这些 job 可以被打断时才运行：

    ```text
    aktion cleanup --force
    ```

如果只想清掉远端状态而暂时不重新启动：

```text
aktion cleanup --no-start
```

如果远端清理过程中有 repository 失败，Aktion 会让本地 listener 保持停止，以避免不断重启并重复报错。先修复显示的 GitHub App、权限或网络问题，再重新执行 cleanup 或 `aktion up`。

## 在改配置前查看最终结果

继承、preset 和 Feature 较多时，先查看一个 repository 的最终环境：

```text
aktion config resolve my-repo
```

要验证生成的 Docker Compose 是否有效：

```text
aktion config
```

只想生成 `compose.generated.yaml` 而不调用 Docker Compose 校验：

```text
aktion render
```

`compose.generated.yaml` 是 Aktion 生成物。不要把它当成长期手工维护的配置来源；下一次 render、up 或 restart 都可能重写它。
