package worker

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (h *WorkerHandler) InstallScript(c *gin.Context) {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if forwarded := c.GetHeader("X-Forwarded-Proto"); forwarded != "" {
		scheme = forwarded
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)

	bootstrapPath := "/api/public/v1/workers/bootstrap"
	if strings.HasSuffix(c.Request.URL.Path, "/install.sh") {
		bootstrapPath = strings.TrimSuffix(c.Request.URL.Path, "/install.sh")
	}
	installURL := baseURL + bootstrapPath + "/install.sh"
	downloadURL := baseURL + bootstrapPath + "/download/${OS}/${ARCH}"

	script := fmt.Sprintf(`#!/bin/bash
set -e

# Lute Worker Installer
# Usage: curl -sSL %s | bash

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
esac

INSTALL_DIR="/usr/local/bin"
BINARY_NAME="lute-worker"

echo "==> Detecting platform: ${OS}/${ARCH}"
echo "==> Downloading worker from %s ..."

curl -fSL -o "/tmp/${BINARY_NAME}" \
  "%s"

chmod +x "/tmp/${BINARY_NAME}"
sudo mv "/tmp/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"

echo "==> Installed ${BINARY_NAME} to ${INSTALL_DIR}/${BINARY_NAME}"
${INSTALL_DIR}/${BINARY_NAME} --version

echo ""
echo "==> Next steps:"
echo "    1. Register this host (copy the command from the Add Worker dialog in the Lute UI):"
echo "         ${BINARY_NAME} setup --claim-code <CODE>"
echo "    2. Start the agent against the server's gRPC address:"
echo "         ${BINARY_NAME} run --server <GRPC_ADDR>"
`, installURL, baseURL, downloadURL)

	c.Data(http.StatusOK, "text/x-shellscript", []byte(script))
}
