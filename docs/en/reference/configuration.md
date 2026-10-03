# Configuration reference

Aktion reads `config.json` from the configuration root by default. The root defaults to the executable directory (resolving symbolic links), regardless of the working directory. Use `--root DIR` to change the root and `--config FILE` to change the filename. Unknown JSON fields are rejected, so a misspelled key is not silently ignored.

## Complete shape

```json
{
  "app": {
    "clientId": "Iv23liXXXXXXXXXXXXXXXXXX",
    "installationId": 12345678,
    "privateKeyFile": "./secrets/github-app.pem"
  },
  "proxy": {
    "http": "http://host.docker.internal:3081",
    "https": "http://host.docker.internal:3081",
    "noProxy": "localhost,127.0.0.1"
  },
  "defaults": {
    "scaleSetName": "home-docker",
    "minRunners": 0,
    "maxRunners": 2,
    "logLevel": "info",
    "runnerFeatures": {
      "ghcr.io/devcontainers/features/node:1": {
        "version": "lts"
      }
    }
  },
  "repositories": [
    {
      "id": "app",
      "url": "https://github.com/OWNER/app",
      "enabled": true
    },
    {
      "id": "large-builds",
      "url": "https://github.com/OWNER/large-builds",
      "enabled": true,
      "maxRunners": 5
    }
  ]
}
```

This shows the overall structure, not a requirement to spell out every optional field. Omitting values that can safely use defaults is usually easier to maintain.

## `app`

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `clientId` | string | Yes | GitHub App Client ID. |
| `installationId` | integer | Yes | Default App installation ID; may not be 0. |
| `privateKeyFile` | string | Yes | GitHub App RSA private-key PEM path; relative paths are resolved from the configuration root. |

The PEM must exist and be readable. Aktion uses GitHub App authentication and does not support PAT authentication.

## `defaults`

Repositories following normal inheritance use these values when they do not override them.

| Field | Type | Default | Meaning |
| --- | --- | --- | --- |
| `scaleSetName` | string | `home-docker` | Runner Scale Set name; also the workflow runner label when `labels` is empty. |
| `minRunners` | integer | `0` | Minimum runners per repository. |
| `maxRunners` | integer | `1` | Maximum runners per repository; `0` means unlimited. |
| `logLevel` | string | `info` | Listener logging: `debug`, `info`, `warn`, or `error`. |
| `labels` | string | Empty | Explicit runner label; empty uses the scale-set name. |
| `runnerImage` | string | `ghcr.io/actions/actions-runner:latest` | Base image used when constructing the final runner image. |
| `runnerFeatures` | object | Aktion default Features | Add, change, or remove Features from the inherited result. |
| `extends` | array[string] | Unset | Explicit OCI runner-preset inheritance chain; when present, it changes default Feature inheritance. |

### Capacity rules

`minRunners` and `maxRunners` may not be negative. When `maxRunners > 0`, `minRunners` may not be greater than `maxRunners`.

`defaults.maxRunners` is **not** a host-wide concurrency limit. Each repository independently inherits it. See [runner capacity and concurrency](../guides/capacity.md).

### `runnerFeatures`

Feature keys are `builtin:<id>` references or OCI Dev Container Feature URIs. Values support four forms:

```json
{
  "runnerFeatures": {
    "ghcr.io/devcontainers/features/node:1": {
      "version": "lts"
    },
    "ghcr.io/devcontainers/features/go:1": "latest",
    "ghcr.io/devcontainers/features/rust:1": true,
    "ghcr.io/devcontainers/features/git-lfs:1": false
  }
}
```

- object: explicit Feature options;
- string: shorthand for `{"version":"..."}`;
- `true`: enable with publisher defaults;
- `false`: remove that Feature from the inherited set.

Option values must be strings or booleans. Options for the same Feature are shallow-merged, with later values replacing options of the same name.

### `defaults.extends`

When `defaults.extends` is omitted, Aktion begins with its built-in default Feature set.

When it is explicitly present, Aktion begins with an empty Feature set and applies OCI presets in order:

```json
{
  "defaults": {
    "extends": [
      "ghcr.io/OWNER/aktion-presets/base:1",
      "ghcr.io/OWNER/aktion-presets/toolchain:1"
    ]
  }
}
```

`defaults.extends` may not contain the literal `"defaults"`.

## `repositories`

Each array entry describes one repository.

| Field | Type | Required/default | Meaning |
| --- | --- | --- | --- |
| `id` | string | Required when enabled | Local identifier used by Aktion commands and normalized for the listener service name. |
| `url` | string | Required when enabled | `https://github.com/OWNER/REPO`. |
| `enabled` | boolean | `false` | Only enabled repositories participate in runtime configuration. |
| `installationId` | integer | `app.installationId` | Override the default GitHub App installation. |
| `scaleSetName` | string | defaults | Override the scale-set name. |
| `minRunners` | integer | defaults | Override minimum runners. |
| `maxRunners` | integer | defaults | Override maximum runners; `0` = unlimited. |
| `logLevel` | string | defaults | Override listener logging. |
| `labels` | string | defaults | Override the runner label. |
| `runnerImage` | string | defaults | Override the final base image. |
| `runnerFeatures` | object | Overlaid on inherited features | Modify the final Feature set. |
| `extends` | array[string] | Unset | Explicitly rebuild this repository's inheritance chain. |

At least one repository must have `enabled: true`.

### Repository `extends`

When omitted, a repository automatically inherits the fully resolved defaults.

When present, Aktion starts that repository's image/Feature inheritance from empty state. To include defaults, put the literal `"defaults"` in the array:

```json
{
  "id": "custom",
  "url": "https://github.com/OWNER/custom",
  "enabled": true,
  "extends": [
    "defaults",
    "ghcr.io/OWNER/aktion-presets/extra:1"
  ]
}
```

`extends: []` means inherit neither defaults nor any preset:

```json
{
  "id": "minimal",
  "url": "https://github.com/OWNER/minimal",
  "enabled": true,
  "extends": []
}
```

Inspect the result with:

```text
aktion config resolve minimal
```

## `proxy`

| Field | Type | Default | Meaning |
| --- | --- | --- | --- |
| `http` | string | Empty | HTTP proxy URL. |
| `https` | string | Empty | HTTPS proxy URL. |
| `noProxy` | string | Empty | Comma-separated hosts/domains that should connect directly. |

`http` and `https` must use `http://` or `https://`. Aktion rejects proxy hosts set to `localhost`, `127.0.0.1`, or `::1` because a container cannot reach a host-side proxy through its own loopback. Use `host.docker.internal` for a proxy running on the Docker host.

## Validate configuration

Inspect one repository's effective runner environment:

```text
aktion config resolve <repo-id>
```

Generate Compose and run Docker Compose validation:

```text
aktion config
```

Check Docker, GitHub App, installations, and repositories as well:

```text
aktion doctor
```
