# Build Process

This repository contains two independent Go programs, each its own module:

| Component        | Path               | Module                                  | Output binary    |
| ---------------- | ------------------ | --------------------------------------- | ---------------- |
| Main application | `./` (repo root)   | `github.com/ragnoaraknos/lickey`        | `lickey`         |
| Go client example| `examples/go/`     | `github.com/ragnoaraknos/lickeyClient`  | `lickeyClient`   |

Because they are separate modules, each is built from its own directory — there
is no single `go build ./...` that covers both.

## Building locally

Requires the Go toolchain matching `go.mod` (Go 1.26.3).

```sh
# Main application
go build -o lickey .

# Go client example
cd examples/go
go build -o lickeyClient .
```

### Cross-compiling

Both programs are pure Go with no cgo dependencies, so they cross-compile to any
platform by setting `GOOS`/`GOARCH`:

```sh
GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -o lickey-linux-amd64   .
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o lickey-windows-amd64.exe .
GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -o lickey-darwin-arm64  .
```

## Continuous integration

Two workflows live in [`.github/workflows/`](../.github/workflows/).

### `build.yml` — CI build

- **Triggers:** push and pull requests to `main`, plus manual `workflow_dispatch`.
- **What it does:** cross-compiles both binaries for Linux, Windows, and macOS
  (amd64) on a single `ubuntu-latest` runner via a build matrix, runs `go vet`
  on both modules, and uploads the resulting binaries as workflow artifacts.
- **Purpose:** verify every change compiles and passes vet on all target
  platforms before it is merged.

Artifacts are named `lickey-<os>-amd64` and can be downloaded from the run's
summary page.

### `release.yml` — Tagged releases

- **Triggers:** pushing a tag matching `v*` (e.g. `v1.0.0`), or manual
  `workflow_dispatch` with a `tag` input.
- **What it does:** builds optimized binaries (`-trimpath -ldflags "-s -w"`) for
  Linux (amd64/arm64), Windows (amd64), and macOS (amd64/arm64), bundles each
  platform's binaries together with `LICENSE` and `README.md` into a
  `lickey-<tag>-<os>-<arch>.zip` archive, generates SHA-256 checksums, and
  attaches everything to a GitHub Release with auto-generated release notes.
- **Permissions:** uses `contents: write` to create/update the release.

Each release lists, per platform:

- `lickey-<tag>-<os>-<arch>.zip` — contains `lickey`, `lickeyClient`, `LICENSE`,
  and `README.md`.
- `checksums-<os>-<arch>.txt` — SHA-256 checksum of the archive.

#### Cutting a release

```sh
git tag v1.0.0
git push origin v1.0.0
```

The workflow then builds all platform binaries and publishes the release
automatically. To re-run for an existing tag, use the **Run workflow** button on
the Release workflow and supply the tag name.

## Build flags reference

| Flag                  | Used in   | Purpose                                            |
| --------------------- | --------- | -------------------------------------------------- |
| `CGO_ENABLED=0`       | both      | Static, dependency-free binaries; reliable cross-compiling. |
| `-trimpath`           | release   | Removes local filesystem paths for reproducible builds. |
| `-ldflags "-s -w"`    | release   | Strips the symbol table and DWARF info to shrink binaries. |
| `-v`                  | both      | Prints package names as they are compiled.         |
