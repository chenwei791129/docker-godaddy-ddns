# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.27.2-alpine AS build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" \
      -o /out/godaddy-ddns ./cmd/godaddy-ddns

# Alpine rather than scratch: the Success Hook and Failure Hook run shell
# commands (docs/adr/0001-alpine-base-for-update-hooks.md). Its CA bundle covers
# the HTTPS calls to GoDaddy and the IP check service.
FROM alpine:3.24.3

# Changes on every CI build (see .github/workflows/release-please.yml) so this
# layer is never reused from the build cache and gets Alpine's latest fixes.
ARG APK_REFRESH
# curl: the original image shipped it and hooks commonly call it.
# tini: PID 1 that reaps processes hooks leave behind.
RUN apk upgrade --no-cache && apk add --no-cache curl tini

COPY --from=build /out/godaddy-ddns /usr/local/bin/godaddy-ddns

ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/godaddy-ddns"]
