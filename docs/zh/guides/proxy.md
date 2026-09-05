# 配置 HTTP/HTTPS proxy

如果 Docker 主机只能通过 proxy 访问 GitHub、OCI registry 或其他外部服务，可以在 `config.json` 的 `proxy` 中统一配置。Aktion 会把这些值传给 listener 和它创建的 runner container。

## 基本配置

```json
{
  "proxy": {
    "http": "http://host.docker.internal:3081",
    "https": "http://host.docker.internal:3081",
    "noProxy": "localhost,127.0.0.1"
  }
}
```

`http` 和 `https` 都必须是完整的 `http://` 或 `https://` URL。只需要一种协议时可以只写对应字段。

!!! warning "不要把 host proxy 写成 localhost"
    container 中的 `localhost` 和 `127.0.0.1` 指向 container 自己，不是 Docker host。proxy 实际运行在 host 上时，应使用 `host.docker.internal`。

Aktion 会为 Linux Docker Engine 增加 `host.docker.internal:host-gateway` 映射，因此同一份配置可以在 Docker Desktop 和普通 Linux Docker host 上使用。

## 排除不应走 proxy 的地址

`noProxy` 会同时传给小写和大写的 `no_proxy` / `NO_PROXY`。它使用常见的逗号分隔形式：

```json
{
  "proxy": {
    "http": "http://host.docker.internal:3081",
    "https": "http://host.docker.internal:3081",
    "noProxy": "localhost,127.0.0.1,.example.internal"
  }
}
```

具体匹配规则由使用这些环境变量的客户端决定。只把确实应该直连的主机或域名加入列表。

## 带用户名和密码的 proxy

URL 可以包含认证信息：

```json
{
  "proxy": {
    "https": "http://user:password@host.docker.internal:3081"
  }
}
```

!!! warning "凭据会写在 config.json 中"
    当前配置格式没有单独的 proxy secret 存储。不要把带用户名或密码的 `config.json` 提交到版本控制，也不要把它作为公开日志或诊断附件发送。

`aktion status` 展示 proxy 时会隐藏 URL 中的用户名和密码，但原始 `config.json` 仍包含它们。

## 验证

修改 proxy 后运行：

```text
aktion doctor
aktion restart
```

如果问题出现在 runner image 的 Feature 构建阶段，再检查：

```text
aktion config resolve <repo-id>
aktion logs <repo-id>
```

如果报错仍显示连接 `localhost`，通常说明 proxy URL 没有改成 host 可达地址。更多症状见[故障排查](../troubleshooting.md)。
