# Aktion

**Aktion is a lightweight GitHub Actions runner scale-set manager for Docker hosts.**

Aktion lets a normal Docker host serve multiple GitHub repositories with runners created on demand. Each enabled repository gets its own GitHub Runner Scale Set listener. When GitHub assigns a job, Aktion creates a temporary Linux runner container and removes it after the job finishes. You keep GitHub Actions' normal workflow and queueing model without deploying Kubernetes or Actions Runner Controller just to get elastic runners.

Aktion is aimed at individuals and small teams that want to control their own runner host and software environment without keeping a fleet of permanent runners alive. The manager runs on Windows, Linux, and macOS; listeners and job runners currently run as Linux containers.

!!! note "This is not a clone of GitHub-hosted Ubuntu"
    The default runner starts from `ghcr.io/actions/actions-runner:latest` and adds Aktion's default Features. It does not automatically contain every tool preinstalled on GitHub-hosted `ubuntu-24.04`. Project-specific dependencies should still be installed by the workflow or explicitly added to the host-controlled runner environment.

## Start here

For a first deployment, work through these steps in order:

1. Prepare Docker Engine or Docker Desktop with Linux container support and Docker Compose v2.
2. Create and install a GitHub App with **Administration: Read and write** access to the target repository.
3. Put the App credentials and repository in `config.json`.
4. Run `aktion doctor` and fix configuration, Docker, or GitHub connectivity issues before starting listeners.
5. Run `aktion up`, then point the workflow's `runs-on` at the configured scale-set name.

[Run your first job →](getting-started.md)

## Common tasks

Once Aktion is running, most administration falls into four tasks: adding repositories, controlling per-repository concurrency, adjusting the runner environment, and checking runtime state.

- [Add or disable a repository](guides/repositories.md)
- [Set runner capacity and concurrency](guides/capacity.md)
- [Customize the runner environment](guides/runner-environment.md)
- [Use an HTTP/HTTPS proxy](guides/proxy.md)

Day-to-day commands are covered in [Operate Aktion](operations.md). Exact `config.json` fields and CLI commands live in the [configuration reference](reference/configuration.md) and [CLI reference](reference/cli.md).

## What Aktion manages

| Aktion manages | You still manage |
| --- | --- |
| One scale-set listener per repository | Availability and capacity of the Docker host |
| Creating and removing runners for assigned jobs | Project-specific dependency installation in workflows |
| Building the host-controlled runner image | Creating, installing, and protecting the GitHub App private key |
| Per-repository runner limits | Which repositories may use this host |
| Listener, runner, job, and Docker status | Appropriate isolation for untrusted workflows |

For the mental model behind repositories, listeners, and runners, see [How Aktion works](concepts/how-it-works.md).