FROM golang:1.19-alpine
ENV CGOENABLED=0
RUN apk add --no-cache git
RUN go install github.com/devcyclehq/local-bucketing-proxy/cmd@latest
ENTRYPOINT ["/go/bin/cmd"]