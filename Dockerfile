FROM golang:1.23.4 AS builder
LABEL org.opencontainers.image.source=https://github.com/authzed/thanos-federate-proxy
WORKDIR /go/src/thanos-federate-proxy

# list the files needed to build, so we don't copy /seed folder
COPY ./* /go/src/thanos-federate-proxy

ENV CGO_ENABLED=0

RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg/mod go build ./

FROM cgr.dev/chainguard/static:latest
COPY --from=builder /go/src/thanos-federate-proxy/thanos-federate-proxy /usr/local/bin/
ENTRYPOINT ["thanos-federate-proxy"]