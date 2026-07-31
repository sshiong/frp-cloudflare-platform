#!/bin/bash
# FRP Panel Client Installation Script
set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
INSTALL_DIR="/usr/local/bin"
DATA_DIR="/var/lib/frp-client-panel"
LOG_DIR="/var/log/frp-client-panel"
USER="frp-panel"
GROUP="frp-panel"

echo -e "${GREEN}=== FRP Panel Client Installation ===${NC}"

# Check root
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}Error: Please run as root${NC}"
    exit 1
fi

# Detect OS
OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
    Linux*)
        PLATFORM="linux"
        ;;
    Darwin*)
        PLATFORM="darwin"
        ;;
    *)
        echo -e "${RED}Unsupported OS: $OS${NC}"
        exit 1
        ;;
esac

case "$ARCH" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo -e "${RED}Unsupported architecture: $ARCH${NC}"
        exit 1
        ;;
esac

echo -e "${GREEN}Detected platform: ${PLATFORM}-${ARCH}${NC}"

# Create user and group
if ! id "$USER" &>/dev/null; then
    echo -e "${YELLOW}Creating user: ${USER}${NC}"
    useradd -r -s /bin/false -d "$DATA_DIR" "$USER"
fi

# Create directories
echo -e "${YELLOW}Creating directories...${NC}"
mkdir -p "$DATA_DIR" "$LOG_DIR"
mkdir -p "$DATA_DIR/frpc/current" "$DATA_DIR/frpc/rollback" "$DATA_DIR/secrets"

# Set permissions
chown -R "$USER:$GROUP" "$DATA_DIR" "$LOG_DIR"
chmod 750 "$DATA_DIR" "$LOG_DIR"
chmod 700 "$DATA_DIR/secrets"

# Copy binary
echo -e "${YELLOW}Installing binary...${NC}"
if [ -f "frp-panel-client" ]; then
    cp frp-panel-client "$INSTALL_DIR/"
    chmod 755 "$INSTALL_DIR/frp-panel-client"
fi

# Install systemd service
if [ -d "/etc/systemd/system" ]; then
    echo -e "${YELLOW}Installing systemd service...${NC}"

    if [ -f "frp-panel-client.service" ]; then
        cp frp-panel-client.service /etc/systemd/system/
    fi

    systemctl daemon-reload

    echo -e "${GREEN}Systemd service installed!${NC}"
    echo -e "  Start client: ${YELLOW}systemctl start frp-panel-client${NC}"
    echo -e "  Enable on boot: ${YELLOW}systemctl enable frp-panel-client${NC}"
fi

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Installation Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "Default configuration:"
echo -e "  Listen address: ${YELLOW}127.0.0.1:7410${NC}"
echo -e "  Data directory: ${YELLOW}${DATA_DIR}${NC}"
echo -e "  Log directory:  ${YELLOW}${LOG_DIR}${NC}"
echo ""
echo -e "Access the client panel at: ${YELLOW}http://127.0.0.1:7410${NC}"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo -e "  1. Open the client panel in your browser"
echo -e "  2. Configure the server address"
echo -e "  3. Login with your server panel credentials"
echo -e "  4. The device will be automatically registered"
