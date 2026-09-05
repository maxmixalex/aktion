# 安装与启动

## 前置条件

主机需要：

- Docker Engine 或 Docker Desktop，并能够运行 Linux containers。
- Docker Compose v2（`docker compose`）。
- 一个安装到目标仓库的 GitHub App。

GitHub App 需要：

- **Repository administration: Read and write**
- Metadata: Read（GitHub 隐含授予）
- 不需要 webhook

## 运行包

Windows amd64 运行包采用固定的最小结构：

```text
aktion.exe
config.json
Dockerfile
packaging/
  start-listener.sh
dist/
  aktion-listener-linux-amd64
```

GitHub App PEM 不会包含在发布包中。按 `config.json` 的 `privateKeyFile` 路径自行放置，例如：

```text
secrets/
  github-app.pem
```

## 首次启动

先检查配置和依赖：

```powershell
.\aktion.exe doctor
```

启动 listener：

```powershell
.\aktion.exe up
```

查看状态：

```powershell
.\aktion.exe status
```

默认 scale set 名称为 `home-docker`。workflow 使用：

```yaml
jobs:
  build:
    runs-on: home-docker
    steps:
      - uses: actions/checkout@v5
      - run: echo hello
```

停止：

```powershell
.\aktion.exe down
```
