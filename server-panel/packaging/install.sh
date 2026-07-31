#!/bin/bash
# FRP Panel Server Installation Script
set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
INSTALL_DIR="/usr/local/bin"
DATA_DIR="/data"
CONFIG_DIR="/etc/frp-panel"
LOG_DIR="/var/log/frp-panel"
USER="frp-panel"
GROUP="frp-panel"

echo -e "${GREEN}=== FRP Panel Server Installation ===${NC}"

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
mkdir -p "$DATA_DIR" "$CONFIG_DIR" "$LOG_DIR"
mkdir -p "$DATA_DIR/router/snapshots" "$DATA_DIR/router/certificates"

# Set permissions
chown -R "$USER:$GROUP" "$DATA_DIR" "$LOG_DIR"
chmod 750 "$DATA_DIR" "$LOG_DIR"

# Copy binaries
echo -e "${YELLOW}Installing binaries...${NC}"
if [ -f "frp-panel-server" ]; then
    cp frp-panel-server "$INSTALL_DIR/"
    chmod 755 "$INSTALL_DIR/frp-panel-server"
fi

if [ -f "frp-panel-server-router" ]; then
    cp frp-panel-server-router "$INSTALL_DIR/"
    chmod 755 "$INSTALL_DIR/frp-panel-server-router"
fi

# Install systemd services
if [ -d "/etc/systemd/system" ]; then
    echo -e "${YELLOW}Installing systemd services...${NC}"

    if [ -f "frp-panel-control.service" ]; then
        cp frp-panel-control.service /etc/systemd/system/
    fi

    if [ -f "frp-panel-router.service" ]; then
        cp frp-panel-router.service /etc/systemd/system/
    fi

    systemctl daemon-reload

    echo -e "${GREEN}Systemd services installed!${NC}"
    echo -e "  Start control: ${YELLOW}systemctl start frp-panel-control${NC}"
    echo -e "  Start router:  ${YELLOW}systemctl start frp-panel-router${NC}"
    echo -e "  Enable on boot: ${YELLOW}systemctl enable frp-panel-control frp-panel-router${NC}"
fi

# Generate initial admin password
echo -e "${YELLOW}Generating initial admin password...${NC}"
ADMIN_PASSWORD=$(openssl rand -base64 12 | tr -dc 'a-zA-Z0-9' | head -c 12)

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Installation Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "Admin credentials:"
echo -e "  Username: ${YELLOW}admin${NC}"
echo -e "  Password: ${YELLOW}${ADMIN_PASSWORD}${NC}"
echo ""
echo -e "${RED}IMPORTANT: Save this password! It will not be shown again.${NC}"
echo -e "${RED}You will be required to change it on first login.${NC}"
echo ""
echo -e "Default ports:"
echo -e "  Control API: ${YELLOW}9000${NC} (localhost only)"
echo -e "  Router HTTP: ${YELLOW}80${NC}"
echo -e "  Router HTTPS:${YELLOW}443${NC}"
echo -e "  FRPS:        ${YELLOW}7000${NC}"
echo ""
echo -e "Access the panel at: ${YELLOW}https://your-server-ip${NC}"
