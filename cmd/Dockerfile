FROM golang:1.19-alpine
ADD .. /go/src/github.com/devcyclehq/local-bucketing-proxy
WORKDIR /go/src/github.com/devcyclehq/local-bucketing-proxy
RUN go install -tags native_bucketing
EXPOSE 8080
ENTRYPOINT ["/go/bin/local-bucketing-proxy"]