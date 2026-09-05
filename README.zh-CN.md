# Aktion

**不用 Kubernetes 的自托管 GitHub Actions。**

Aktion 是一个面向 Docker host 的轻量 GitHub Runner Scale Set manager：每个仓库保留一个很轻的 listener，GitHub 分配任务时创建临时 Linux runner container，一次 job 完成后立即销毁。

> **目前只支持 Linux runner。** Aktion manager 本身是跨平台的：Windows、Linux、macOS 上只要 Docker CLI 能连接到一个可以运行 Linux container 的 Docker daemon 就能使用。Docker Desktop 只是其中一种 daemon provider，并不是架构依赖。

## 主要能力

- scale-to-zero 临时 Linux runners
- GitHub App 认证
- 一个配置管理多个 private repo
- 每个 repo 单独覆盖 `minRunners` / `maxRunners`
- Rich 风格终端状态：repo → runner → job，并支持 OSC 8 可点击链接
- stale scale set 自动复用/reconcile 与显式 `cleanup`
- HTTP/HTTPS proxy 透传到 listener 和 runner
- 直接消费 OCI Dev Container Features 构建 runner 环境
- 完全由 host 控制的 defaults / OCI preset → repository runner 继承
- 不需要 Kubernetes / ARC cluster

Aktion 底层使用 GitHub 官方 [`actions/scaleset`](https://github.com/actions/scaleset) 实现 Scale Set 协议。

## Manager 与 listener

```text
aktion                Windows / Linux / macOS
  └─ 控制 Docker daemon
       └─ aktion-listener        Linux container
            └─ ephemeral runner  Linux container
```

因此发布物包含跨平台的 `aktion` manager，以及预编译的 Linux amd64/arm64 `aktion-listener`。

## 使用

复制：

```text
config.example.json → config.json
```

把 GitHub App PEM 放到 `secrets/`，然后：

```text
aktion doctor
aktion up
aktion status
```

## Host 侧 runner 继承与 OCI preset

runner 环境仍然**完全由 host 的 `config.json` 决定**。Aktion 不读取目标仓库里的
`.github/aktion.yml` 或任何等价文件，repository 本身不能扩大 runner 能力。

最简单的 repo 继承仍然是：

```json
{
  "id": "hpclang",
  "url": "https://github.com/zyf722/hpclang",
  "enabled": true,
  "extends": ["defaults"],
  "runnerFeatures": {
    "builtin:glib": {},
    "builtin:gtk4": {},
    "builtin:ncurses": {},
    "builtin:lua54": {}
  }
}
```

`extends` 也可以由 host 指向真实 OCI runner preset：

```json
{
  "id": "hpclang",
  "url": "https://github.com/zyf722/hpclang",
  "enabled": true,
  "extends": [
    "defaults",
    "ghcr.io/YOUR_NAME/aktion-presets/native-extra:1"
  ],
  "runnerFeatures": {
    "builtin:gtk4": {}
  }
}
```

`defaults.extends` 可以包含 OCI preset，但不能包含特殊值 `"defaults"`。
`repositories[].extends` 可以按顺序混合 `"defaults"` 和 OCI preset。
后面的来源覆盖前面的来源，最后由同一段 host config 中的
`runnerImage` / `runnerFeatures` 覆盖。

为了避免重新演化成复杂配置系统，**OCI preset 自身不能再 extends 其它 preset**。
它只是一个不可执行的 runner 环境描述：

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

Aktion runner preset 使用 OCI artifact type：

```text
application/vnd.aktion.runner-preset.v1
```

JSON layer media type：

```text
application/vnd.aktion.runner-preset.v1+json
```

例如可用 ORAS 发布：

```bash
oras push \
  --artifact-type application/vnd.aktion.runner-preset.v1 \
  ghcr.io/YOUR_NAME/aktion-presets/native-extra:1 \
  preset.json:application/vnd.aktion.runner-preset.v1+json
```

目前和 Dev Container Features 一样，runner preset 支持 public registry 的
anonymous/Bearer-token pull，也可用 `@sha256:...` 固定到不可变 digest。

## Runner Features

`runnerFeatures` 现在直接采用 Dev Container Features 的对象模型：key 是 Feature URI，value 是 Feature options。

例如：

```json
"runnerFeatures": {
  "ghcr.io/devcontainers/features/node:1": {
    "version": "lts"
  },
  "ghcr.io/devcontainers/features/go:1": {
    "version": "latest"
  },
  "ghcr.io/devcontainers/features/rust:1": {}
}
```

也支持 Dev Container 的简写语义：字符串值等价于 `{"version":"..."}`，`false` 表示不启用该 Feature。

Aktion 会直接从 OCI registry：

1. resolve manifest；
2. 处理 Bearer token challenge；
3. 校验 manifest/layer SHA-256；
4. 下载并缓存 Feature tar；
5. 解析 `devcontainer-feature.json`；
6. 递归解析 `dependsOn`；
7. 按 `installsAfter` 排序；
8. 校验 options/default/enum；
9. 为每个 Feature 生成独立 Docker stage/layer；
10. 执行官方 `install.sh`。

Feature layer 会按 digest 缓存在用户 cache 目录，不需要 Aktion 自己复制维护官方语言安装脚本。

### 运行时语义

Aktion 还会把 Feature 的这些 runtime metadata 应用到真正的 ephemeral runner：

- `containerEnv`
- `init`
- `privileged`
- `capAdd`
- `securityOpt`
- `mounts`
- `entrypoint`

因此例如官方 Go/Rust Feature 要求的 `SYS_PTRACE` / `seccomp=unconfined`，以及 Docker/Nix 类 Feature 的 runtime 配置，不会出现“软件装进去了但容器语义没生效”的半支持状态。

Dev Container lifecycle hooks（`onCreateCommand`、`postCreateCommand` 等）目前**明确不执行**。GitHub Actions 的 job workspace 是 runner 启动后才建立的，Dev Container 的 lifecycle 语义不能干净地映射到这种 ephemeral job 模型。Aktion 遇到声明这些 hooks 的 Feature 会打印 warning。

### Aktion 自己还保留什么

只有官方 collection 没有直接对应项的基础依赖仍由 Aktion 内置：

```text
builtin:build
builtin:native
builtin:glib
builtin:gtk4
builtin:ncurses
builtin:lua54
builtin:svn
```

原来的：

```text
network
archives
python
```

已经从仓库中删除，不再自己维护。默认环境改用官方 OCI：

```text
ghcr.io/devcontainers/features/common-utils:2
ghcr.io/devcontainers/features/python:1
ghcr.io/devcontainers/features/git-lfs:1
```

默认配置会关闭 common-utils 的 zsh/Oh My Zsh/package upgrade 等交互式开发容器倾向设置，并让 Python 使用 `os-provided` 且不额外安装一大批编辑器/lint 工具，使 runner image 保持相对克制。

在 repository 层，省略 `runnerFeatures` 表示直接继承 defaults。
如果写了 `runnerFeatures` 但没有 `extends`，仍保持原来的“整段替换”语义；
因此显式 `{}` 表示完全不加 Feature，直接使用 `runnerImage`。
如果写了 `extends`，则其中的 `defaults` / OCI preset 会按顺序增量合并；
同一个 Feature 的 options 做浅层覆盖，`false` 可以删除继承项。

`aktion features` 会列出 Aktion builtin 和默认 OCI Feature，但并不是 allow-list；任意兼容的 public OCI Dev Container Feature URI 都可以直接写进配置。

### 当前 OCI 范围

目前支持 public OCI registry 的 anonymous/Bearer-token pull。private registry credential 还没有接入。

## Proxy

如果 Windows/macOS/Linux Docker host 上有本地 HTTP proxy，例如 3081：

```json
"proxy": {
  "http": "http://host.docker.internal:3081",
  "https": "http://host.docker.internal:3081",
  "noProxy": "localhost,127.0.0.1"
}
```

Aktion 会增加 `host-gateway` 映射，因此普通 Linux Docker Engine 也可以使用同一个 `host.docker.internal` 名称。

## CLI

```text
aktion status
aktion repos
aktion runners
aktion stats
aktion doctor
aktion features
aktion config resolve <repo-id>
aktion logs [repo-id]
aktion scalesets
aktion cleanup
aktion up
aktion down
aktion restart
```

## 构建

```bash
go mod tidy
go test ./...
go vet ./...
bash ./scripts/build-artifacts.sh dist
```

发布 CI 会交叉编译 Linux/Windows/macOS amd64/arm64 manager，以及 Linux amd64/arm64 listener。

## 自举 CI

正常 CI 使用：

```yaml
runs-on: home-docker
```

也就是 **Aktion builds Aktion**。同时保留仅手动触发的 `Bootstrap` workflow 使用 `ubuntu-latest`，用于 self-hosted control plane 自己坏掉时恢复。


## GitHub App 权限

仓库级 scale set 需要把 GitHub App 安装到对应仓库，并授予：

- **Repository administration: Read and write**
- Metadata: Read（隐含）

无需 webhook。
