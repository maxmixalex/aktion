# 添加和管理仓库

Aktion 按仓库配置。每个 `enabled: true` 的仓库都有独立的 listener 和 runner 容量，因此一台主机可以同时服务多个仓库，各仓库的运行状态和容量互不影响。

## 添加仓库

在 `repositories` 数组中增加一项：

```json
{
  "id": "website",
  "url": "https://github.com/OWNER/website",
  "enabled": true
}
```

`id` 是 Aktion CLI 中使用的短名称，例如 `aktion logs website` 和 `aktion config resolve website`。建议使用字母、数字、`-` 和 `_`，并确保各仓库的 ID 唯一。

`url` 必须采用 `https://github.com/OWNER/REPO` 形式。GitHub App 必须已经安装到该仓库。

重启前先验证配置：

```text
aktion doctor
aktion config resolve website
```

如果 listener 已经在运行，再执行：

```text
aktion restart
```

`restart` 会重新生成 Compose 配置，按需准备 runner image，并重新创建 listener。

## 使用其他 GitHub App installation

`app.installationId` 是默认 installation。某个仓库属于其他 installation 时，可在仓库配置中覆盖：

```json
{
  "id": "team-repo",
  "url": "https://github.com/OTHER_OWNER/team-repo",
  "enabled": true,
  "installationId": 87654321
}
```

`app.clientId` 和私钥保持不变，仅 installation ID 不同。`aktion doctor` 会检查该 installation 是否可访问，以及它是否包含目标仓库。

## 使用其他 scale set 名称

仓库默认继承 `defaults.scaleSetName`，其默认值为 `home-docker`。scale set 的作用域是单个仓库，因此不同仓库可以使用相同的名称。

如需让某个仓库使用其他标签，可覆盖 scale set 名称：

```json
{
  "id": "gpu-tools",
  "url": "https://github.com/OWNER/gpu-tools",
  "enabled": true,
  "scaleSetName": "build-host"
}
```

对应工作流使用：

```yaml
runs-on: build-host
```

也可以显式设置 `labels`。省略 `labels` 或将其留空时，listener 会使用 scale set 名称作为标签。没有明确需要时，让两者保持一致更容易维护。

## 停用仓库

将配置改为：

```json
"enabled": false
```

然后运行 `aktion restart`。停用的仓库不会参与配置解析，也不会启动 listener。配置中必须至少有一个 `enabled: true` 的仓库，否则 Aktion 会拒绝加载配置。

如果 GitHub 端残留了 listener 异常退出后留下的 scale set，请使用[清理遗留的 scale set](../operations.md#cleanup-stale-scale-set)，不要修改生成的 Compose 文件。
