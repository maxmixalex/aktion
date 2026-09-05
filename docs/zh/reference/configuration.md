# 配置参考

Aktion 默认从当前配置根目录读取 `config.json`。可以用 `--root DIR` 改变根目录，用 `--config FILE` 改变文件名。未知字段会被拒绝，因此拼错 key 不会被静默忽略。

## 完整结构

```json
{
  "app": {
    "clientId": "Iv23liXXXXXXXXXXXXXXXXXX",
    "installationId": 12345678,
    "privateKeyFile": "./secrets/github-app.pem"
  },
  "proxy": {
    "http": "http://host.docker.internal:3081",
    "https": "http://host.docker.internal:3081",
    "noProxy": "localhost,127.0.0.1"
  },
  "defaults": {
    "scaleSetName": "home-docker",
    "minRunners": 0,
    "maxRunners": 2,
    "logLevel": "info",
    "runnerFeatures": {
      "ghcr.io/devcontainers/features/node:1": {
        "version": "lts"
      }
    }
  },
  "repositories": [
    {
      "id": "app",
      "url": "https://github.com/OWNER/app",
      "enabled": true
    },
    {
      "id": "large-builds",
      "url": "https://github.com/OWNER/large-builds",
      "enabled": true,
      "maxRunners": 5
    }
  ]
}
```

这只是一个完整形态的示例，不表示你必须写出所有字段。省略可选字段通常比显式复制默认值更容易维护。

## `app`

| 字段 | 类型 | 必需 | 说明 |
| --- | --- | --- | --- |
| `clientId` | string | 是 | GitHub App Client ID。 |
| `installationId` | integer | 是 | repository 未覆盖时使用的 App installation ID。不能为 0。 |
| `privateKeyFile` | string | 是 | GitHub App RSA private key PEM 路径；相对路径以配置根目录为基准。 |

PEM 必须存在并可读取。Aktion 使用 GitHub App 认证，不支持 PAT。

## `defaults`

`defaults` 为普通 repository 提供共享值。repository 没有相应覆盖时使用这里的结果。

| 字段 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `scaleSetName` | string | `home-docker` | GitHub Runner Scale Set 名称；没有自定义 `labels` 时也作为 workflow 的 runner label。 |
| `minRunners` | integer | `0` | 每个 repository 的最小 runner 数量。 |
| `maxRunners` | integer | `1` | 每个 repository 的最大 runner 数量；`0` 表示 unlimited。 |
| `logLevel` | string | `info` | listener 日志级别；使用 `debug`、`info`、`warn` 或 `error`。 |
| `labels` | string | 空 | 显式 runner label；为空时使用 scale set 名称。 |
| `runnerImage` | string | `ghcr.io/actions/actions-runner:latest` | Feature 构建的基础 runner image。 |
| `runnerFeatures` | object | Aktion 默认 Features | 在继承结果上增加、修改或删除 Feature。 |
| `extends` | array[string] | 未设置 | 显式指定 OCI runner preset 继承链；一旦出现，会改变默认 Feature 继承方式。 |

### 容量规则

`minRunners` 与 `maxRunners` 不能为负数。当 `maxRunners > 0` 时，`minRunners` 不能大于 `maxRunners`。

`defaults.maxRunners` **不是全主机总并发**。每个 repository 独立继承这个值。更多例子见[Runner 数量与并发](../guides/capacity.md)。

### `runnerFeatures`

Feature key 是 `builtin:<id>` 或 OCI Dev Container Feature URI。value 支持四种形式：

```json
{
  "runnerFeatures": {
    "ghcr.io/devcontainers/features/node:1": {
      "version": "lts"
    },
    "ghcr.io/devcontainers/features/go:1": "latest",
    "ghcr.io/devcontainers/features/rust:1": true,
    "ghcr.io/devcontainers/features/git-lfs:1": false
  }
}
```

- object：Feature options；
- string：`{"version":"..."}` 简写；
- `true`：启用并采用 publisher 默认 options；
- `false`：从已经继承的 Feature 集中删除。

option value 只接受 string 或 boolean。同一 Feature 在后续配置中再次出现时，options 进行浅层覆盖。

### `defaults.extends`

省略 `defaults.extends` 时，Aktion 从自身默认 Features 开始。

一旦显式设置 `defaults.extends`，Aktion 会从空 Feature 集开始，并按数组顺序应用 OCI preset。例如：

```json
{
  "defaults": {
    "extends": [
      "ghcr.io/OWNER/aktion-presets/base:1",
      "ghcr.io/OWNER/aktion-presets/toolchain:1"
    ]
  }
}
```

`defaults.extends` 不能包含字面值 `"defaults"`。

## `repositories`

每个数组元素表示一个 repository。

| 字段 | 类型 | 必需/默认 | 说明 |
| --- | --- | --- | --- |
| `id` | string | 已启用时必需 | Aktion CLI 中使用的本地标识，也会归一化为 listener service 名称。 |
| `url` | string | 已启用时必需 | `https://github.com/OWNER/REPO`。 |
| `enabled` | boolean | `false` | 只有 `true` 的 repository 会参与运行。 |
| `installationId` | integer | `app.installationId` | 覆盖默认 GitHub App installation。 |
| `scaleSetName` | string | defaults | 覆盖 scale set 名称。 |
| `minRunners` | integer | defaults | 覆盖最小 runner 数。 |
| `maxRunners` | integer | defaults | 覆盖最大 runner 数；`0` = unlimited。 |
| `logLevel` | string | defaults | 覆盖 listener 日志级别。 |
| `labels` | string | defaults | 覆盖 runner label。 |
| `runnerImage` | string | defaults | 覆盖最终基础 image。 |
| `runnerFeatures` | object | defaults 后叠加 | 修改最终 Feature 集。 |
| `extends` | array[string] | 未设置 | 显式重建此 repository 的继承链。 |

至少需要一个 `enabled: true` 的 repository。

### repository 的 `extends`

省略时，repository 自然继承已经解析完成的 defaults。

显式写出 `extends` 时，Aktion 会从空的 image/Feature 继承链开始。要继续使用 defaults，必须把字面值 `"defaults"` 写进数组：

```json
{
  "id": "custom",
  "url": "https://github.com/OWNER/custom",
  "enabled": true,
  "extends": [
    "defaults",
    "ghcr.io/OWNER/aktion-presets/extra:1"
  ]
}
```

`extends: []` 表示不继承 defaults 或任何 preset：

```json
{
  "id": "minimal",
  "url": "https://github.com/OWNER/minimal",
  "enabled": true,
  "extends": []
}
```

最可靠的检查方式是：

```text
aktion config resolve minimal
```

## `proxy`

| 字段 | 类型 | 默认 | 说明 |
| --- | --- | --- | --- |
| `http` | string | 空 | HTTP proxy URL。 |
| `https` | string | 空 | HTTPS proxy URL。 |
| `noProxy` | string | 空 | 逗号分隔的直连地址/域名。 |

`http` 和 `https` 必须使用 `http://` 或 `https://`。Aktion 会拒绝把 proxy host 配成 `localhost`、`127.0.0.1` 或 `::1`，因为 runner container 无法通过自己的 loopback 访问 Docker host；host 上的 proxy 应使用 `host.docker.internal`。

## 验证配置

只解析一个 repository 的最终 runner 环境：

```text
aktion config resolve <repo-id>
```

生成 Compose 并让 Docker Compose 验证：

```text
aktion config
```

同时检查 Docker、GitHub App、installation 和 repository：

```text
aktion doctor
```
