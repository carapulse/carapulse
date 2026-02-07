FROM golang:1.23-alpine AS build

WORKDIR /src
RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/tool-router ./cmd/tool-router

FROM debian:bookworm-slim

ARG KUBECTL_VERSION=v1.29.0
ARG HELM_VERSION=v3.14.0
ARG ARGOCD_VERSION=v2.10.0
ARG VAULT_VERSION=1.15.6
ARG BOUNDARY_VERSION=0.14.4
ARG GH_VERSION=2.45.0
ARG GLAB_VERSION=1.46.0
ARG AWSCLI_VERSION=2.13.0

SHELL ["/bin/bash", "-euxo", "pipefail", "-c"]

RUN apt-get update && \
  apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    jq \
    git \
    unzip \
    tar \
    gnupg \
    openssh-client \
    docker.io \
  && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" -o /usr/local/bin/kubectl && \
  chmod +x /usr/local/bin/kubectl

RUN curl -fsSL "https://get.helm.sh/helm-${HELM_VERSION}-linux-amd64.tar.gz" -o /tmp/helm.tgz && \
  tar -C /tmp -xzf /tmp/helm.tgz && \
  mv /tmp/linux-amd64/helm /usr/local/bin/helm && \
  chmod +x /usr/local/bin/helm && \
  rm -rf /tmp/helm.tgz /tmp/linux-amd64

RUN curl -fsSL "https://github.com/argoproj/argo-cd/releases/download/${ARGOCD_VERSION}/argocd-linux-amd64" -o /usr/local/bin/argocd && \
  chmod +x /usr/local/bin/argocd

RUN curl -fsSL "https://awscli.amazonaws.com/awscli-exe-linux-x86_64-${AWSCLI_VERSION}.zip" -o /tmp/awscliv2.zip && \
  unzip -q /tmp/awscliv2.zip -d /tmp && \
  /tmp/aws/install && \
  rm -rf /tmp/aws /tmp/awscliv2.zip

RUN curl -fsSL "https://releases.hashicorp.com/vault/${VAULT_VERSION}/vault_${VAULT_VERSION}_linux_amd64.zip" -o /tmp/vault.zip && \
  unzip -q /tmp/vault.zip -d /usr/local/bin && \
  chmod +x /usr/local/bin/vault && \
  rm -f /tmp/vault.zip

RUN curl -fsSL "https://releases.hashicorp.com/boundary/${BOUNDARY_VERSION}/boundary_${BOUNDARY_VERSION}_linux_amd64.zip" -o /tmp/boundary.zip && \
  unzip -q /tmp/boundary.zip -d /usr/local/bin && \
  chmod +x /usr/local/bin/boundary && \
  rm -f /tmp/boundary.zip

RUN curl -fsSL "https://github.com/cli/cli/releases/download/v${GH_VERSION}/gh_${GH_VERSION}_linux_amd64.tar.gz" -o /tmp/gh.tgz && \
  tar -C /tmp -xzf /tmp/gh.tgz && \
  mv "/tmp/gh_${GH_VERSION}_linux_amd64/bin/gh" /usr/local/bin/gh && \
  chmod +x /usr/local/bin/gh && \
  rm -rf /tmp/gh.tgz "/tmp/gh_${GH_VERSION}_linux_amd64"

RUN curl -fsSL "https://github.com/profclems/glab/releases/download/v${GLAB_VERSION}/glab_${GLAB_VERSION}_Linux_x86_64.tar.gz" -o /tmp/glab.tgz && \
  tar -C /tmp -xzf /tmp/glab.tgz && \
  if [ -f /tmp/bin/glab ]; then mv /tmp/bin/glab /usr/local/bin/glab; else mv /tmp/glab /usr/local/bin/glab; fi && \
  chmod +x /usr/local/bin/glab && \
  rm -rf /tmp/glab.tgz /tmp/bin /tmp/glab

RUN useradd -m -u 10001 -s /usr/sbin/nologin app

COPY --from=build /out/tool-router /usr/local/bin/tool-router

USER 10001:10001
ENV HOME=/home/app

EXPOSE 8081
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 CMD wget -qO- http://localhost:8081/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/tool-router","-config","/etc/carapulse/config.json"]

