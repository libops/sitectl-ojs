# sitectl-ojs

`sitectl-ojs` simplifies the creation and operation of repositories created using the [LibOps OJS template](https://github.com/libops/ojs). It provides sitectl commands for OJS and PKP tools, recurring maintenance, validation, and health checks.

Documentation: https://sitectl.libops.io/plugins/ojs

## Requirements

- [`sitectl`](https://sitectl.libops.io/install) v1.7.0 or newer provides the RPC verifier SDK; promotion must pin the first core release that also includes `verify --strict` semantics.
- Docker with the Compose v2 plugin for local OJS sites.
- No additional app-plugin dependency beyond core `sitectl`.

## Quick Start

Create a local OJS site from the matching template:

```bash
sitectl create ojs/default \
  --template-repo https://github.com/libops/ojs \
  --path ./my-ojs-site \
  --type local \
  --checkout-source template \
  --default-context
```

The template README is at https://github.com/libops/ojs.

## Basic Operations

Use [`sitectl compose`](https://sitectl.libops.io/commands/compose) to start or inspect the stack:

```bash
sitectl compose up --remove-orphans -d
```

Use [`sitectl healthcheck`](https://sitectl.libops.io/commands/healthcheck) and [`sitectl validate`](https://sitectl.libops.io/commands/validate) to check the site:

```bash
sitectl healthcheck
sitectl validate
sitectl verify --strict
```

## Behavioral verification

`sitectl verify --strict` checks the running OJS code/database version pair, scoped MariaDB identity, rendered URL/OAI/scheduler/SMTP configuration, OAI-PMH `Identify` for the first enabled journal, and private/public storage access. The database probe reads the connection selected by the rendered `config.inc.php`; its password stays inside the container and is never copied into Docker process arguments or verifier output. A new installation with no enabled journal reports OAI as not yet applicable; once a journal is enabled, the same gate requires a valid OAI-PMH envelope.

Production verification is read-only. Disposable CI may add a reversible service-account storage write/read/delete probe:

```bash
sitectl verify --strict --disposable
```

Never use `--disposable` for a retained customer site. The local verifier does not replace hosted acceptance for public DNS/TLS, external ingress, real mail delivery, browser authentication, editorial publication, authorized submission download, or upgrade-from-prior-release fixtures.

Use [`sitectl image`](https://sitectl.libops.io/commands/image) for local image or build-arg overrides:

```bash
sitectl image set --tag ojs=3.5.0-5-php84
```

The plugin intentionally does not register broad development bind mounts: mounting an entire OJS plugin category would hide plugins bundled in the versioned base image. Add custom plugins through the downstream build or an explicit per-plugin override.

Use [`sitectl set`](https://sitectl.libops.io/commands/set) for component changes; it updates component-owned files immediately:

```bash
sitectl set ingress enabled --mode https-custom --domain ojs.localhost
sitectl set ingress enabled --trusted-ip 203.0.113.10/32 --max-upload-size 2G --upload-timeout 10m
```

See the [OJS plugin docs](https://sitectl.libops.io/plugins/ojs) for lifecycle operations, OJS tools, PKP tools, and recurring maintenance.

## License

`sitectl-ojs` is licensed under the MIT License.
