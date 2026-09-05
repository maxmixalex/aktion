# CLI 参考

## 语法

```text
aktion [--root DIR] [--config FILE] [--color MODE] [--links MODE] <command> [args]
```

不提供 command 时默认执行 `status`。

## 全局选项

| 选项 | 默认值 | 说明 |
| --- | --- | --- |
| `--root DIR` | 当前工作目录 | 配置根目录。相对 private-key 路径、生成的 Compose 文件等都以此目录为基准。 |
| `--config FILE` | `config.json` | 配置文件名。 |
| `--color auto\|always\|never` | `auto` | ANSI 颜色模式。 |
| `--links auto\|always\|never` | `auto` | OSC 8 终端链接模式。 |

`NO_COLOR=1` 会在 `auto` 模式关闭颜色；`NO_HYPERLINK=1` 会关闭自动终端链接。

## 命令

### `status`

```text
aktion status
```

显示 listener 健康状态、ephemeral runner、active job、已分配但等待中的 job，以及 repository 汇总。无参数运行 `aktion` 时执行同一命令。

### `up`

```text
aktion up
```

解析配置，生成 `compose.generated.yaml`，准备需要的 runner Feature image，然后用 Docker Compose 启动所有已启用 repository 的 listener。

### `down`

```text
aktion down
```

停止并移除 listener stack。不会删除配置、private key 或已构建 image。

### `restart`

```text
aktion restart
```

重新生成配置，停止当前 listener，重新准备 runner image，再启动 listener。修改运行配置后使用。

### `doctor`

```text
aktion doctor
```

检查配置、private key、Docker Engine、Docker Compose、现有 listener 状态、GitHub App identity、installation 的 Administration write 权限，以及 App 与 repository 的 installation 对应关系。

### `repos`

```text
aktion repos
```

显示当前已启用的 repository 及其解析后的运行配置摘要。

### `runners`

```text
aktion runners
```

显示当前正在运行的 Aktion runner container；能够从 listener 日志关联时也显示其 workflow job。

### `stats`

```text
aktion stats
```

显示 listener 和 runner container 的一次性 Docker CPU、内存和网络 I/O 快照。

### `logs [repo-id]`

```text
aktion logs
aktion logs my-repo
```

持续跟踪所有 listener，或只跟踪指定 repository。初始输出包含最近 200 行。

### `scalesets`

```text
aktion scalesets
```

直接查询 GitHub Actions 服务，显示每个已启用 repository 的 scale set 是否存在以及其 ID / runner group。

### `cleanup`

```text
aktion cleanup [--force|-f] [--no-start]
```

停止本地 listener，删除已配置 repository 的远端 scale set，并在成功后默认重新启动 listener。

| 选项 | 作用 |
| --- | --- |
| `--force`, `-f` | 即使检测到活跃 ephemeral runner 也继续。 |
| `--no-start` | 清理完成后让 listener 保持停止。 |

!!! danger
    `--force` 可能直接中断正在执行的 workflow job。正常情况下先等待 runner 退出。

### `features`

```text
aktion features
```

列出当前 Aktion builtin、默认 OCI Dev Container Features 以及 builtin 依赖关系。不需要加载 `config.json`。

### `config`

```text
aktion config
```

生成 `compose.generated.yaml`，确认 Docker 可用，再运行 Docker Compose 的配置验证。

### `config resolve <repo-id>`

```text
aktion config resolve my-repo
```

以 JSON 输出一个已启用 repository 最终解析出的 `minRunners`、`maxRunners`、`baseImage`、本地 `image` 和 `features`。排查继承或 Feature 问题时优先使用。

### `render`

```text
aktion render
```

只生成 `compose.generated.yaml`，不执行 Docker Compose 配置验证。

### `version`

```text
aktion version
```

显示 Aktion 版本以及 manager 的 OS/architecture。

### `help`

```text
aktion help
```

显示内置命令帮助。
