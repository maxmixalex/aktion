# Quick start

This guide starts with a Docker host where Aktion is not running and ends with a GitHub Actions job executing on an Aktion runner. At the end, `aktion status` should show a healthy listener and GitHub Actions should dispatch a job to `home-docker`.

## Prepare the runtime directory

The host needs [Docker Engine](https://docs.docker.com/engine/) or [Docker Desktop](https://docs.docker.com/desktop/) capable of running **Linux containers**, plus [Docker Compose v2](https://docs.docker.com/compose/) through the `docker compose` command. The manager supports Windows, Linux, and macOS, while the listener and runner are currently Linux containers.

A runnable Aktion directory contains at least:

```text
aktion.exe                 # Windows; usually named aktion on Linux/macOS
config.json
Dockerfile
packaging/
  start-listener.sh
dist/
  aktion-listener-linux-amd64   # or linux-arm64, matching the Docker host
```

The GitHub App PEM should not be included in a distribution archive or committed to source control. The examples below place it at `secrets/github-app.pem`.

!!! warning "Protect the private key"
    The PEM referenced by `privateKeyFile` can authenticate as your GitHub App. Do not commit it or share a runtime directory that contains it.

Check Docker first:

```text
docker version
docker compose version
```

Both commands should succeed. If Docker Desktop is using Windows containers, switch it to Linux containers first.

## Create a GitHub App

Aktion uses [GitHub App](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/registering-a-github-app) authentication only; there is no PAT mode.

1. Create a GitHub App and set the repository **Administration** permission to **Read and write**. GitHub's [permissions guide](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/choosing-permissions-for-a-github-app) explains how App permissions work.
2. Aktion does not need webhooks. If this App exists only for Aktion, webhooks can be disabled.
3. Install the App on the repository that should use this runner host.
4. Record the App **Client ID** and the numeric **Installation ID**.
5. Generate a private key and save the PEM on the Aktion host.

Repositories in the same App installation can share `app.installationId`. A repository in another installation can override that ID in its own entry.

## Create the minimal configuration

Create `config.json`:

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
    "logLevel": "info"
  },
  "repositories": [
    {
      "id": "my-repo",
      "url": "https://github.com/OWNER/REPO",
      "enabled": true
    }
  ]
}
```

Place the PEM at `./secrets/github-app.pem`. Relative paths are resolved from Aktion's configuration root, which defaults to the directory containing the Aktion executable (after resolving symbolic links). You can add that directory to `PATH` and run Aktion from any working directory. Use `--root DIR` for a different installation, or `--root .` to use the current working directory.

`minRunners: 0` lets the repository scale to zero when there are no jobs. `maxRunners: 1` allows one runner for this repository at a time. The limit is **per repository**, not a host-wide cap.

## Validate before starting

=== ":fontawesome-brands-windows: Windows"

    ```powershell
    .\aktion.exe doctor
    ```

=== ":fontawesome-brands-apple: macOS / :fontawesome-brands-linux: Linux"

    ```bash
    ./aktion doctor
    ```

`doctor` checks the JSON configuration, private key, Docker Engine, Docker Compose, GitHub App identity, installation permissions, and whether the App is actually installed on every enabled repository.

Fix failed checks first. A successful run ends with **All systems operational**.

## Start the listener

=== ":fontawesome-brands-windows: Windows"

    ```powershell
    .\aktion.exe up
    .\aktion.exe status
    ```

=== ":fontawesome-brands-apple: macOS / :fontawesome-brands-linux: Linux"

    ```bash
    ./aktion up
    ./aktion status
    ```

The first `up` may pull the base image, resolve Dev Container Features, and build a local runner image. Aktion then starts one listener container for each enabled repository.

In `status`, the configured repository should show a `RUNNING` listener. Seeing zero runners before a job is assigned is normal.

## Send a workflow to Aktion

The default scale-set name is `home-docker`. In the configured repository, create or update a workflow:

```yaml
name: Aktion smoke test

on:
  workflow_dispatch:

jobs:
  test:
    runs-on: home-docker
    steps:
      - uses: actions/checkout@v7
      - run: uname -a
```

Trigger the workflow manually, then run:

```text
aktion status
```

You should briefly see an ephemeral runner and an active job. After the job finishes, the runner container is removed. If the job remains queued waiting for a runner, see [Workflow keeps waiting for a runner](troubleshooting.md#workflow-waiting-for-runner).

## Next steps

After one repository works, you can extend the same setup without learning internal implementation details:

- [Add more repositories](guides/repositories.md)
- [Set different concurrency limits per repository](guides/capacity.md)
- [Add Node, Go, Rust, or other runner Features](guides/runner-environment.md)
- [Learn routine status, logs, restart, and cleanup commands](operations.md)
