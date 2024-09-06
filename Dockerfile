FROM golang:1.20-alpine
ENV CGOENABLED=0

WORKDIR /usr/src/app

COPY . .
RUN go mod download
RUN go build -v -o /usr/local/bin/devcycle-local-bucketing-proxy ./cmd

ENTRYPOINT ["/usr/local/bin/devcycle-local-bucketing-proxy"]