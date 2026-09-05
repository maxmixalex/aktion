# Runner 环境

## 基础镜像

默认基础镜像：

```text
ghcr.io/actions/actions-runner:latest
```

它是轻量的 Linux runner container，不等同于 GitHub-hosted `ubuntu-24.04` VM 的完整预装软件集合。

## 默认 Features

Aktion 默认叠加：

```text
builtin:build
builtin:clang
ghcr.io/devcontainers/features/common-utils:2
ghcr.io/devcontainers/features/python:1
ghcr.io/devcontainers/features/git-lfs:1
```

`builtin:build` 只安装基础 C/C++ 构建需求：

```text
build-essential pkg-config cmake file
```

`builtin:clang` 只安装发行版 `clang` 包。

另外保留两个**非默认** builtin：

```text
builtin:lua54
builtin:svn
```

项目专用开发库应优先由项目 workflow 安装，而不是塞入 host 的公共 runner 基线。

## 当前命令基线

默认环境面向以下常用命令：

```text
git curl wget tar xz unzip python3
cc gcc g++ make pkg-config clang file cmake
sha256sum find timeout
```

## OCI Dev Container Features

可直接写 public OCI Feature：

```json
"runnerFeatures": {
  "ghcr.io/devcontainers/features/go:1": {
    "version": "latest"
  },
  "ghcr.io/devcontainers/features/rust:1": {}
}
```

Aktion 处理 Feature 的 `dependsOn`、`installsAfter`、options，以及这些 runtime metadata：

- `containerEnv`
- `init`
- `privileged`
- `capAdd`
- `securityOpt`
- `mounts`
- `entrypoint`

Dev Container lifecycle hooks 不执行，因为 GitHub Actions job workspace 在 runner 启动后才建立。

当前 registry 支持 anonymous / Bearer-token pull；private registry credentials 尚未接入。

## Host-only OCI runner preset

host 可把一组基础 image + Features 发布为 OCI preset，然后从 `config.json` 引用。目标 repository 无法声明 preset。

preset JSON：

```json
{
  "version": 1,
  "runnerFeatures": {
    "builtin:build": {},
    "ghcr.io/devcontainers/features/common-utils:2": {
      "installZsh": false
    }
  }
}
```

artifact type：

```text
application/vnd.aktion.runner-preset.v1
```

layer media type：

```text
application/vnd.aktion.runner-preset.v1+json
```

preset 本身不能继续 extends 其它 preset。

在 defaults 中引用：

```json
"defaults": {
  "extends": [
    "ghcr.io/OWNER/aktion-presets/base:1"
  ]
}
```

在 repository 中按顺序组合 defaults 与 preset：

```json
"extends": [
  "defaults",
  "ghcr.io/OWNER/aktion-presets/extra:1"
]
```
