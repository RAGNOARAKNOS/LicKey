# Running LicKey with Docker

LicKey ships as a tiny container image so you can generate license keys without
installing the Go toolchain. The application is a one-shot CLI: the container
runs the binary, prints a key (or a fresh keypair), and exits — it is not a
long-running service.

The image is built from a [`Dockerfile`](../Dockerfile) at the repo root using a
two-stage build (compile in `golang:1.26-alpine`, copy the static binary into
`scratch`). The result is a ~3 MB image with no shell or OS layer, running as a
non-root user.

## Pulling the published image

Tagged releases publish a multi-arch image (`linux/amd64`, `linux/arm64`) to the
GitHub Container Registry via the [`docker.yml`](../.github/workflows/docker.yml)
workflow:

```sh
docker pull ghcr.io/ragnoaraknos/lickey:latest
```

Available tags: `latest`, the full version (`1.0.0`), and the major.minor
(`1.0`).

## Building locally

```sh
docker build -t lickey:local .
```

## Usage

The image's entrypoint is the `lickey` binary, so any
[arguments](../README.md#usage) are appended to `docker run`. The private key is
supplied at runtime through the `lickey_privatekey` environment variable.

### Generate a keypair

With no key set, LicKey generates a fresh Ed25519 keypair, prints it, and exits:

```sh
docker run --rm ghcr.io/ragnoaraknos/lickey:latest \
  -user "DonKey@example.com" -sku 101 -features 3 -expiry "2026-12-25"
```

Store the printed private key securely and distribute the public key to your
client applications.

### Generate a license key

Pass the private key with `-e` and supply the license arguments:

```sh
docker run --rm \
  -e lickey_privatekey="<your-base64-private-key>" \
  ghcr.io/ragnoaraknos/lickey:latest \
  -user "DonKey@example.com" -sku 101 -features 3 -expiry "2026-12-25"
```

The generated key is printed to stdout, so you can capture it directly:

```sh
KEY=$(docker run --rm -e lickey_privatekey="$LICKEY_PRIVATEKEY" \
  ghcr.io/ragnoaraknos/lickey:latest \
  -user "DonKey@example.com" -sku 101 -expiry "2026-12-25")
```

## Security notes

> [!WARNING]
> **Never bake the private key into the image.** The signing key is the entire
> security model of LicKey, and anyone with the image layers can extract a
> baked-in value. Always pass `lickey_privatekey` at runtime via `-e` (or a
> secret manager / `--env-file`), and avoid committing it to shell history —
> prefer reading it from an environment variable as shown above.

## Publishing manually

The CI workflow publishes on every `v*` tag, but you can also push by hand:

```sh
echo "$CR_PAT" | docker login ghcr.io -u ragnoaraknos --password-stdin

docker build \
  -t ghcr.io/ragnoaraknos/lickey:1.0.0 \
  -t ghcr.io/ragnoaraknos/lickey:latest .

docker push ghcr.io/ragnoaraknos/lickey:1.0.0
docker push ghcr.io/ragnoaraknos/lickey:latest
```

> [!NOTE]
> The first publish creates the GHCR package as **private**. To allow public
> pulls, change the package visibility in its GitHub package settings.
