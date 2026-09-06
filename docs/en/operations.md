# Operate Aktion

Routine Aktion administration does not require editing generated Compose files. Keep desired state in `config.json` and use the `aktion` command for startup, status, logs, and recovery.

## Start, stop, and restart

```text
aktion up
aktion down
aktion restart
```

`up` resolves configuration and runner Features, builds local runner images when needed, then starts listeners for every enabled repository.

`down` stops and removes the listener stack. It does not delete `config.json`, the private key, or built Docker images.

`restart` regenerates configuration, stops the current listener stack, prepares runner images, and starts it again. Use it after changing repositories, capacity, Features, proxy settings, or other runtime configuration.

## Check overall status

```text
aktion status
```

Running `aktion` with no command does the same thing. Status includes:

- healthy listener count;
- current ephemeral runners;
- active jobs;
- jobs assigned by GitHub but still waiting for a runner;
- listener and runner state per repository.

If a listener is not `RUNNING`, the status view tries to surface a useful recent error. Use logs when you need full context.

## Inspect repositories, runners, and resources

```text
aktion repos
aktion runners
aktion stats
```

`repos` is useful for confirming which repositories are actually enabled and their resolved capacity. Entries with `enabled: false` do not participate in runtime configuration.

`runners` lists currently visible ephemeral runner containers and, when the listener logs provide enough information, the workflow job each runner is executing.

`stats` is a one-shot Docker resource snapshot for Aktion listeners and runners, including CPU, memory, and network I/O. It is not a monitoring system, but it is useful when deciding whether `maxRunners` is too high.

## Follow logs

All listeners:

```text
aktion logs
```

One repository:

```text
aktion logs my-repo
```

The command follows output after showing the last 200 lines. Exiting log follow in the normal terminal way (usually Ctrl+C) does not stop the listener.

## Inspect GitHub-side scale sets

```text
aktion scalesets
```

This checks the GitHub Actions service directly for each enabled repository. A healthy running repository should normally show `FOUND`.

If a listener exited without removing its remote scale set, the next startup can fail with `RunnerScaleSetExistsException`. Use cleanup rather than trying to edit generated state.

## Clean up a stale scale set { #cleanup-stale-scale-set }

First make sure no important job is running:

```text
aktion cleanup
```

Aktion stops local listeners, deletes the configured repositories' remote scale sets, and starts the listeners again after successful cleanup.

By default, cleanup refuses to proceed while an ephemeral runner is active.

!!! danger "cleanup --force can interrupt active jobs"
    Only use:

    ```text
    aktion cleanup --force
    ```

    when those jobs may safely be terminated.

To remove remote state but leave listeners stopped:

```text
aktion cleanup --no-start
```

If cleanup fails for one or more repositories, Aktion leaves listeners stopped rather than entering a restart loop. Fix the GitHub App, permission, or network error shown, then retry cleanup or run `aktion up`.

## Inspect effective configuration before applying it

When inheritance, presets, or Features are involved:

```text
aktion config resolve my-repo
```

To generate Compose and ask Docker Compose to validate it:

```text
aktion config
```

To only generate `compose.generated.yaml`:

```text
aktion render
```

`compose.generated.yaml` is generated state. Do not treat manual edits as durable configuration; render, up, or restart may overwrite it.
