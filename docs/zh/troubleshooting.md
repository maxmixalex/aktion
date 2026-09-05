# 故障排查

遇到问题时先运行：

```text
aktion doctor
```

它会把本地配置、Docker、GitHub App 和 repository installation 的问题分组显示。下面的条目从你能观察到的症状出发。

## `doctor` 报 Docker Engine 不可用

**症状：** `aktion doctor` 或 `aktion status` 提示 Docker unavailable。

先直接验证：

```text
docker version
docker compose version
```

如果失败，启动 Docker Engine / Docker Desktop。Windows 和 macOS 上还要确认 Docker 能运行 Linux containers；Aktion 的 listener 和 runner 当前不是 Windows container。

如果 `docker version` 成功而 `docker compose version` 失败，需要安装或启用 Docker Compose v2。

## `doctor` 报 private key 或 GitHub App identity 失败

**症状：** Local / GitHub 检查中 private key、JWT 或 App identity 为 FAIL。

检查 `app.clientId` 是否来自生成该 PEM 的同一个 GitHub App，并确认 `app.privateKeyFile` 路径存在、可读。相对路径以 `--root` 指定的配置目录为基准。

如果你换过 App private key，只需要让 `privateKeyFile` 指向当前有效 PEM；不要把 private key 内容直接写进 JSON。

## Installation 权限检查失败

**症状：** `doctor` 显示 installation 可访问，但 `administration` 不是 `write`。

在 GitHub App 的 repository permissions 中把 **Administration** 设置为 **Read and write**，然后确认安装已接受更新后的权限。再次运行：

```text
aktion doctor
```

Aktion 不支持用 PAT 绕过这个要求。

## Repository 显示 App 未安装或 installation 不匹配

**症状：** `doctor` 的 Repositories 区域显示 App is not installed、repo inaccessible，或者实际 installation ID 与配置不同。

先在 GitHub App 的 installation 页面确认目标 repository 已被选中。如果多个 repository 分属于不同 installation，在对应 repository 配置中覆盖：

```json
"installationId": 87654321
```

然后再次运行 `aktion doctor`。

## Listener 不断重启或显示 `RunnerScaleSetExistsException`

**症状：** `aktion status` 中 listener 为 `RESTARTING` / `EXITED`，最近错误包含 `RunnerScaleSetExistsException`。

先确认没有重要 job 在执行：

```text
aktion runners
aktion scalesets
```

然后：

```text
aktion cleanup
```

Aktion 默认会在仍有 ephemeral runner 时拒绝 cleanup。只有确认可以中断 job 时才使用 `aktion cleanup --force`。

## Workflow 一直等待 runner

**症状：** GitHub Actions job 长时间处于 queued / waiting for runner。

依次检查：

```text
aktion status
aktion scalesets
aktion logs <repo-id>
```

确认 listener 为 `RUNNING`、GitHub 侧 scale set 为 `FOUND`，并确认 workflow 的 `runs-on` 与实际 label 一致。没有显式 `labels` 时，label 就是 `scaleSetName`；默认值为 `home-docker`。

如果 `status` 显示 Pending 大于 0，但 runner 数量已经达到 `maxRunners`，这是容量限制在起作用。提高该 repository 的上限，或等待现有 job 完成：

```text
aktion config resolve <repo-id>
```

`maxRunners: 0` 表示 unlimited，但不要用它来掩盖主机资源不足。

## Runner image 或 Feature 构建失败

**症状：** `aktion up` / `restart` 在 Resolving runner features 或 Building runner image 阶段失败。

先检查最终解析结果：

```text
aktion config resolve <repo-id>
aktion features
```

确认 Feature URI 和 options 正确。公开 OCI Feature 可以使用 anonymous 或 Bearer-token registry 流程；当前没有 private registry credential 配置，因此需要认证的私有 registry 会失败。

如果 warning 提到 Dev Container lifecycle hook，这是已知限制：Aktion 不执行 `postCreateCommand` 等 lifecycle hooks。需要的项目初始化应放到 GitHub Actions workflow 中。

## 修改了配置但运行状态没有变化

**症状：** `config.json` 已改，但 listener 仍使用旧容量、Feature 或 proxy。

配置文件不会热更新。运行：

```text
aktion config resolve <repo-id>
aktion restart
```

第一条确认解析结果是你预期的，第二条让 listener stack 使用新配置。

如果你手工编辑过 `compose.generated.yaml`，不要依赖其中改动；它会被 Aktion 重新生成。

## 无法通过 proxy 访问 GitHub 或 registry

**症状：** host 上 proxy 正常，但 listener 或 runner 报 connection refused / timeout。

如果 proxy 在 Docker host 上，配置中不能写：

```text
http://localhost:3081
```

应改为：

```text
http://host.docker.internal:3081
```

Aktion 会为 Linux Docker Engine 提供对应 host-gateway 映射。修改后运行 `aktion restart`。

## Runner 很慢或主机内存不足

**症状：** 多个 job 同时开始后，CPU 持续满载、内存接近耗尽或 Docker 明显变慢。

查看一次性资源快照：

```text
aktion stats
```

然后降低该 repository 的 `maxRunners`。记住多个 repository 的上限是彼此独立的；例如三个 repository 都设为 4，主机可能同时运行 12 个 runner。

## 还需要更多日志

跟踪全部 listener：

```text
aktion logs
```

只看一个 repository：

```text
aktion logs <repo-id>
```

排障时优先提供具体失败段落、`aktion doctor` 的失败项和 `aktion config resolve <repo-id>` 输出；不要附带 GitHub App private key 或包含 proxy 密码的原始配置。
