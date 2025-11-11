#!/bin/bash

# Usage function
usage() {
    echo "Usage: $0 -arch|--arch <architecture> [-version|--version <version>] [-host|--host <host>] [-port|--port <port>]"
    echo "  -arch, --arch         Specify the system architecture. Supported values: amd64, arm64"
    echo "  -version, --version   Specify the LeafWiki version to install (default: latest)"
    echo "  -host, --host         Specify the host address on which LeafWiki will be hosted"
    echo "  -port, --port         Specify the port on which LeafWiki will be hosted"
    exit 1
}

ARCH=""
PUBLIC_ACCESS="false"
DATA_DIR="$PWD/data"
EXEC_NAME=""
PORT="8080"
HOST="0.0.0.0"

# test wget is installed
if ! command -v wget &> /dev/null; then
    echo "Error: wget is not installed. Please install wget and try again."
    exit 1
fi

# test if systemctl is installed
if ! command -v systemctl &> /dev/null; then
    echo "Error: systemctl is not installed. This script requires systemd to manage the LeafWiki service."
    exit 1
fi

while [[ $# -gt 0 ]]; do
    case "$1" in 
        -arch|--arch)
            ARCH="$2"
            shift 2
            ;;
        -version|--version)
            if [[ -z "$2" ]]; then
                echo "Error: --version requires a non-empty option argument."
                usage
            fi
            VERSION="$2"
            shift 2
            ;;
        -host|--host)
            HOST="$2"
            shift 2
            ;;
        -port|--port)
            if ! [[ "$2" =~ ^[0-9]+$ ]] || [ "$2" -lt 1 ] || [ "$2" -gt 65535 ]; then
                echo "Error: --port requires a valid port number (1-65535)."
                usage
            fi
            PORT="$2"
            shift 2
            ;;
        -help|--help)
            usage
            exit 1;
            ;;
        *)
            echo "Error: Unknown option '$1'"
            usage
            ;;
    esac
done

# Validate architecture
if [[ -z "$ARCH" ]]; then
    usage
    echo "Error: Architecture not specified."
    exit 1
fi
if [[ "$ARCH" != "amd64" && "$ARCH" != "arm64" ]]; then
    echo "Error: Unsupported architecture '$ARCH'. Supported values are: amd64, arm64"
    exit 1
fi

# If no version is specified, get the latest version from GitHub
if [[ -z "$VERSION" ]]; then
    LATEST_VERSION=$(curl -s https://api.github.com/repos/perber/leafwiki/releases/latest \
    | grep '"tag_name":' \
    | sed -E 's/.*"v([^"]+)".*/\1/')
    VERSION=$LATEST_VERSION
fi


echo "___________________________________________________"
echo "|       __               _____       ___ __   _   |";
echo "|      / /   ___  ____ _/ __/ |     / (_) /__(_). |";
echo "|     / /   / _ \\/ __ \`/ /_ | | /| / / / //_/ /   |";
echo "|    / /___/  __/ /_/ / __/ | |/ |/ / / ,< / /    |";
echo "|.  /_____/\\___/\\__,_/_/    |__/|__/_/_/|_/_/     |";
echo "|_________________________________________________|";
echo ""
                                   
echo "========================================"
echo "   LeafWiki — Installer"
echo "========================================"
echo ""
echo ""


read -rsp "Which JWT password do you want to use: " JWT_PASSWORD
echo
read -rsp "Which admin password do you want to use: " ADMIN_PASSWORD
echo
read -rp "Should people without an account have read access? (default: n) y/N: " RESPONSE
if [[ $RESPONSE == "y" || $RESPONSE == "Y" ]]; then
    PUBLIC_ACCESS="true"
else
    PUBLIC_ACCESS="false"
fi

read -p "Where should the data be saved? (default: $DATA_DIR): " RESPONSE_DATA_DIR
if [[ -n "$RESPONSE_DATA_DIR" ]]; then
    DATA_DIR="$RESPONSE_DATA_DIR"
fi

case "$ARCH" in 
    "amd64")
        wget https://github.com/perber/leafwiki/releases/download/v$VERSION/leafwiki-v$VERSION-linux-amd64 || exit 1
        chmod +x ./leafwiki-v$VERSION-linux-amd64
        EXEC_NAME="leafwiki-v$VERSION-linux-amd64"
        ;;
    "arm64")
        wget https://github.com/perber/leafwiki/releases/download/v$VERSION/leafwiki-v$VERSION-linux-arm64 || exit 1
        chmod +x ./leafwiki-v$VERSION-linux-arm64
        EXEC_NAME="leafwiki-v$VERSION-linux-arm64"
        ;;
    *)
        echo "The architecture $ARCH is not supported"
        exit 1
        ;;
esac

mkdir -p "$DATA_DIR"
RUN_USER="${SUDO_USER:-$USER}"
chown -R "$RUN_USER:$RUN_USER" "$DATA_DIR"
echo "[Unit]
Description=LeafWiki
After=network.target

[Service]
User=$RUN_USER
ExecStart=$(realpath $PWD)/$EXEC_NAME --data-dir=$DATA_DIR --jwt-secret=\"$JWT_PASSWORD\" --public-access=$PUBLIC_ACCESS --admin-password=$ADMIN_PASSWORD --port=$PORT --host=$HOST
Restart=on-failure

[Install]
WantedBy=multi-user.target
" > /etc/systemd/system/leafwiki.service

systemctl enable leafwiki
systemctl start leafwiki
IS_ACTIVE=$(systemctl is-active leafwiki)
if [[ "$IS_ACTIVE" == "failed" ]]; then
    echo "Installation failed: "
    systemctl status leafwiki
else
    echo "leafWiki installation completed!"
    echo "Host: $HOST"
    echo "Port: $PORT"
    echo "DataDirectory: $DATA_DIR"
    echo "Status : $IS_ACTIVE"
fi

