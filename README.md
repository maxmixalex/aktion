# Aktion

**Self-hosted GitHub Actions without Kubernetes.**

Aktion is a lightweight manager for GitHub Runner Scale Sets on a Docker host. It keeps a small repository-scoped listener online, scales Linux runner containers from zero when GitHub assigns jobs, and removes each runner after one job.

> **Linux runners only.** The Aktion manager itself is cross-platform and is intended to run on Windows, Linux, or macOS wherever the Docker CLI can reach a Linux Docker Engine/Desktop.

## Why

- scale-to-zero ephemeral Linux runners
- GitHub App authentication
- one configuration for multiple private repositories
- per-repository `minRunners` / `maxRunners`
- rich terminal status with repo → runner → job visibility and OSC 8 links
- stale scale-set diagnosis and cleanup
- optional HTTP/HTTPS proxy forwarding into listener and job containers
- composable runner image features
- no Kubernetes / ARC cluster required

Aktion uses GitHub's standalone [`actions/scaleset`](https://github.com/actions/scaleset) client for the Runner Scale Set protocol.

## Architecture

```text
GitHub Actions
      │
      │ Runner Scale Set protocol
      ▼
aktion-listener (Linux container, one per repository)
      │
      │ Docker API
      ▼
ephemeral Linux runner containers
      │
      └─ one job → destroy

aktion (manager, Windows/Linux/macOS)
      ├─ renders Compose
      ├─ manages listeners
      ├─ builds feature-composed runner images
      ├─ diagnoses GitHub App / Docker state
      ├─ shows jobs and runners
      └─ cleans stale scale sets
```

## Configuration

Copy `config.example.json` to `config.json`, place the GitHub App PEM under `secrets/`, then edit the repositories.

```json
{
  "app": {
    "clientId": "Iv23liXXXXXXXXXXXXXXXXXX",
    "installationId": 12345678,
    "privateKeyFile": "./secrets/github-app.pem"
  },
  "defaults": {
    "scaleSetName": "home-docker",
    "minRunners": 0,
    "maxRunners": 1,
    "logLevel": "info",
    "labels": "",
    "runnerImage": "",
    "runnerFeatures": ["build", "network", "python", "archives", "native"]
  },
  "repositories": [
    {
      "id": "repo-a",
      "url": "https://github.com/YOUR_NAME/YOUR_PRIVATE_REPO_A",
      "enabled": true,
      "maxRunners": 3
    }
  ],
  "proxy": {
    "http": "",
    "https": "",
    "noProxy": "localhost,127.0.0.1"
  }
}
```

Repository values override `defaults`; omitted values inherit them.

## Runner Features

Aktion can build a runner image from small composable features instead of forcing every project onto one giant image. Built-in features are declared with metadata plus an idempotent `install.sh`, inspired by Dev Container Features. Hard dependencies are resolved with `dependsOn`; `installsAfter` provides a soft ordering edge when both features are selected.

The default feature set is:

```json
"runnerFeatures": ["build", "network", "python", "archives", "native"]
```

This provides common compiler/build tooling, source/network utilities, Python, archive tools, and common native development headers. Optional features include `glib`, `gtk4`, `ncurses`, and `lua54`.

A repository can replace the inherited feature set:

```json
{
  "id": "desktop-native",
  "url": "https://github.com/example/desktop-native",
  "enabled": true,
  "runnerFeatures": [
    "build", "network", "python", "archives", "native",
    "gtk4", "ncurses", "lua54"
  ]
}
```

Omit `runnerFeatures` to inherit; set it to `[]` to use the base runner image unchanged. `runnerImage` is the base image onto which features are applied.

Each resolved feature becomes its own Docker build stage/layer. Aktion hashes the base image reference plus resolved feature contents and reuses the same generated image for repositories with the same plan. `aktion features` lists the built-in catalog.

The current built-ins target Debian/Ubuntu-style runner images because their install scripts use `apt`.

## Proxy

For a proxy running on the Docker host, use `host.docker.internal`, for example:

```json
"proxy": {
  "http": "http://host.docker.internal:3081",
  "https": "http://host.docker.internal:3081",
  "noProxy": "localhost,127.0.0.1"
}
```

Aktion adds the Docker `host-gateway` mapping so this name also works with a normal Linux Docker Engine.

## CLI

```text
aktion status
aktion repos
aktion runners
aktion stats
aktion doctor
aktion features
aktion logs [repo-id]
aktion scalesets
aktion cleanup
aktion up
aktion down
aktion restart
```

`status` shows repository/listener capacity plus active runner/job metadata. `--color auto|always|never` controls color and `--links auto|always|never` controls terminal hyperlinks.

## GitHub App

For repository-scoped scale sets, install the App on each configured repository and grant:

- **Repository administration: Read and write**
- Metadata read access (implicit)

No webhook is required.

## Building

Aktion requires Go 1.25.3 or newer because the current `actions/scaleset` dependency does.

```bash
go mod tidy
go test ./...
go vet ./...
go build -o dist/aktion ./cmd/aktion
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/aktion-listener-linux-amd64 ./cmd/aktion-listener
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o dist/aktion-listener-linux-arm64 ./cmd/aktion-listener
```

The manager is portable; release CI cross-compiles it for Windows, Linux and macOS. The listener is intentionally Linux-only.

## CI bootstrap

The normal CI workflow uses `runs-on: home-docker`, so Aktion can build Aktion. A separate manual `Bootstrap` workflow uses `ubuntu-latest` as a recovery path if the self-hosted control plane is broken.

## Status

Aktion is early-stage and built on the standalone Runner Scale Set client, which GitHub currently publishes as a preview component. Expect interfaces to evolve.
