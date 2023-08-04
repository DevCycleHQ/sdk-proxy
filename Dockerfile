FROM golang:1.19-alpine
ENV CGOENABLED=0

WORKDIR /usr/src/app

# pre-copy/cache go.mod
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o /usr/local/bin/devcycle-local-bucketing-proxy ./cmd

ENTRYPOINT ["/usr/local/bin/devcycle-local-bucketing-proxy"]