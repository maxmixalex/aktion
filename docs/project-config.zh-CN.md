# Aktion 项目配置与 OCI Preset

Aktion 支持仓库自带的：

```text
.github/aktion.yml
```

项目文件只描述“这个项目需要怎样的 runner 环境”。本机 `config.json`
继续负责 GitHub App、仓库注册、代理、runner 数量上限等基础设施策略。

## 继承必须显式

只要仓库存在 `.github/aktion.yml`，runner 环境就从空配置开始；Aktion
不会暗中追加某个 `recommended`。需要基础环境时必须明确写：

```yaml
version: 1

extends:
  - ghcr.io/zyf722/aktion/recommended:1

runner:
  features:
    builtin:glib: {}
    builtin:gtk4: {}
    builtin:ncurses: {}
    builtin:lua54: {}
```

目前项目仍在 `zyf722` 账号下，所以
`ghcr.io/zyf722/aktion/recommended:1` 是过渡 namespace。若之后迁移到
独立的 `aktion` GitHub organization，预期正式名称为：

```text
ghcr.io/aktion/aktion/recommended:1
```

多个 preset 按 `extends` 顺序从左到右合并，后者覆盖前者；随后项目
自己的 `runner` 覆盖 preset；最后本机
`repositories[].runnerImage` / `repositories[].runnerFeatures`
作为管理员 override，拥有最高优先级。

## Feature 合并

Feature 仍采用 Dev Container Features 的对象模型。对象会浅合并 option；
字符串是 `version` 简写；`true` 启用；`false` 删除继承来的 Feature。

例如：

```yaml
runner:
  features:
    ghcr.io/devcontainers/features/python:1:
      version: "3.13"
    ghcr.io/devcontainers/features/git-lfs:1: false
```

## Preset 真正使用 OCI Artifact

Aktion preset 不是“长得像 OCI 的字符串”，而是真实 OCI 1.1 artifact。

Manifest artifact type：

```text
application/vnd.aktion.preset.v1
```

YAML layer media type：

```text
application/vnd.aktion.preset.v1+yaml
```

当前 recommended preset 的源文件是 `presets/recommended.yml`。使用 ORAS
1.3 可以发布为：

```bash
oras push \
  --artifact-type application/vnd.aktion.preset.v1 \
  ghcr.io/zyf722/aktion/recommended:1 \
  presets/recommended.yml:application/vnd.aktion.preset.v1+yaml
```

平时可使用 tag：

```text
ghcr.io/zyf722/aktion/recommended:1
```

要求完全可复现时可钉 digest：

```text
ghcr.io/zyf722/aktion/recommended@sha256:...
```

## GitHub App 权限与安全边界

Aktion 从仓库 default branch 读取 `.github/aktion.yml`。GitHub App
因此需要：

- Repository administration: Read and write
- Contents: Read
- Metadata: Read（隐含）

固定读取 default branch 是刻意的安全设计：PR 不能靠修改自己分支里的
Aktion 配置，在 job 启动前自行扩大 runner 权限。

没有 `.github/aktion.yml` 的旧仓库暂时保持现有本机配置语义；一旦加入
项目文件，base 就完全由该文件及其 `extends` 明确决定。

可用：

```text
aktion config resolve <repo-id>
```

查看 preset、项目配置和本机管理员覆盖全部合并后的最终 runner 配置。
