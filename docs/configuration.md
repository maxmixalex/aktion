# 配置

Aktion 只读取 **host 上的 `config.json`**。目标仓库中的文件不能修改 runner image、Features 或权限。

## 最小配置

```json
{
  "app": {
    "clientId": "Iv23liXXXXXXXXXXXXXXXXXX",
    "installationId": 12345678,
    "privateKeyFile": "./secrets/github-app.pem"
  },
  "defaults": {
    "scaleSetName": "home-docker",
    "minRunners": 0,
    "maxRunners": 1
  },
  "repositories": [
    {
      "id": "my-repo",
      "url": "https://github.com/OWNER/REPO",
      "enabled": true
    }
  ]
}
```

## `app`

| 字段 | 含义 |
| --- | --- |
| `clientId` | GitHub App Client ID |
| `installationId` | 默认 installation ID |
| `privateKeyFile` | PEM 文件路径，可相对 `config.json` 所在目录 |

单个 repository 可用自己的 `installationId` 覆盖默认值。

## `defaults`

常用字段：

| 字段 | 默认值 | 含义 |
| --- | --- | --- |
| `scaleSetName` | `home-docker` | workflow 的 `runs-on` label / scale set 名称 |
| `minRunners` | `0` | 最少保留的 runner |
| `maxRunners` | `1` | 每个 repository 的默认 runner 上限；`0` 表示无限 |
| `logLevel` | `info` | `debug/info/warn/error` |
| `labels` | scale set 名称 | 自定义 labels |
| `runnerImage` | 官方 actions runner image | Feature 构建的基础镜像 |
| `runnerFeatures` | Aktion 默认集 | host 提供的 runner Features |
| `extends` | 未设置 | OCI runner preset 列表 |

## Repository 覆盖

`defaults.maxRunners` 是**所有 repository 的默认值**，不是跨 repository 的总并发上限。每个 repository 会启动自己的 listener，并独立使用最终解析出的 `minRunners` / `maxRunners`。

repository 可覆盖 `installationId`、`scaleSetName`、`minRunners`、`maxRunners`、`logLevel`、`labels`、`runnerImage`。例如：

```json
{
  "id": "large-builds",
  "url": "https://github.com/OWNER/LARGE_BUILDS",
  "enabled": true,
  "maxRunners": 4
}
```

若某个 repository 不希望设置上限，写：

```json
{
  "id": "unlimited",
  "url": "https://github.com/OWNER/UNLIMITED",
  "enabled": true,
  "maxRunners": 0
}
```

`maxRunners: 0` 在 Aktion 配置中表示 unlimited；manager 会把它转换为 scaleset client 支持的最大容量。

普通 repository **不需要写 `extends`**：省略时直接继承 defaults，并把自己的 `runnerFeatures` 合并在最后。

```json
{
  "id": "node-project",
  "url": "https://github.com/OWNER/NODE_PROJECT",
  "enabled": true,
  "runnerFeatures": {
    "ghcr.io/devcontainers/features/node:1": {
      "version": "lts"
    }
  }
}
```

同一个 Feature 的 options 做浅层覆盖。写 `false` 可删除继承的 Feature：

```json
"runnerFeatures": {
  "ghcr.io/devcontainers/features/git-lfs:1": false
}
```

### 明确从空环境开始

`extends: []` 表示不继承 defaults：

```json
{
  "id": "minimal",
  "url": "https://github.com/OWNER/MINIMAL",
  "enabled": true,
  "extends": []
}
```

此时若不再指定 Feature，就直接使用 `runnerImage`。

## Proxy

Docker host 上有 HTTP proxy 时：

```json
"proxy": {
  "http": "http://host.docker.internal:3081",
  "https": "http://host.docker.internal:3081",
  "noProxy": "localhost,127.0.0.1"
}
```

不要把容器需要访问的 host proxy 写成 `localhost` 或 `127.0.0.1`。
