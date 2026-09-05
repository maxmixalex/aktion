# 故障排查

## 先运行 doctor

```text
aktion doctor
```

它会同时检查配置文件、PEM、Docker、Compose、GitHub App identity、installation 和当前 listener 状态。

## Docker 不可用

确认：

```text
docker version
docker compose version
```

Windows / macOS 上需要 Docker Desktop 或其它能提供 Linux container 的 daemon。

## GitHub App 验证失败

检查：

- `clientId` 是否来自同一个 GitHub App。
- `installationId` 是否对应安装到目标 repository 的 installation。
- `privateKeyFile` 是否指向可读的 PEM。
- App 是否拥有 **Repository administration: Read and write**。

Aktion listener 只支持 GitHub App，不提供 PAT 认证分支。

## RunnerScaleSetExistsException

通常表示之前的 listener 没有正常删除 GitHub 侧 scale set。

```text
aktion cleanup
```

如果有活跃 runner，先让 job 结束；只有确认可以中断时才用 `--force`。

## Runner image 构建失败

先检查最终环境：

```text
aktion config resolve <repo-id>
aktion features
```

OCI Feature 目前只能从支持 anonymous / Bearer token 的 public registry 拉取。

## Proxy 无法访问

container 中的 `localhost` 指向 container 自己。访问 host 上的 proxy 应使用：

```text
host.docker.internal
```

Aktion 会为普通 Linux Docker Engine 增加对应的 `host-gateway` 映射。

## 查看 listener 日志

```text
aktion logs
aktion logs <repo-id>
```
