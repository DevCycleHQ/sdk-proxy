# Build container
FROM --platform=${BUILDPLATFORM} golang:1.24 AS builder
LABEL org.opencontainers.image.source=https://github.com/devcyclehq/sdk-proxy
LABEL org.opencontainers.image.description="DevCycle SDK Proxy"
LABEL org.opencontainers.image.licenses=MIT

ARG TARGETARCH
ARG TARGETOS

ENV CGO_ENABLED=0

WORKDIR /usr/src/app

RUN --mount=type=bind \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOARCH="$TARGETARCH" GOOS="$TARGETOS" \
    go build -v -o /usr/local/bin/devcycle-sdk-proxy "./cmd"

# Runtime container
FROM gcr.io/distroless/static:latest

COPY --from=builder /usr/local/bin/devcycle-sdk-proxy /usr/local/bin/devcycle-sdk-proxy

ENTRYPOINT ["/usr/local/bin/devcycle-sdk-proxy"]
