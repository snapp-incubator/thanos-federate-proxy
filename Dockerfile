#build stage
FROM golang:1.19.1-buster AS builder
WORKDIR /go/src/app

COPY go.sum go.mod /go/src/app/
RUN go mod download

COPY . /go/src/app
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o prom_query_federate

#final stage
FROM debian:buster-slim

ENV TZ=UTC \
    PATH="/app:${PATH}"

RUN DEBIAN_FRONTEND=noninteractive apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

WORKDIR /app

COPY --from=builder /go/src/app/prom_query_federate /app/prom_query_federate
ENTRYPOINT ["/app/prom_query_federate"]
LABEL Name=prom_query_federate Version=1.0.0

