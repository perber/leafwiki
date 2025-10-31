#!/bin/bash

# Usage function
usage() {
    echo "Usage: $0 -arch|--arch <architecture> [-version|--version <version>]"
    echo "  -arch, --arch         Specify the system architecture. Supported values: amd64, arm64"
    echo "  -version, --version   Specify the LeafWiki version to install (default: latest)"
    exit 1
}

ARCH=""
PUBLIC_ACCESS="false"
DATA_DIR="$PWD/data"
EXEC_NAME=""

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
        -help|--help)
            usage
            exit 1;
            ;;
        *)
            usage
            echo "Error: Unknown option '$1'"
            exit 1;
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


read -rsp "Which JWT password do you want to use: " JWT_PASSWORD
echo
read -rsp "Which admin password do you want to use: " ADMIN_PASSWORD
echo
read -rp "Should people without an account have read access? (default: n) Y/n: " RESPONSE
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
        wget https://github.com/perber/leafwiki/releases/download/v0.4.8/leafwiki-v$VERSION-linux-amd64 || exit 1
        chmod +x ./leafwiki-v0.4.8-linux-amd64
        EXEC_NAME="leafwiki-v0.4.8-linux-amd64"
        ;;
    "arm64")
        wget https://github.com/perber/leafwiki/releases/download/v$VERSION/leafwiki-v$VERSION-linux-arm64 || exit 1
        chmod +x ./leafwiki-v0.4.8-linux-arm64
        EXEC_NAME="leafwiki-v0.4.8-linux-arm64"
        ;;
    *)
        echo "The archtecture $ARCH is not supported"
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
ExecStart=$(realpath $PWD)/$EXEC_NAME --data-dir=$DATA_DIR --jwt-secret=\"$JWT_PASSWORD\" --public-access=$PUBLIC_ACCESS --admin-password=$ADMIN_PASSWORD
Restart=on-failure

[Install]
WantedBy=multi-user.target
" > /etc/systemd/system/leafwiki.service

systemctl enable leafwiki
systemctl start leafwiki
