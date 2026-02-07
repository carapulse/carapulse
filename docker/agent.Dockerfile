FROM golang:1.25-alpine AS build

WORKDIR /src
RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/agent ./cmd/agent

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
RUN adduser -D -u 10001 -s /sbin/nologin app

COPY --from=build /out/agent /usr/local/bin/agent

USER 10001:10001
ENV HOME=/home/app

EXPOSE 8090
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 CMD wget -qO- http://localhost:8090/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/agent","-config","/etc/carapulse/config.json"]

