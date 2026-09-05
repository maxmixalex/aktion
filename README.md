# Aktion

**Self-hosted GitHub Actions without Kubernetes.**

Aktion is a lightweight GitHub Runner Scale Set manager for Docker hosts. It keeps one small repository-scoped listener online, creates ephemeral Linux runner containers when GitHub assigns jobs, and removes each runner after one job.

> **Linux runners only.** The Aktion manager itself is cross-platform and can run on Windows, Linux, or macOS wherever the Docker CLI can reach a Docker daemon capable of running Linux containers. Docker Desktop is only one possible daemon provider.

## Why

- scale-to-zero ephemeral Linux runners
- GitHub App authentication
- one configuration for multiple private repositories
- per-repository `minRunners` / `maxRunners`
- rich terminal status with repo → runner → job visibility and OSC 8 links
- stale scale-set reconciliation and cleanup
- optional HTTP/HTTPS proxy forwarding into listener and job containers
- OCI Dev Container Features for runner environments
- host-controlled defaults / OCI presets → repository runner inheritance
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
      ├─ resolves OCI Dev Container Features
      ├─ builds feature-composed runner images
      ├─ diagnoses GitHub App / Docker state
      ├─ shows jobs and runners
      └─ reconciles stale scale sets
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
    "runnerFeatures": {
      "builtin:build": {},
      "builtin:native": {},
      "ghcr.io/devcontainers/features/common-utils:2": {
        "installZsh": false,
        "installOhMyZsh": false,
        "upgradePackages": false,
        "username": "none"
      },
      "ghcr.io/devcontainers/features/python:1": {
        "version": "os-provided",
        "installTools": false
      },
      "ghcr.io/devcontainers/features/git-lfs:1": {
        "autoPull": false
      }
    }
  },
  "repositories": [
    {
      "id": "repo-a",
      "url": "https://github.com/YOUR_NAME/YOUR_PRIVATE_REPO_A",
      "enabled": true,
      "maxRunners": 3
    }
  ]
}
```

Repository values override `defaults`; omitted values inherit them.

Runner environment policy stays entirely on the host. Aktion does not read
`.github/aktion.yml` or any equivalent file from target repositories, so a
repository cannot grant itself additional runner capabilities.

The simplest repository inheritance remains:

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

The host may also reference real OCI runner presets:

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

`defaults.extends` accepts OCI preset references only.
`repositories[].extends` may mix the special `"defaults"` value with OCI
preset references. Sources are applied left-to-right; the host-local
`runnerImage` and `runnerFeatures` fields are applied last.

To keep this model deliberately shallow, an OCI runner preset cannot extend
another preset. It is only a versioned runner environment description:

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

Aktion runner presets use OCI artifact type
`application/vnd.aktion.runner-preset.v1` and JSON layer media type
`application/vnd.aktion.runner-preset.v1+json`. For example:

```bash
oras push \
  --artifact-type application/vnd.aktion.runner-preset.v1 \
  ghcr.io/YOUR_NAME/aktion-presets/native-extra:1 \
  preset.json:application/vnd.aktion.runner-preset.v1+json
```

As with Dev Container Features, public registries using anonymous/Bearer-token
pull are currently supported. Digest-pinned references such as
`@sha256:...` are also accepted.

## Runner Features

`runnerFeatures` follows the Dev Container Features object model: keys are Feature identifiers and values are option objects. Public OCI Features can be referenced directly:

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

The Dev Container shorthand form is also accepted, so a string value maps to the `version` option and `false` disables a Feature.

Aktion resolves OCI manifests and Feature tar layers directly from the registry, validates content digests, recursively resolves `dependsOn`, respects `installsAfter`, validates option types/defaults/enums, and gives every resolved Feature its own Docker build stage. Feature blobs are cached by digest under the user's cache directory.

Runtime Feature metadata is propagated to ephemeral runners:

- `containerEnv`
- `init`
- `privileged`
- `capAdd`
- `securityOpt`
- `mounts`
- `entrypoint`

This is important for official Features such as Go/Rust, which request ptrace/seccomp settings, and Docker/Nix Features, which need runtime configuration.

Dev Container lifecycle hooks (`onCreateCommand`, `postCreateCommand`, and related hooks) are intentionally not executed. GitHub Actions creates the job workspace after the runner starts, so those dev-container lifecycle semantics do not map cleanly to an ephemeral Actions runner. Aktion prints a warning when a selected Feature declares them.

Aktion keeps a small builtin catalog only where the official Dev Container collection has no equivalent:

- `builtin:build`
- `builtin:native`
- `builtin:glib`
- `builtin:gtk4`
- `builtin:ncurses`
- `builtin:lua54`
- `builtin:svn`

The former builtin `network`, `archives`, and `python` Features were removed. The default environment now uses official `common-utils`, `python`, and `git-lfs` OCI Features instead.

At repository level, omitting `runnerFeatures` inherits the default set. When
`runnerFeatures` is present without `extends`, it replaces the default set;
an explicit `{}` therefore uses `runnerImage` unchanged. With
`"extends": "defaults"`, repository Feature entries are merged onto the host
defaults and option objects shallow-override matching Feature options.
`runnerImage` is the base image onto which Features are applied.

`aktion features` lists Aktion builtins and the default OCI Features. Any compatible public OCI Dev Container Feature may be referenced even if it is not listed there.

### Current OCI scope

Public OCI registries using anonymous/Bearer-token pull are supported. Private registry credentials are not yet wired into Aktion.

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
aktion config resolve <repo-id>
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
bash ./scripts/build-artifacts.sh dist
```

Release CI cross-compiles the manager for Linux, Windows, and macOS on amd64/arm64. The listener is intentionally Linux-only and is built for amd64/arm64.

## CI bootstrap

The normal CI workflow uses `runs-on: home-docker`, so Aktion can build Aktion. A separate manual `Bootstrap` workflow uses `ubuntu-latest` as a recovery path if the self-hosted control plane is broken.

## Status

Aktion is early-stage and built on the standalone Runner Scale Set client, which GitHub currently publishes as a preview component. Expect interfaces to evolve.
