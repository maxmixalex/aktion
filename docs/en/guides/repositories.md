# Add and manage repositories

Aktion is configured around repositories. Every `enabled: true` repository gets its own listener and independent runner capacity, so one host can serve several repositories without sharing repository-scoped state.

## Add a repository

Add an entry to the `repositories` array:

```json
{
  "id": "website",
  "url": "https://github.com/OWNER/website",
  "enabled": true
}
```

`id` is the short name used by Aktion commands such as `aktion logs website` and `aktion config resolve website`. Prefer letters, numbers, `-`, and `_`, and keep IDs unique.

`url` must have the form `https://github.com/OWNER/REPO`. The GitHub App must already be installed on that repository.

Validate the change before restarting:

```text
aktion doctor
aktion config resolve website
```

If listeners are already running:

```text
aktion restart
```

`restart` regenerates the Compose configuration, prepares any required runner image, and recreates the listener stack.

## Use another GitHub App installation

`app.installationId` is the default installation. If a repository belongs to another installation, override it:

```json
{
  "id": "team-repo",
  "url": "https://github.com/OTHER_OWNER/team-repo",
  "enabled": true,
  "installationId": 87654321
}
```

The same `app.clientId` and private key are still used; only the installation changes. `aktion doctor` verifies that the installation is reachable and actually contains the repository.

## Use another scale-set name

Repositories normally inherit `defaults.scaleSetName`, which defaults to `home-docker`. Scale sets are repository-scoped, so multiple repositories can use the same name.

To target a different label for one repository:

```json
{
  "id": "gpu-tools",
  "url": "https://github.com/OWNER/gpu-tools",
  "enabled": true,
  "scaleSetName": "build-host"
}
```

The workflow uses:

```yaml
runs-on: build-host
```

You can also set `labels` explicitly. When labels are omitted or empty, the listener uses the scale-set name as the label. Keeping them the same is usually easiest unless you have a specific reason to separate them.

## Disable a repository

Set:

```json
"enabled": false
```

and run `aktion restart`. Disabled entries do not participate in configuration resolution or listener startup. At least one repository must remain enabled or Aktion will reject the configuration.

If you are trying to recover from a GitHub-side scale set left behind by a crashed listener, use [Clean up a stale scale set](../operations.md#cleanup-stale-scale-set) instead of editing generated Compose files.
