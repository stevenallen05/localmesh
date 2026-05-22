FROM mcr.microsoft.com/devcontainers/base:ubuntu

RUN apt-get update && apt-get install -y \
    zsh \
    git \
    curl \
    wget \
    ca-certificates \
    build-essential

# golang
ARG GO_VERSION=1.24.3

RUN curl -fsSL https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz \
    | tar -C /usr/local -xz

ENV PATH="/usr/local/go/bin:${PATH}"

# vscode user shell
RUN chsh -s /usr/bin/zsh vscode

USER vscode

