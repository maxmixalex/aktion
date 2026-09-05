# 定制 runner 环境

Aktion 把 runner 环境视为主机管理员控制的配置，而不是 repository 中 workflow 可以随意改变的基础镜像。你可以使用 Aktion 内置 Feature、公开的 OCI Dev Container Feature，或者 host-only 的 OCI runner preset 组合出环境；项目特有依赖仍然适合在 workflow 中安装。

## 默认环境

如果 `defaults.runnerImage`、`defaults.runnerFeatures` 和 `defaults.extends` 都省略，基础镜像是：

```text
ghcr.io/actions/actions-runner:latest
```

Aktion 再加入以下默认 Features：

| Feature | 默认用途 |
| --- | --- |
| `builtin:build` | `build-essential`、`pkg-config`、`cmake`、`file` |
| `builtin:clang` | 发行版 `clang` 包；依赖 `builtin:build` |
| `ghcr.io/devcontainers/features/common-utils:2` | 常用命令行工具，关闭 Zsh/Oh My Zsh 与升级系统包 |
| `ghcr.io/devcontainers/features/python:1` | 使用基础镜像已有 Python，不额外安装 tools |
| `ghcr.io/devcontainers/features/git-lfs:1` | Git LFS，关闭自动 pull |

因此默认环境适合一般源码 checkout、脚本和 C/C++ 构建，但它**不是** GitHub-hosted Ubuntu VM 的完整软件清单。

你可以随时运行：

```text
aktion features
```

查看当前版本的 builtin 和默认 OCI Features。

## 增加 Dev Container Feature

在 defaults 中加入会影响所有普通继承的 repository：

```json
"runnerFeatures": {
  "ghcr.io/devcontainers/features/node:1": {
    "version": "lts"
  },
  "ghcr.io/devcontainers/features/go:1": "latest",
  "ghcr.io/devcontainers/features/rust:1": true
}
```

Feature 值可以是：

- object：显式 options；
- string：`{"version": "..."}` 的简写；
- `true`：使用 publisher 默认 options；
- `false`：从继承结果中删除这个 Feature。

例如只为一个 repository 删除默认 Git LFS：

```json
{
  "id": "small-repo",
  "url": "https://github.com/OWNER/small-repo",
  "enabled": true,
  "runnerFeatures": {
    "ghcr.io/devcontainers/features/git-lfs:1": false
  }
}
```

同一个 Feature 被多层配置时，options 做浅层合并，后面的值覆盖前面的同名 option。

## 从完全基础的 runner 开始

如果 repository 显式写了 `extends`，Aktion 会按该数组重新建立继承链。最明显的例子是：

```json
{
  "id": "minimal",
  "url": "https://github.com/OWNER/minimal",
  "enabled": true,
  "extends": []
}
```

这表示**不继承 defaults，也不使用 Aktion 默认 Features**。如果没有再写 `runnerFeatures`，最终直接使用基础 runner image。

如果想先继承 defaults，再继续叠加其他 preset，必须明确写入 `"defaults"`：

```json
"extends": [
  "defaults",
  "ghcr.io/OWNER/aktion-presets/extra:1"
]
```

!!! important "显式 extends 会改变继承"
    repository 省略 `extends` 时会自然继承 defaults；写了 `extends` 后，只会按数组中出现的内容组合。每次调整后用 `aktion config resolve <repo-id>` 查看最终 `baseImage`、`image` 和 `features`。

## Host-only OCI preset

当多台 Aktion 主机或多个配置需要复用同一组基础 image 与 Features 时，可以把它们发布为 Aktion runner preset。preset 内容是一个 JSON layer，例如：

```json
{
  "version": 1,
  "runnerImage": "ghcr.io/actions/actions-runner:latest",
  "runnerFeatures": {
    "builtin:build": {},
    "ghcr.io/devcontainers/features/common-utils:2": {
      "installZsh": false
    }
  }
}
```

Aktion 识别的 artifact type 是 `application/vnd.aktion.runner-preset.v1`，layer media type 是 `application/vnd.aktion.runner-preset.v1+json`。preset 自身不能继续 extends 其它 preset；组合顺序由 `config.json` 决定。

## 支持范围与限制

Aktion 会处理 Feature options、`dependsOn`、`installsAfter`、`containerEnv`，以及影响 runner container 的 `init`、`privileged`、`capAdd`、`securityOpt`、`mounts` 和 `entrypoint` 元数据。

Dev Container lifecycle hooks（例如 `postCreateCommand`）不会执行，因为 GitHub job workspace 是 runner 启动后才创建的。包含这些 hooks 的 Feature 会产生 warning。

当前 OCI registry 客户端支持 anonymous pull 和 Bearer-token 流程，但没有配置 private registry credentials 的入口。需要认证的私有 Feature/preset registry 因此不能直接使用。

## 哪些依赖应该留在 workflow

如果某个库只服务一个 repository，或者版本需要跟着项目代码变化，优先在 workflow 中安装。把所有项目依赖都塞进公共 runner image 会增加构建时间，也会让 repository 之间产生不必要的耦合。runner 环境更适合放稳定、跨项目共享且明显能节省每次 job 安装成本的工具。
