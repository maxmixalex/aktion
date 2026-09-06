# Configure an HTTP/HTTPS proxy

If the Docker host must use a proxy to reach GitHub, an OCI registry, or other external services, configure it under `proxy` in `config.json`. Aktion passes these values to listeners and the runner containers they create.

## Basic configuration

```json
{
  "proxy": {
    "http": "http://host.docker.internal:3081",
    "https": "http://host.docker.internal:3081",
    "noProxy": "localhost,127.0.0.1"
  }
}
```

`http` and `https` must be complete `http://` or `https://` URLs. You may configure only one when that is all your environment needs.

!!! warning "Do not use localhost for a host-side proxy"
    Inside a container, `localhost` and `127.0.0.1` refer to that container, not the Docker host. If the proxy runs on the host, use `host.docker.internal`.

Aktion adds the `host.docker.internal:host-gateway` mapping for Linux Docker Engine, allowing the same hostname to work on Docker Desktop and ordinary Linux Docker hosts.

## Bypass the proxy for selected hosts

`noProxy` is passed as both `no_proxy` and `NO_PROXY` using the usual comma-separated form:

```json
{
  "proxy": {
    "http": "http://host.docker.internal:3081",
    "https": "http://host.docker.internal:3081",
    "noProxy": "localhost,127.0.0.1,.example.internal"
  }
}
```

Exact matching semantics are determined by the client consuming those variables. Add only hosts or domains that should genuinely connect directly.

## Proxy authentication

Credentials may be embedded in the URL:

```json
{
  "proxy": {
    "https": "http://user:password@host.docker.internal:3081"
  }
}
```

!!! warning "Credentials are stored in config.json"
    The current configuration format has no separate proxy secret field. Do not commit a `config.json` containing credentials or attach it to public logs or diagnostics.

`aktion status` redacts username and password when displaying proxy URLs, but the original configuration still contains them.

## Verify the change

After changing proxy settings:

```text
aktion doctor
aktion restart
```

For failures while building a Feature-based runner image, also inspect:

```text
aktion config resolve <repo-id>
aktion logs <repo-id>
```

If an error still attempts to reach `localhost`, the proxy address is usually not reachable from containers. See [Cannot reach GitHub or a registry through the proxy](../troubleshooting.md#proxy-connectivity).
