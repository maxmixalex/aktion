# Customize the runner environment

Aktion treats the runner environment as host-administered configuration rather than something a repository can redefine through its workflow. You can combine Aktion builtins, public [Dev Container Features](https://containers.dev/features), and host-only runner presets distributed through [OCI](https://opencontainers.org/about/overview/) registries. Dependencies specific to one project are usually better installed by that project's workflow.

## Default environment

If `defaults.runnerImage`, `defaults.runnerFeatures`, and `defaults.extends` are all omitted, the base image is:

```text
ghcr.io/actions/actions-runner:latest
```

Aktion then adds these default Features:

| Feature | Default purpose |
| --- | --- |
| `builtin:build` | `build-essential`, `pkg-config`, `cmake`, and `file` |
| `builtin:clang` | Distribution `clang` package; depends on `builtin:build` |
| `ghcr.io/devcontainers/features/common-utils:2` | Common CLI tools with Zsh/Oh My Zsh and package upgrades disabled |
| `ghcr.io/devcontainers/features/python:1` | Use the Python already present in the base image; do not install tools |
| `ghcr.io/devcontainers/features/git-lfs:1` | Git LFS with automatic pull disabled |

The result is useful for common source checkouts, scripts, and C/C++ builds, but it is **not** the complete software inventory of a [GitHub-hosted runner](https://docs.github.com/en/actions/concepts/runners/github-hosted-runners).

Run:

```text
aktion features
```

to see the builtins and default OCI Features in the version you are using.

## Add a Dev Container Feature

Adding Features under defaults affects every repository that follows normal defaults inheritance:

```json
"runnerFeatures": {
  "ghcr.io/devcontainers/features/node:1": {
    "version": "lts"
  },
  "ghcr.io/devcontainers/features/go:1": "latest",
  "ghcr.io/devcontainers/features/rust:1": true
}
```

A Feature value may be:

- an object containing options;
- a string, shorthand for `{"version": "..."}`;
- `true`, which uses publisher defaults;
- `false`, which removes that Feature from the inherited result.

For example, remove the default Git LFS Feature for one repository:

```json
{
  "id": "small-repo",
  "url": "https://github.com/OWNER/small-repo",
  "enabled": true,
  "runnerFeatures": {
    "ghcr.io/devcontainers/features/git-lfs:1": false
  }
}
```

When the same Feature appears in several stages of configuration, its options are shallow-merged and later values replace options with the same name.

## Start from the bare runner image

If a repository explicitly contains `extends`, Aktion rebuilds its inheritance chain from the items in that array. The clearest example is:

```json
{
  "id": "minimal",
  "url": "https://github.com/OWNER/minimal",
  "enabled": true,
  "extends": []
}
```

This means **do not inherit defaults and do not use Aktion's default Features**. If no `runnerFeatures` are added, the repository uses the base runner image directly.

To start with defaults and then apply another preset, include the literal `"defaults"`:

```json
"extends": [
  "defaults",
  "ghcr.io/OWNER/aktion-presets/extra:1"
]
```

!!! important "Explicit extends changes inheritance"
    With no repository `extends`, defaults are inherited automatically. Once `extends` is present, only the listed sources are composed. After changing it, run `aktion config resolve <repo-id>` to inspect the final `baseImage`, `image`, and `features`.

## Host-only OCI presets

If several Aktion hosts or configurations need the same base image and Features, you can publish them as an Aktion runner preset. The preset contains one JSON layer such as:

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

Aktion recognizes artifact type `application/vnd.aktion.runner-preset.v1` and layer media type `application/vnd.aktion.runner-preset.v1+json`. A preset cannot itself extend another preset; ordering is controlled by `config.json`.

## Supported metadata and limitations

Aktion handles Feature options, `dependsOn`, `installsAfter`, and `containerEnv`, plus runner-container metadata including `init`, `privileged`, `capAdd`, `securityOpt`, `mounts`, and `entrypoint`.

Dev Container lifecycle hooks such as `postCreateCommand` are not executed because the GitHub job workspace is created after the runner starts. Features declaring lifecycle hooks produce a warning.

The OCI client currently supports anonymous pulls and Bearer-token flows, but there is no configuration for private-registry credentials. Features or presets that require authenticated private registry access therefore cannot be used directly.

## Keep project dependencies in the workflow

If a library belongs to one repository or its version should move with project code, install it in the workflow. Putting every project dependency into the shared runner image increases image build time and couples unrelated repositories. The runner environment is most useful for stable tools shared across projects and expensive to install for every job.
