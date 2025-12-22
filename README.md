# barge [![CI](https://github.com/frantjc/barge/actions/workflows/ci.yml/badge.svg?branch=main&event=push)](https://github.com/frantjc/barge/actions) [![godoc](https://pkg.go.dev/badge/github.com/frantjc/barge.svg)](https://pkg.go.dev/github.com/frantjc/barge) [![goreportcard](https://goreportcard.com/badge/github.com/frantjc/barge)](https://goreportcard.com/report/github.com/frantjc/barge)

Barge copies Helm Charts between archives, releases, repositories, OCI registries, source code, and more.

## use cases

- You want to move your Charts from an old HTTP(S) Helm repository to a new OCI Helm registry.

```sh
barge cp https://example.com/charts/example-1.0.0.tgz oci://ghcr.io/frantjc/charts/example:1.0.0
```

- You need to copy OSS Helm Charts into an internal registry or repository.

```sh
helm repo add chartmuseum https://chartmuseum.github.io/charts
barge cp repo://chartmuseum/chartmuseum artifactory://example.com/artifactory/helm-local
```

- You want to save a one-off Helm release for re-use later.

```sh
mkdir ./example
barge cp release://example ./example
```
