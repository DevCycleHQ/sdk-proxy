FROM golang:1.19-alpine
RUN go install github.com/devcyclehq/local-bucketing-proxy/cmd -tags native_bucketing
EXPOSE 8080
ENTRYPOINT ["/go/bin/local-bucketing-proxy"]