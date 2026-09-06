<p align="center">
  <img src="docs/en/assets/aktion.svg" alt="Aktion logo" width="112">
</p>

<h1 align="center">Aktion</h1>

<p align="center"><strong>Aktion is a lightweight GitHub Actions runner scale-set manager for Docker hosts.</strong></p>

Aktion runs repository-scoped GitHub Runner Scale Set listeners on a normal Docker host and creates an ephemeral Linux runner container for each assigned job.

- Docker host instead of a Kubernetes / ARC cluster
- scale-to-zero ephemeral runners
- GitHub App authentication
- multiple repositories from one host config
- host-controlled runner images, OCI presets, and Dev Container Features
- Windows, Linux, and macOS manager; Linux runners

## Documentation

English user documentation:

**https://zyf722.github.io/aktion/en/**

中文文档：

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
