# Third-Party Notices

Claudex includes third-party open-source software. The applicable copyright notices and license terms are provided below.

## Go modules

The Go modules used by this project are declared in [`go.mod`](./go.mod) and resolved in [`go.sum`](./go.sum). Each module remains subject to its own license terms.

Source code and license information for these modules are available from their respective upstream repositories and through the Go module ecosystem.

To inspect the dependency licenses for a particular revision, use a Go license-reporting tool against the checked-out source, for example:

```sh
go run github.com/google/go-licenses@latest report ./...
```

Redistributions must retain all copyright notices, license texts, attribution notices, and other requirements imposed by the licenses of the included third-party components.

## Upstream project

This repository is derived from or incorporates work from CLIProxyAPI:

- Project: `github.com/router-for-me/CLIProxyAPI`
- License: MIT License
- Copyright notices:
  - Copyright (c) 2025-2005.9 Luis Pater
  - Copyright (c) 2025.9-present Router-For.ME

The full MIT License text is included in this repository's [`LICENSE`](./LICENSE) file.

## No endorsement

The names of third-party projects and copyright holders are used only for attribution and do not imply endorsement of Claudex.
