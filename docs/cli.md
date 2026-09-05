# CLI

```text
aktion [--root DIR] [--config FILE] [--color MODE] [--links MODE] <command>
```

| 命令 | 用途 |
| --- | --- |
| `status` | 总览 listener、runner 与 job；也是无参数时的默认命令 |
| `up` | 构建 runner 环境并启动 listener |
| `down` | 停止 listener stack |
| `restart` | 重建并重启 listener stack |
| `doctor` | 检查配置、Docker 和 GitHub App |
| `repos` | 查看已启用 repository |
| `runners` | 查看当前 ephemeral runner |
| `stats` | 查看相关 Docker container 的一次性资源快照 |
| `logs [repo-id]` | 跟踪 listener 日志 |
| `scalesets` | 查看 GitHub 侧 scale set |
| `cleanup` | 删除 stale scale set 并重启 listener |
| `features` | 查看 builtin 和默认 OCI Features |
| `config` | 让 Docker Compose 校验生成配置 |
| `config resolve <repo-id>` | 查看 repository 的最终 runner 环境 |
| `render` | 生成 `compose.generated.yaml` |
| `version` | 显示版本 |
| `help` | 显示帮助 |

## 全局选项

`--root DIR`
: 配置目录。默认当前工作目录。

`--config FILE`
: 配置文件名。默认 `config.json`。

`--color auto|always|never`
: 控制 ANSI 色彩。

`--links auto|always|never`
: 控制支持终端中的 OSC 8 链接。

`NO_COLOR=1` 可在 auto 模式关闭颜色；`NO_HYPERLINK=1` 可关闭链接。
