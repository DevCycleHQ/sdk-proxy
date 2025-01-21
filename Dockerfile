FROM golang:1.21-alpine
LABEL org.opencontainers.image.source=https://github.com/devcyclehq/sdk-proxy
LABEL org.opencontainers.image.description="DevCycle SDK Proxy"
LABEL org.opencontainers.image.licenses=MIT

ENV CGOENABLED=0

WORKDIR /usr/src/app

COPY . .
RUN go mod download
RUN go build -v -o /usr/local/bin/devcycle-local-bucketing-proxy ./cmd

ENTRYPOINT ["/usr/local/bin/devcycle-local-bucketing-proxy"]