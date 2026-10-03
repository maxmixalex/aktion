# CLI reference

## Syntax

```text
aktion [--root DIR] [--config FILE] [--color MODE] [--links MODE] <command> [args]
```

With no command, Aktion runs `status`.

## Global options

| Option | Default | Meaning |
| --- | --- | --- |
| `--root DIR` | Executable directory | Configuration root (symbolic links are resolved). Explicit relative paths, including `--root .`, are resolved from the working directory. Relative private-key paths and generated Compose state use this root. |
| `--config FILE` | `config.json` | Configuration filename. |
| `--color auto\|always\|never` | `auto` | ANSI color output. |
| `--links auto\|always\|never` | `auto` | OSC 8 terminal hyperlinks. |

`NO_COLOR=1` disables color in `auto` mode. `NO_HYPERLINK=1` disables automatically generated terminal links.

## Commands

### `status`

```text
aktion status
```

Shows listener health, ephemeral runners, active jobs, assigned jobs still waiting, and repository summaries. This is also the default command.

### `up`

```text
aktion up
```

Resolves configuration, generates `compose.generated.yaml`, prepares Feature-based runner images, and starts listeners for all enabled repositories with Docker Compose.

### `down`

```text
aktion down
```

Stops and removes the listener stack without deleting configuration, the private key, or built images.

### `restart`

```text
aktion restart
```

Regenerates configuration and builds runner and listener images before stopping listeners, then starts listeners using the built images. Build failures leave existing listeners running. Use after runtime configuration changes.

### `doctor`

```text
aktion doctor
```

Checks configuration, private key, Docker Engine, Docker Compose, existing listener state, GitHub App identity, installation Administration write permission, and repository-to-installation matching.

### `repos`

```text
aktion repos
```

Shows enabled repositories and their resolved runtime configuration summary.

### `runners`

```text
aktion runners
```

Shows current Aktion runner containers and, when available from listener logs, the workflow job each runner is executing.

### `stats`

```text
aktion stats
```

Shows a one-shot Docker CPU, memory, and network I/O snapshot for listener and runner containers.

### `logs [repo-id]`

```text
aktion logs
aktion logs my-repo
```

Follows all listener logs or only one repository. It starts with the most recent 200 lines.

### `scalesets`

```text
aktion scalesets
```

Queries the GitHub Actions service and reports whether each enabled repository's scale set exists, including its ID and runner group.

### `cleanup`

```text
aktion cleanup [--force|-f] [--no-start]
```

Stops local listeners, deletes remote scale sets for configured repositories, and normally restarts listeners after successful cleanup.

| Option | Effect |
| --- | --- |
| `--force`, `-f` | Continue even when active ephemeral runners are detected. |
| `--no-start` | Leave listeners stopped after cleanup. |

!!! danger
    `--force` can interrupt running workflow jobs. Normally wait for runners to exit first.

### `features`

```text
aktion features
```

Lists Aktion builtins, default OCI Dev Container Features, and builtin dependencies. It does not need to load `config.json`.

### `config`

```text
aktion config
```

Generates `compose.generated.yaml`, verifies Docker is available, and runs Docker Compose configuration validation.

### `config resolve <repo-id>`

```text
aktion config resolve my-repo
```

Prints JSON containing an enabled repository's final `minRunners`, `maxRunners`, `baseImage`, local `image`, and `features`. Prefer this when diagnosing inheritance or Feature configuration.

### `render`

```text
aktion render
```

Generates `compose.generated.yaml` without Docker Compose validation.

### `version`

```text
aktion version
```

Shows the Aktion version and manager OS/architecture.

### `help`

```text
aktion help
```

Shows built-in command help.
