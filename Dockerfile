# syntax=docker/dockerfile:1

# --- Build stage -------------------------------------------------------------
FROM golang:1.26-alpine AS build

WORKDIR /src

# Cache module downloads separately from source.
COPY go.mod ./
RUN go mod download

COPY lickey.go ./

# Pure Go, static binary; trimmed and stripped to match the release build.
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/lickey .

# --- Final stage -------------------------------------------------------------
# Static binary with no network/TLS use, so scratch is enough.
FROM scratch

COPY --from=build /out/lickey /lickey

# Run as a non-root UID (numeric so it works without /etc/passwd).
USER 65534

ENTRYPOINT ["/lickey"]
