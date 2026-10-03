# CLI 参考

## 语法

```text
aktion [--root DIR] [--config FILE] [--color MODE] [--links MODE] <command> [args]
```

未指定 command 时，Aktion 默认执行 `status`。

## 全局选项

| 选项 | 默认值 | 说明 |
| --- | --- | --- |
| `--root DIR` | 可执行文件所在目录 | 配置根目录（符号链接解析到目标文件）。显式指定的相对路径以工作目录为基准，`--root .` 使用当前工作目录。相对私钥路径和生成的 Compose 文件均以此目录为基准。 |
| `--config FILE` | `config.json` | 配置文件名。 |
| `--color auto\|always\|never` | `auto` | ANSI 颜色输出模式。 |
| `--links auto\|always\|never` | `auto` | OSC 8 终端链接输出模式。 |

`NO_COLOR=1` 会在 `auto` 模式下关闭颜色。`NO_HYPERLINK=1` 会关闭自动生成的终端链接。

## 命令

### `status`

```text
aktion status
```

显示 listener 健康状态、临时 runner、正在执行的作业、已分配但仍在等待的作业，以及各仓库的状态摘要。不指定 command 时也会执行该命令。

### `up`

```text
aktion up
```

解析配置，生成 `compose.generated.yaml`，准备基于 Feature 的 runner image，然后通过 Docker Compose 为所有已启用的仓库启动 listener。

### `down`

```text
aktion down
```

停止并删除 listener，不会删除配置、私钥或已经构建的 image。

### `restart`

```text
aktion restart
```

重新生成配置，先构建 runner 和 listener 镜像，再停止并重新启动 listener。构建失败时不会停止现有 listener。修改运行配置后使用该命令。

### `doctor`

```text
aktion doctor
```

检查配置、私钥、Docker Engine、Docker Compose、现有 listener 状态、GitHub App 身份、installation 的 Administration write 权限，以及仓库与 installation 是否匹配。

### `repos`

```text
aktion repos
```

显示已启用的仓库及其解析后的运行配置摘要。

### `runners`

```text
aktion runners
```

显示当前 Aktion runner 容器。能够从 listener 日志关联作业时，还会显示各 runner 正在执行的工作流作业。

### `stats`

```text
aktion stats
```

显示 listener 和 runner 容器当前的 Docker CPU、内存和网络 I/O 快照。

### `logs [repo-id]`

```text
aktion logs
aktion logs my-repo
```

持续跟踪所有 listener 的日志，或只跟踪指定仓库。开始跟踪前会先显示最近 200 行。

### `scalesets`

```text
aktion scalesets
```

查询 GitHub Actions 服务，显示每个已启用仓库的 scale set 是否存在，并列出其 ID 和 runner group。

### `cleanup`

```text
aktion cleanup [--force|-f] [--no-start]
```

停止本地 listener，删除已配置仓库的远端 scale set；清理全部成功后，默认重新启动 listener。

| 选项 | 作用 |
| --- | --- |
| `--force`、`-f` | 即使检测到正在运行的临时 runner 也继续清理。 |
| `--no-start` | 清理完成后保持 listener 停止。 |

!!! danger
    `--force` 可能中断正在执行的工作流作业。正常情况下应等待 runner 退出后再执行 cleanup。

### `features`

```text
aktion features
```

列出 Aktion 内置 Feature、默认 OCI Dev Container Feature，以及内置 Feature 的依赖关系。该命令不需要读取 `config.json`。

### `config`

```text
aktion config
```

生成 `compose.generated.yaml`，检查 Docker 是否可用，然后运行 Docker Compose 配置验证。

### `config resolve <repo-id>`

```text
aktion config resolve my-repo
```

以 JSON 输出已启用仓库最终解析出的 `minRunners`、`maxRunners`、`baseImage`、本地 `image` 和 `features`。排查继承或 Feature 配置时，可用该命令确认最终结果。

### `render`

```text
aktion render
```

生成 `compose.generated.yaml`，但不运行 Docker Compose 配置验证。

### `version`

```text
aktion version
```

显示 Aktion 版本以及主程序运行的 OS 和 architecture。

### `help`

```text
aktion help
```

显示内置命令帮助。
