# docker-secrets-engine-shim

[![CI](https://github.com/chadsr/docker-secrets-engine-shim/actions/workflows/ci.yml/badge.svg)](https://github.com/chadsr/docker-secrets-engine-shim/actions/workflows/ci.yml)
[![Dependabot Updates](https://github.com/chadsr/docker-secrets-engine-shim/actions/workflows/dependabot/dependabot-updates/badge.svg)](https://github.com/chadsr/docker-secrets-engine-shim/actions/workflows/dependabot/dependabot-updates)

Docker's [secrets engine](https://github.com/docker/secrets-engine) resolves
`se://` references to real values when a container starts, keeping secret
literals out of Compose files, `.env` files, and shell history. On Linux it
only ships as a proprietary, prebuilt binary. The public repo contains just
the SDK.

This shim is a thin, from-source replacement built on that SDK. It behaves the
same from the outside, but stores secrets through the standard Docker
credential helpers you may already have. No proprietary binary and no separate
secret store are needed.

## What you get

- A per-user **daemon** that resolves `se://` secret references
- **`docker pass`**, a Docker CLI plugin that manages secrets (`set`, `get`,
  `ls`, `rm`, `run`)
- A **dockerd plugin** that injects resolved secrets into containers at
  creation, so `docker run -e TOKEN=se://mytoken` just works

## Requirements

- Go 1.25+
- Docker Engine 29.2+
- A credential helper from [docker-credential-helpers](https://github.com/docker/docker-credential-helpers)
  and a backend: `pass` (works headless) or `secretservice` (e.g. gnome-keyring)

## Build

```bash
make all
```

## Install

```bash
sudo install -Dm755 dist/docker-secrets-engine-shim /usr/bin/docker-secrets-engine-shim
sudo ln -sf docker-secrets-engine-shim /usr/bin/docker-pass
mkdir -p ~/.docker/cli-plugins
ln -sf /usr/bin/docker-secrets-engine-shim ~/.docker/cli-plugins/docker-pass

# the dockerd plugin is discovered at this path
sudo install -d /usr/libexec/docker/nri-plugins
sudo ln -sf /usr/bin/docker-secrets-engine-shim /usr/libexec/docker/nri-plugins/10-secrets-engine
```

## Enable se:// resolution in containers

```bash
sudo install -d /etc/docker/nri/conf.d
echo "uid: $(id -u)" | sudo tee /etc/docker/nri/conf.d/10-secrets-engine.conf
echo '{"nri-opts":{"enable":true}}' | sudo tee /etc/docker/daemon.json   # merge if one exists
sudo systemctl restart docker
```

## Usage

Start the daemon as your user:

```bash
docker-secrets-engine-shim
```

Or run it under systemd as a user service (unit in `packaging/systemd/user/`):

```bash
cp packaging/systemd/user/docker-secrets-engine.service ~/.config/systemd/user/
systemctl --user enable --now docker-secrets-engine.service
```

Then:

```bash
docker pass set mytoken=s3cr3t
docker run --rm -e TOKEN=se://mytoken alpine sh -c 'echo $TOKEN'   # s3cr3t
TOKEN=se://mytoken docker pass run -- env | grep TOKEN             # resolve without dockerd
docker mcp secret set apikey=sk-...                                # mcp-gateway interop
```

## Security

The daemon only accepts connections from your own user and root. Secrets share
the credential helper store with `docker login` registry credentials: only use
`se://` references you control.

## Differences from the official engine

- Built from source (public SDK) instead of a prebuilt binary
- Stores secrets via credential helpers instead of the engine's own keychain integration
- Single user per host (the official Docker CE support has the same limitation)
- No `docker-auth` or 1Password plugin

## License

MIT
