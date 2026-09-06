# 配置 runner 环境

runner 环境由 Aktion 主机配置，仓库不能通过工作流重新定义 Aktion 使用的基础环境。可以组合 Aktion 内置 Feature、公开的 OCI Dev Container Feature，以及仅由主机配置的 OCI runner preset。只服务于单个项目的依赖通常更适合在该项目的工作流中安装。

## 默认环境

如果省略 `defaults.runnerImage`、`defaults.runnerFeatures` 和 `defaults.extends`，基础 image 为：

```text
ghcr.io/actions/actions-runner:latest
```

Aktion 随后加入以下默认 Feature：

| Feature | 用途 |
| --- | --- |
| `builtin:build` | 安装 `build-essential`、`pkg-config`、`cmake` 和 `file` |
| `builtin:clang` | 安装发行版提供的 `clang`；依赖 `builtin:build` |
| `ghcr.io/devcontainers/features/common-utils:2` | 提供常用命令行工具，并关闭 Zsh、Oh My Zsh 和系统包升级 |
| `ghcr.io/devcontainers/features/python:1` | 使用基础 image 中已有的 Python，不额外安装工具 |
| `ghcr.io/devcontainers/features/git-lfs:1` | 提供 Git LFS，并关闭自动 pull |

该环境适合常见的源码检出、脚本和 C/C++ 构建，但它不等同于 GitHub-hosted Ubuntu VM 的完整预装软件环境。

运行：

```text
aktion features
```

可查看当前 Aktion 版本提供的内置 Feature 和默认 OCI Feature。

## 添加 Dev Container Feature

在 defaults 中增加 Feature，会影响所有按默认方式继承配置的仓库：

```json
"runnerFeatures": {
  "ghcr.io/devcontainers/features/node:1": {
    "version": "lts"
  },
  "ghcr.io/devcontainers/features/go:1": "latest",
  "ghcr.io/devcontainers/features/rust:1": true
}
```

Feature 的值支持以下形式：

- object：显式指定 options；
- string：`{"version": "..."}` 的简写；
- `true`：启用 Feature，并使用发布者提供的默认 options；
- `false`：从继承结果中删除该 Feature。

例如，只对一个仓库移除默认的 Git LFS Feature：

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

同一个 Feature 在多处配置中出现时，options 会浅层合并；后出现的同名 option 会覆盖前面的值。

## 从基础 runner image 开始

仓库显式设置 `extends` 后，Aktion 会按照该数组重新构建继承关系。最简单的形式是：

```json
{
  "id": "minimal",
  "url": "https://github.com/OWNER/minimal",
  "enabled": true,
  "extends": []
}
```

这表示该仓库既不继承 defaults，也不使用 Aktion 的默认 Feature。如果没有再设置 `runnerFeatures`，最终会使用基础 runner image。

如果需要先继承 defaults，再叠加其他 preset，请显式加入字面值 `"defaults"`：

```json
"extends": [
  "defaults",
  "ghcr.io/OWNER/aktion-presets/extra:1"
]
```

!!! important "显式设置 extends 会改变继承关系"
    仓库未设置 `extends` 时，会自动继承 defaults。只要 `extends` 出现，Aktion 就只按数组中列出的来源组合。修改后运行 `aktion config resolve <repo-id>`，检查最终的 `baseImage`、`image` 和 `features`。

## 仅由主机配置的 OCI runner preset

多台 Aktion 主机或多份配置需要复用相同的基础 image 和 Feature 时，可以将它们发布为 Aktion runner preset。preset 包含一个 JSON layer，例如：

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

Aktion 识别的 artifact type 为 `application/vnd.aktion.runner-preset.v1`，layer media type 为 `application/vnd.aktion.runner-preset.v1+json`。preset 本身不能继续继承其他 preset；组合顺序由 `config.json` 决定。

## 支持范围与限制

Aktion 支持 Feature options、`dependsOn`、`installsAfter` 和 `containerEnv`，以及会影响 runner 容器的 `init`、`privileged`、`capAdd`、`securityOpt`、`mounts` 和 `entrypoint` 元数据。

Dev Container lifecycle hook（例如 `postCreateCommand`）不会执行，因为 GitHub 作业的 workspace 在 runner 启动后才创建。Feature 声明 lifecycle hook 时，Aktion 会显示 warning。

当前 OCI 客户端支持匿名 pull 和 Bearer token 流程，但没有私有 registry 凭据的配置入口。因此，需要私有认证的 Feature 或 preset registry 无法使用。

## 将项目专用依赖保留在工作流中

如果某个依赖只用于一个仓库，或者其版本需要随项目代码一起变化，应在工作流中安装。把所有项目依赖都加入共享 runner image 会增加 image 构建时间，也会让原本无关的仓库相互依赖。runner 环境更适合放置多个项目共用、版本相对稳定，并且每次作业重复安装成本较高的工具。
