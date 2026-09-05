# Aktion project configuration and OCI presets

Aktion supports an optional repository-owned configuration at:

```text
.github/aktion.yml
```

The file describes the runner environment required by the project. Host-local
`config.json` remains responsible for GitHub App credentials, repository
registration, proxy settings, scale limits, and other infrastructure policy.

## Explicit inheritance

A project config starts from an empty runner environment. There is no implicit
`recommended` preset. Reusable bases are named explicitly with `extends`:

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

The `ghcr.io/zyf722/aktion/recommended:1` reference is the provisional
namespace while Aktion remains under the `zyf722` account. The intended
project namespace after an organization migration is
`ghcr.io/aktion/aktion/recommended:1`.

Presets are applied from left to right. Later presets override earlier ones,
then the repository's own `runner` block is applied. Finally, explicit
`repositories[].runnerImage` and `repositories[].runnerFeatures` values in
the host's `config.json` are administrator overrides and win last.

## Feature merge semantics

Feature objects use the Dev Container Features object model.

An object shallow-merges options:

```yaml
runner:
  features:
    ghcr.io/devcontainers/features/python:1:
      version: "3.13"
```

A string is shorthand for `version`:

```yaml
runner:
  features:
    ghcr.io/devcontainers/features/node:1: lts
```

`true` enables a Feature with inherited/default options. `false` removes an
inherited Feature:

```yaml
runner:
  features:
    ghcr.io/devcontainers/features/git-lfs:1: false
```

## OCI preset format

An Aktion preset is a real OCI 1.1 artifact, not a pseudo-URI.

Manifest artifact type:

```text
application/vnd.aktion.preset.v1
```

Preset YAML layer media type:

```text
application/vnd.aktion.preset.v1+yaml
```

The layer contains the same versioned Aktion YAML schema and may itself contain
`extends`.

Tags are convenient version aliases:

```text
ghcr.io/zyf722/aktion/recommended:1
```

For fully reproducible configuration, a digest can be used instead:

```text
ghcr.io/zyf722/aktion/recommended@sha256:...
```

The source of the current recommended preset lives at
`presets/recommended.yml`.

Using ORAS 1.3, the source can be published as:

```bash
oras push \
  --artifact-type application/vnd.aktion.preset.v1 \
  ghcr.io/zyf722/aktion/recommended:1 \
  presets/recommended.yml:application/vnd.aktion.preset.v1+yaml
```

## Resolution and security boundary

Aktion reads `.github/aktion.yml` from the repository's default branch using
the configured GitHub App. The App therefore needs:

- Repository administration: Read and write
- Contents: Read
- Metadata: Read (implicit)

Reading from the default branch is deliberate: a pull request must not be able
to increase runner privileges merely by modifying its own Aktion config before
the job starts.

Repositories without `.github/aktion.yml` retain the legacy host-config
behavior for compatibility. Once the project file exists, its base is explicit
and does not silently inherit Aktion's legacy built-in Feature defaults.

Use:

```text
aktion config resolve <repo-id>
```

to inspect the final effective base image, presets, and Feature options after
all inheritance and administrator overrides have been applied.
