FROM golang:1.19-alpine
RUN apk add --no-cache git
RUN go install github.com/devcyclehq/local-bucketing-proxy/cmd@8ba1f1c
EXPOSE 8080
ENTRYPOINT ["/go/bin/local-bucketing-proxy"]