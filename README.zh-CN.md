<p align="center">
  <img src="docs/zh/assets/aktion.svg" alt="Aktion logo" width="112">
</p>

<h1 align="center">Aktion</h1>

<p align="center"><strong>Aktion 是一个面向 Docker 主机的轻量级 GitHub Actions runner scale-set 管理器。</strong></p>

Aktion 在普通 Docker host 上运行 repository-scoped GitHub Runner Scale Set listener；GitHub 分配 job 后创建临时 Linux runner container，job 完成后销毁。

- 不需要 Kubernetes / ARC cluster
- scale-to-zero ephemeral runner
- GitHub App 认证
- 一个 host 配置多个 repository
- runner image、OCI preset 和 Dev Container Features 全部由 host 控制
- manager 支持 Windows / Linux / macOS；runner 当前只支持 Linux

## 使用文档

完整的面向使用者文档：

**https://zyf722.github.io/aktion/**

## 快速启动

把 GitHub App PEM 放到 `config.json` 指定的位置，然后：

```text
aktion doctor
aktion up
aktion status
```

workflow 默认使用：

```yaml
runs-on: home-docker
```

## 开发

```bash
go test ./...
go vet ./...
bash ./scripts/build-artifacts.sh dist
```

Aktion 基于 GitHub 的独立 `actions/scaleset` client，目前仍处于早期阶段。
