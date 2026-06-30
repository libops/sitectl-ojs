# sitectl-ojs

`sitectl-ojs` simplifies the creation and operation of repositories created using the [LibOps OJS template](https://github.com/libops/ojs). It provides sitectl commands for OJS and PKP tools, recurring maintenance, validation, and health checks.

Documentation: https://sitectl.libops.io/plugins/ojs

## Requirements

- [`sitectl`](https://sitectl.libops.io/install).
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
```

Use [`sitectl image`](https://sitectl.libops.io/commands/image) for local image or build-arg overrides:

```bash
sitectl image set --tag ojs=nginx-1.30.3-php84
```

Use [`sitectl set`](https://sitectl.libops.io/commands/set) and [`sitectl converge`](https://sitectl.libops.io/commands/converge) for component changes:

```bash
sitectl set ingress enabled --mode https-default --domain ojs.localhost
sitectl set ingress enabled --trusted-ip 203.0.113.10/32 --max-upload-size 2G --upload-timeout 10m
sitectl converge
```

See the [OJS plugin docs](https://sitectl.libops.io/plugins/ojs) for lifecycle operations, OJS tools, PKP tools, and recurring maintenance.

## License

`sitectl-ojs` is licensed under the MIT License.
