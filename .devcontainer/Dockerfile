FROM mcr.microsoft.com/devcontainers/go:2-1.26

RUN go install github.com/air-verse/air@v1.62.0 && \
    go install github.com/dapr/cli@v1.16.5 && \
    mv /go/bin/cli /go/bin/dapr

RUN curl -fsSL https://opencode.ai/install | VERSION=1.15.10 bash && \
    cp /root/.opencode/bin/opencode /usr/local/bin/opencode && \
    chmod 755 /usr/local/bin/opencode

# Install Docker CLI (engine is accessed via host socket mount)
RUN curl -fsSL https://download.docker.com/linux/debian/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/debian $(. /etc/os-release && echo "$VERSION_CODENAME") stable" > /etc/apt/sources.list.d/docker.list && \
    apt-get update -qq && apt-get install -y -qq --no-install-recommends docker-ce-cli docker-compose-plugin

# Install Google Cloud CLI
RUN curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | gpg --dearmor -o /usr/share/keyrings/cloud.google.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" > /etc/apt/sources.list.d/google-cloud-sdk.list && \
    apt-get update -qq && apt-get install -y -qq --no-install-recommends google-cloud-cli

# Install OpenTofu
RUN TOFU_VERSION=${TOFU_VERSION:-1.10.1} && \
    TOFU_ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') && \
    curl -sSfL "https://github.com/opentofu/opentofu/releases/download/v${TOFU_VERSION}/tofu_${TOFU_VERSION}_linux_${TOFU_ARCH}.zip" -o /tmp/tofu.zip && \
    unzip -qo /tmp/tofu.zip -d /tmp/tofu && \
    install /tmp/tofu/tofu /usr/local/bin/tofu && \
    rm -rf /tmp/tofu.zip /tmp/tofu

# Give vscode user access to Go module cache and Docker socket
RUN chmod -R g+w /go/pkg/mod && \
    groupadd -g 102 docker 2>/dev/null; \
    usermod -aG docker vscode 2>/dev/null; true
