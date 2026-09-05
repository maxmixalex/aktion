# Set runner capacity and concurrency

Aktion capacity limits are **per repository**. This distinction matters: `defaults.maxRunners` is the value each repository inherits, not a global concurrency cap for the Docker host.

## Scale to zero by default

A practical starting point is:

```json
"defaults": {
  "minRunners": 0,
  "maxRunners": 1
}
```

With `minRunners: 0`, a repository can have zero runners when there is no work; only its lightweight listener remains. The listener creates runners after GitHub assigns jobs.

For a repository where startup latency matters, keep some capacity ready:

```json
{
  "id": "latency-sensitive",
  "url": "https://github.com/OWNER/latency-sensitive",
  "enabled": true,
  "minRunners": 1,
  "maxRunners": 3
}
```

Neither value may be negative. When `maxRunners` is greater than zero, `minRunners` may not exceed it.

## Raise the limit for one repository

Suppose the default is:

```json
"defaults": {
  "maxRunners": 2
}
```

If two repositories inherit that value, each may run up to two runners, so the host can theoretically have four runners at once. Aktion currently has no additional host-wide “maximum N runners” limit.

Override a busier repository directly:

```json
{
  "id": "hpclang",
  "url": "https://github.com/OWNER/hpclang",
  "enabled": true,
  "maxRunners": 5
}
```

Confirm the effective value instead of guessing through inheritance:

```text
aktion config resolve hpclang
```

The returned `minRunners` and `maxRunners` are the values the listener will use.

## Remove the limit

In Aktion, `maxRunners: 0` means unlimited:

```json
{
  "id": "unlimited",
  "url": "https://github.com/OWNER/unlimited",
  "enabled": true,
  "maxRunners": 0
}
```

!!! warning "Unlimited does not mean unlimited host resources"
    Aktion does not impose a CPU, memory, or disk guard when `maxRunners: 0` is used. A burst of jobs can exhaust the Docker host. Use unlimited only when an external limit exists or you knowingly accept that risk.

During operation, `aktion stats` shows a one-shot CPU/memory snapshot for listener and runner containers, while `aktion status` shows runners, active jobs, and assigned jobs still waiting.

## Choose a practical limit

For compilation and test workloads, begin with the number of simultaneous jobs the host can sustain rather than mechanically mapping CPU thread count to runners. Memory and I/O requirements can vary substantially by workflow. Start conservatively, then adjust from real build times and `aktion stats`.
