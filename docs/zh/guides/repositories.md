# 添加和管理 repository

Aktion 的配置以 repository 为单位。每个 `enabled: true` 的 repository 都会得到自己的 listener 和独立的 runner 容量，因此一台主机可以同时服务多个 repository，而不用让它们共享一份 repository 级状态。

## 添加一个 repository

在 `repositories` 数组中加入一项：

```json
{
  "id": "website",
  "url": "https://github.com/OWNER/website",
  "enabled": true
}
```

`id` 是你在 Aktion CLI 中识别这个 repository 的短名称，例如 `aktion logs website` 或 `aktion config resolve website`。建议只用字母、数字、`-` 和 `_`，并保持唯一。

`url` 必须是 `https://github.com/OWNER/REPO` 形式。GitHub App 必须已经安装到这个 repository。

修改后先验证：

```text
aktion doctor
aktion config resolve website
```

如果 listener 已经在运行，再执行：

```text
aktion restart
```

`restart` 会重新生成 Compose 配置、按需要准备 runner image，并重建 listener stack。

## 使用不同的 GitHub App installation

`app.installationId` 是默认 installation。某个 repository 属于另一个 installation 时，可以覆盖：

```json
{
  "id": "team-repo",
  "url": "https://github.com/OTHER_OWNER/team-repo",
  "enabled": true,
  "installationId": 87654321
}
```

仍然使用同一个 `app.clientId` 和 private key；只有 installation ID 改变。运行 `aktion doctor` 可以验证 installation 是否可访问，以及它是否真的包含目标 repository。

## 使用不同的 scale set 名称

默认情况下，所有 repository 都继承 `defaults.scaleSetName`，通常是 `home-docker`。因为 scale set 是 repository-scoped 的，不同 repository 可以使用同一个名称。

如果某个 repository 的 workflow 需要另一个 label：

```json
{
  "id": "gpu-tools",
  "url": "https://github.com/OWNER/gpu-tools",
  "enabled": true,
  "scaleSetName": "build-host"
}
```

对应 workflow 写：

```yaml
runs-on: build-host
```

`labels` 也可以显式指定自定义 label；省略或留空时，listener 会使用 scale set 名称作为 label。除非你确实需要不同的匹配标签，否则保持两者一致最容易理解。

## 暂时停用

不想再为某个 repository 启动 listener 时，将：

```json
"enabled": false
```

然后运行 `aktion restart`。被禁用的项不会参与配置解析或 listener 启动。配置中必须至少保留一个已启用的 repository，否则 Aktion 会拒绝启动。

如果你是在处理 GitHub 侧遗留的旧 scale set，而不是普通停用，请使用[清理 stale scale set](../operations.md)，不要直接编辑生成的 Compose 文件。
