# 故障排查

首先运行：

```text
aktion doctor
```

该命令会分别检查本地配置、Docker、GitHub App 认证和仓库 installation。以下内容按实际可观察的故障现象组织。

## `doctor` 报告 Docker Engine 不可用

**现象：** `aktion doctor` 或 `aktion status` 报告 Docker 不可用。

使用以下命令检查 Docker：

```text
docker version
docker compose version
```

任一命令失败时，启动 Docker Engine 或 Docker Desktop。Windows 和 macOS 上还应确认 Docker 能运行 Linux 容器；Aktion 的 listener 和 runner 当前不是 Windows 容器。

如果 `docker version` 成功，而 `docker compose version` 失败，请安装或启用 Docker Compose v2。

## `doctor` 报告私钥或 GitHub App 身份验证失败

**现象：** Local 或 GitHub 检查项报告 private key、JWT 或 App identity 失败。

确认 `app.clientId` 属于生成该 PEM 的同一个 GitHub App，并确认 `app.privateKeyFile` 指向存在且可读的文件。相对路径以 `--root` 指定的配置根目录为基准。

如果 GitHub App 私钥已经轮换，将 `privateKeyFile` 改为当前有效的 PEM。不要把私钥内容写入 JSON。

## installation 权限检查失败

**现象：** installation 可以访问，但 `doctor` 显示 `administration` 不是 `write`。

将 GitHub App 的仓库 **Administration** 权限设为 **Read and write**，并确认 installation 已接受更新后的权限。然后重新运行：

```text
aktion doctor
```

Aktion 不支持使用 PAT 绕过该权限要求。

## 仓库未安装 App 或 installation 不匹配

**现象：** Repositories 区域显示 App 未安装、仓库不可访问，或者实际 installation ID 与配置不一致。

检查 GitHub App installation，确认目标仓库已包含在安装范围内。如果不同仓库属于不同 installation，在对应的仓库配置中覆盖：

```json
"installationId": 87654321
```

然后重新运行 `aktion doctor`。

## listener 持续重启或出现 `RunnerScaleSetExistsException`

**现象：** `aktion status` 显示 listener 为 `RESTARTING` 或 `EXITED`，近期错误中包含 `RunnerScaleSetExistsException`。

先确认没有需要继续执行的作业：

```text
aktion runners
aktion scalesets
```

然后运行：

```text
aktion cleanup
```

检测到正在运行的临时 runner 时，cleanup 默认会拒绝继续。只有确认可以中断这些作业时，才使用 `aktion cleanup --force`。

## 工作流一直等待 runner

**现象：** GitHub Actions 作业长时间处于 queued 状态，或持续提示正在等待 runner。

依次检查：

```text
aktion status
aktion scalesets
aktion logs <repo-id>
```

listener 应为 `RUNNING`，远端 scale set 应为 `FOUND`，工作流的 `runs-on` 也必须与最终 runner 标签一致。未显式设置 `labels` 时，标签就是 `scaleSetName`，默认值为 `home-docker`。

如果 `status` 显示有等待中的作业，同时该仓库的 runner 数量已经达到 `maxRunners`，说明容量上限正在生效。提高该仓库的上限，或者等待现有作业结束：

```text
aktion config resolve <repo-id>
```

`maxRunners: 0` 表示不设 runner 数量上限，但不应以此掩盖主机容量不足的问题。

## runner image 或 Feature 构建失败

**现象：** `aktion up` 或 `restart` 在解析 runner Feature 或构建 runner image 时失败。

先检查最终配置：

```text
aktion config resolve <repo-id>
aktion features
```

确认 Feature URI 和 options 正确。公开 OCI Feature 可以使用匿名或 Bearer token registry 流程；当前不能配置私有 registry 凭据，因此需要私有认证的 registry 会失败。

如果 warning 提到 Dev Container lifecycle hook，这是当前的已知限制：Aktion 不执行 `postCreateCommand` 等 hook。项目初始化步骤应放在 GitHub Actions 工作流中。

## 修改配置后运行行为没有变化

**现象：** `config.json` 已修改，但 listener 仍使用原来的容量、Feature 或代理配置。

Aktion 不会热加载配置。运行：

```text
aktion config resolve <repo-id>
aktion restart
```

第一条命令用于确认新的最终配置，第二条命令会按新配置重新创建 listener。

不要依赖对 `compose.generated.yaml` 的手工修改；Aktion 会重新生成该文件。

## 无法通过代理访问 GitHub 或 registry

**现象：** 主机上的代理可以正常使用，但 listener 或 runner 报告 connection refused 或 timeout。

如果代理运行在 Docker 主机上，不要配置为：

```text
http://localhost:3081
```

应使用：

```text
http://host.docker.internal:3081
```

在 Linux Docker Engine 上，Aktion 会增加对应的 host-gateway 映射。修改配置后运行 `aktion restart`。

## runner 性能骤降或主机内存不足

**现象：** 多个作业并发执行后，CPU 长时间满载、内存接近耗尽，或者 Docker 响应明显变慢。

查看资源快照：

```text
aktion stats
```

然后降低相应仓库的 `maxRunners`。各仓库的限制彼此独立：三个仓库都设置为四个 runner 时，主机最多可能同时运行十二个 runner。

## 收集诊断信息

跟踪所有 listener：

```text
aktion logs
```

或只查看一个仓库：

```text
aktion logs <repo-id>
```

提交诊断信息时，优先提供失败的 `aktion doctor` 检查项、相关日志片段，以及 `aktion config resolve <repo-id>` 的输出。不要包含 GitHub App 私钥或未隐藏密码的代理 URL。
