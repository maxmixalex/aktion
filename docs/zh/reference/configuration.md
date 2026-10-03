# 配置参考

Aktion 默认从配置根目录读取 `config.json`。默认根目录是可执行文件所在目录（符号链接解析到目标文件），不随当前工作目录变化。使用 `--root DIR` 可以更改配置根目录，使用 `--config FILE` 可以指定其他配置文件名。配置中出现未知 JSON 字段时，Aktion 会报错，因此拼写错误的字段不会被静默忽略。

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

该示例展示了配置的整体结构，并不要求实际配置写出所有可选字段。能够安全使用默认值的字段通常无需重复声明。

## `app`

| 字段 | 类型 | 必需 | 说明 |
| --- | --- | --- | --- |
| `clientId` | string | 是 | GitHub App Client ID。 |
| `installationId` | integer | 是 | 默认 GitHub App installation ID，不能为 `0`。 |
| `privateKeyFile` | string | 是 | GitHub App RSA 私钥 PEM 的路径；相对路径以配置根目录为基准。 |

PEM 文件必须存在且可读。Aktion 仅支持 GitHub App 认证，不支持 PAT 认证。

## `defaults`

按默认方式继承配置的仓库，如果没有覆盖相应字段，就会使用 `defaults` 中的值。

| 字段 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `scaleSetName` | string | `home-docker` | Runner Scale Set 名称；`labels` 为空时也作为工作流匹配 runner 的标签。 |
| `minRunners` | integer | `0` | 每个仓库保留的最少 runner 数量。 |
| `maxRunners` | integer | `1` | 每个仓库允许的最多 runner 数量；`0` 表示不设上限。 |
| `logLevel` | string | `info` | listener 日志级别：`debug`、`info`、`warn` 或 `error`。 |
| `labels` | string | 空 | 显式指定 runner 标签；为空时使用 scale set 名称。 |
| `runnerImage` | string | `ghcr.io/actions/actions-runner:latest` | 构建最终 runner image 时使用的基础 image。 |
| `runnerFeatures` | object | Aktion 默认 Feature | 在继承结果上增加、修改或删除 Feature。 |
| `extends` | array[string] | 未设置 | 显式指定 OCI runner preset 的继承顺序；设置后会改变默认 Feature 的继承方式。 |

### 容量限制

`minRunners` 和 `maxRunners` 不能为负数。当 `maxRunners > 0` 时，`minRunners` 不能大于 `maxRunners`。

`defaults.maxRunners` 不是整台主机的总并发上限。每个仓库都会独立继承该值。示例见 [配置 runner 数量和并发](../guides/capacity.md)。

### `runnerFeatures`

Feature key 可以是 `builtin:<id>`，也可以是 OCI Dev Container Feature URI。value 支持四种形式：

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

- object：显式指定 Feature options；
- string：`{"version":"..."}` 的简写；
- `true`：启用 Feature，并使用发布者提供的默认 options；
- `false`：从继承结果中删除该 Feature。

option value 只能是 string 或 boolean。同一个 Feature 在多个位置出现时，options 会浅层合并；后出现的同名 option 会覆盖前面的值。

### `defaults.extends`

未设置 `defaults.extends` 时，Aktion 从自身的默认 Feature 集开始解析配置。

显式设置该字段后，Aktion 会从空 Feature 集开始，并按数组顺序应用 OCI preset：

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

`repositories` 数组中的每一项表示一个仓库。

| 字段 | 类型 | 必需/默认值 | 说明 |
| --- | --- | --- | --- |
| `id` | string | 启用时必需 | Aktion CLI 使用的本地标识；Aktion 也会据此生成 listener 的服务名。 |
| `url` | string | 启用时必需 | 仓库 URL，格式为 `https://github.com/OWNER/REPO`。 |
| `enabled` | boolean | `false` | 只有启用的仓库才参与运行配置。 |
| `installationId` | integer | `app.installationId` | 覆盖默认 GitHub App installation。 |
| `scaleSetName` | string | defaults | 覆盖 scale set 名称。 |
| `minRunners` | integer | defaults | 覆盖最少 runner 数量。 |
| `maxRunners` | integer | defaults | 覆盖最多 runner 数量；`0` 表示不设上限。 |
| `logLevel` | string | defaults | 覆盖 listener 日志级别。 |
| `labels` | string | defaults | 覆盖 runner 标签。 |
| `runnerImage` | string | defaults | 覆盖最终使用的基础 image。 |
| `runnerFeatures` | object | 在继承结果上叠加 | 修改最终 Feature 集。 |
| `extends` | array[string] | 未设置 | 显式重新构建该仓库的继承关系。 |

配置中至少要有一个 `enabled: true` 的仓库。

### 仓库的 `extends`

未设置 `extends` 时，仓库会自动继承已经解析完成的 defaults。

显式设置 `extends` 后，Aktion 会从空的 image 和 Feature 继承状态开始。要继续使用 defaults，必须在数组中加入字面值 `"defaults"`：

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

`extends: []` 表示既不继承 defaults，也不应用任何 preset：

```json
{
  "id": "minimal",
  "url": "https://github.com/OWNER/minimal",
  "enabled": true,
  "extends": []
}
```

运行以下命令检查最终结果：

```text
aktion config resolve minimal
```

## `proxy`

| 字段 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `http` | string | 空 | HTTP 代理 URL。 |
| `https` | string | 空 | HTTPS 代理 URL。 |
| `noProxy` | string | 空 | 以逗号分隔、需要直连的主机或域名。 |

`http` 和 `https` 必须使用 `http://` 或 `https://`。Aktion 会拒绝将代理主机设置为 `localhost`、`127.0.0.1` 或 `::1`，因为容器不能通过自身的 loopback 访问主机上的代理。代理运行在 Docker 主机上时，请使用 `host.docker.internal`。

## 验证配置

查看单个仓库最终解析出的 runner 环境：

```text
aktion config resolve <repo-id>
```

生成 Compose 文件，并运行 Docker Compose 配置验证：

```text
aktion config
```

同时检查 Docker、GitHub App、installation 和仓库：

```text
aktion doctor
```
