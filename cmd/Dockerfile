FROM golang:1.19-alpine
RUN apk add --no-cache git
RUN go install github.com/devcyclehq/local-bucketing-proxy/cmd@latest
EXPOSE 8080
ENTRYPOINT ["/go/bin/local-bucketing-proxy"]