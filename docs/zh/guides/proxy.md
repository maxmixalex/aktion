# 配置 HTTP/HTTPS 代理

Docker 主机需要通过代理访问 GitHub、OCI registry 或其他外部服务时，在 `config.json` 的 `proxy` 中配置代理。Aktion 会将这些值传给 listener，以及由 listener 创建的 runner 容器。

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

`http` 和 `https` 必须是完整的 `http://` 或 `https://` URL。环境只使用其中一种代理时，仅配置对应字段。

!!! warning "主机上的代理不要使用 localhost"
    容器中的 `localhost` 和 `127.0.0.1` 指向容器自身，而不是 Docker 主机。如果代理运行在主机上，请使用 `host.docker.internal`。

在 Linux Docker Engine 上，Aktion 会增加 `host.docker.internal:host-gateway` 映射，因此 Docker Desktop 和普通 Linux Docker 主机可以使用相同的主机名。

## 为指定地址绕过代理

`noProxy` 会同时传递为 `no_proxy` 和 `NO_PROXY`，使用常见的逗号分隔形式：

```json
{
  "proxy": {
    "http": "http://host.docker.internal:3081",
    "https": "http://host.docker.internal:3081",
    "noProxy": "localhost,127.0.0.1,.example.internal"
  }
}
```

具体匹配规则由读取这些环境变量的客户端决定。仅将确实需要直连的主机或域名加入列表。

## 代理认证

可以在 URL 中包含认证信息：

```json
{
  "proxy": {
    "https": "http://user:password@host.docker.internal:3081"
  }
}
```

!!! warning "凭据会保存在 config.json 中"
    当前配置格式没有单独的代理密钥字段。不要将包含凭据的 `config.json` 提交到版本控制，也不要把它附在公开日志或诊断信息中。

`aktion status` 显示代理 URL 时会隐藏用户名和密码，但原始 `config.json` 中仍然保存完整凭据。

## 验证配置

修改代理设置后运行：

```text
aktion doctor
aktion restart
```

如果失败发生在基于 Feature 构建 runner image 的阶段，再检查：

```text
aktion config resolve <repo-id>
aktion logs <repo-id>
```

如果错误仍然显示正在访问 `localhost`，通常说明代理地址无法从容器访问。参阅[无法通过代理访问 GitHub 或 registry](../troubleshooting.md#proxy-connectivity)。
