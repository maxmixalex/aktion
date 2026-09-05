# Aktion

**Self-hosted GitHub Actions without Kubernetes.**

Aktion runs repository-scoped GitHub Runner Scale Set listeners on a normal Docker host and creates an ephemeral Linux runner container for each assigned job.

- Docker host instead of a Kubernetes / ARC cluster
- scale-to-zero ephemeral runners
- GitHub App authentication
- multiple repositories from one host config
- host-controlled runner images, OCI presets, and Dev Container Features
- Windows, Linux, and macOS manager; Linux runners

## Documentation

User documentation is published at:

**https://zyf722.github.io/aktion/**

Chinese project overview: [README.zh-CN.md](README.zh-CN.md)

## Quick start

Put the GitHub App PEM where `config.json` points, then:

```text
aktion doctor
aktion up
aktion status
```

Workflows target the configured scale-set name, which defaults to:

```yaml
runs-on: home-docker
```

## Development

```bash
go test ./...
go vet ./...
bash ./scripts/build-artifacts.sh dist
```

Aktion uses GitHub's standalone `actions/scaleset` client. The project is still early-stage and its interfaces may evolve.
