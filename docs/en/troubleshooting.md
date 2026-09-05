# Troubleshooting

Start with:

```text
aktion doctor
```

It groups problems across local configuration, Docker, GitHub App authentication, and repository installations. The sections below start from symptoms you can observe.

## `doctor` says Docker Engine is unavailable

**Symptom:** `aktion doctor` or `aktion status` reports that Docker is unavailable.

Verify Docker directly:

```text
docker version
docker compose version
```

If either fails, start Docker Engine or Docker Desktop. On Windows and macOS, make sure Docker can run Linux containers; Aktion listeners and runners are not Windows containers.

If `docker version` succeeds but `docker compose version` fails, install or enable Docker Compose v2.

## `doctor` reports a private-key or GitHub App identity failure

**Symptom:** the Local or GitHub section reports a failed private key, JWT, or App identity check.

Make sure `app.clientId` belongs to the same GitHub App that generated the PEM and that `app.privateKeyFile` exists and is readable. Relative paths are resolved from the configuration root selected by `--root`.

If you rotated the App key, point `privateKeyFile` to the current PEM. Do not paste the private key into JSON.

## Installation permission check fails

**Symptom:** the installation is reachable, but `doctor` reports `administration` is not `write`.

Set the GitHub App repository **Administration** permission to **Read and write**, then make sure the installation has accepted the updated permission. Run:

```text
aktion doctor
```

again. A PAT cannot be used to bypass this requirement.

## Repository says the App is not installed or the installation does not match

**Symptom:** the Repositories section says the App is not installed, the repository is inaccessible, or the actual installation ID differs from configuration.

Check the GitHub App installation and make sure the repository is selected. If repositories belong to different installations, override the affected entry:

```json
"installationId": 87654321
```

Then rerun `aktion doctor`.

## Listener keeps restarting or shows `RunnerScaleSetExistsException`

**Symptom:** `aktion status` reports `RESTARTING` or `EXITED`, and the recent error contains `RunnerScaleSetExistsException`.

First check that no important job is still running:

```text
aktion runners
aktion scalesets
```

Then run:

```text
aktion cleanup
```

Cleanup refuses to run while ephemeral runners are active unless forced. Use `aktion cleanup --force` only when interrupting those jobs is acceptable.

## Workflow keeps waiting for a runner

**Symptom:** a GitHub Actions job remains queued or says it is waiting for a runner.

Check:

```text
aktion status
aktion scalesets
aktion logs <repo-id>
```

The listener should be `RUNNING`, the remote scale set should be `FOUND`, and the workflow's `runs-on` must match the effective label. With no explicit `labels`, the label is the `scaleSetName`, which defaults to `home-docker`.

If `status` reports pending work while the repository has already reached `maxRunners`, the capacity limit is working as configured. Raise that repository's limit or wait for existing jobs:

```text
aktion config resolve <repo-id>
```

`maxRunners: 0` means unlimited, but it should not be used to hide insufficient host capacity.

## Runner image or Feature build fails

**Symptom:** `aktion up` or `restart` fails while resolving runner Features or building the runner image.

Inspect the effective configuration:

```text
aktion config resolve <repo-id>
aktion features
```

Check Feature URIs and options. Public OCI Features can use anonymous or Bearer-token registry flows; private registry credentials are not configurable, so a registry requiring private authentication will fail.

A warning about Dev Container lifecycle hooks is an expected limitation: Aktion does not run hooks such as `postCreateCommand`. Put required project initialization in the GitHub Actions workflow instead.

## Configuration changed but runtime behavior did not

**Symptom:** `config.json` changed but listeners still use the old capacity, Features, or proxy.

Configuration is not hot-reloaded. Run:

```text
aktion config resolve <repo-id>
aktion restart
```

The first command verifies the new effective configuration; the second recreates the listener stack.

Do not rely on manual edits to `compose.generated.yaml`; Aktion regenerates it.

## Cannot reach GitHub or a registry through the proxy

**Symptom:** a host-side proxy works locally, but listeners or runners report connection refused or timeouts.

A proxy on the Docker host must not be configured as:

```text
http://localhost:3081
```

Use:

```text
http://host.docker.internal:3081
```

Aktion adds the corresponding host-gateway mapping for Linux Docker Engine. Restart after changing the configuration.

## Runner performance collapses or the host runs out of memory

**Symptom:** concurrent jobs saturate CPU, memory approaches exhaustion, or Docker becomes unresponsive.

Take a resource snapshot:

```text
aktion stats
```

Then reduce the affected repository's `maxRunners`. Repository limits are independent: three repositories each configured for four runners can produce twelve runners at once.

## Collect more useful diagnostics

Follow all listeners:

```text
aktion logs
```

or one repository:

```text
aktion logs <repo-id>
```

When sharing diagnostics, prefer the failing `aktion doctor` checks, the relevant log excerpt, and `aktion config resolve <repo-id>` output. Never include the GitHub App private key or an unredacted proxy password.
