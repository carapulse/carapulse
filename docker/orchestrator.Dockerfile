FROM golang:1.26-alpine AS build

WORKDIR /src
RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/orchestrator ./cmd/orchestrator

FROM debian:bookworm-slim

ARG VAULT_VERSION=1.15.6
ARG BOUNDARY_VERSION=0.14.4

SHELL ["/bin/bash", "-euxo", "pipefail", "-c"]

RUN apt-get update && \
  apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    unzip \
    docker.io \
  && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL "https://releases.hashicorp.com/vault/${VAULT_VERSION}/vault_${VAULT_VERSION}_linux_amd64.zip" -o /tmp/vault.zip && \
  unzip -q /tmp/vault.zip -d /usr/local/bin && \
  chmod +x /usr/local/bin/vault && \
  rm -f /tmp/vault.zip

RUN curl -fsSL "https://releases.hashicorp.com/boundary/${BOUNDARY_VERSION}/boundary_${BOUNDARY_VERSION}_linux_amd64.zip" -o /tmp/boundary.zip && \
  unzip -q /tmp/boundary.zip -d /usr/local/bin && \
  chmod +x /usr/local/bin/boundary && \
  rm -f /tmp/boundary.zip

RUN useradd -m -u 10001 -s /usr/sbin/nologin app

COPY --from=build /out/orchestrator /usr/local/bin/orchestrator

USER 10001:10001
ENV HOME=/home/app

EXPOSE 7233
HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 CMD test -f /tmp/healthy || exit 1
ENTRYPOINT ["/usr/local/bin/orchestrator","-config","/etc/carapulse/config.json"]

