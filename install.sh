#!/bin/bash

# Usage function
usage() {
    echo "Usage: $0 [--non-interactive [options]] | [interactive mode]"
    echo ""
    echo "Interactive mode (default):"
    echo "  $0"
    echo ""
    echo "Non-interactive mode:"
    echo "  $0 --non-interactive -arch|--arch <architecture> -jwt-password|--jwt-password <password> -admin-password|--admin-password <password> [options]"
    echo ""
    echo "Required flags (non-interactive mode only):"
    echo "  -arch, --arch                 Specify the system architecture. Supported values: amd64, arm64"
    echo "  -jwt-password, --jwt-password Specify the JWT password"
    echo "  -admin-password, --admin-password Specify the admin password"
    echo ""
    echo "Optional flags:"
    echo "  -version, --version           Specify the LeafWiki version to install (default: latest)"
    echo "  -host, --host                 Specify the host address on which LeafWiki will be hosted (default: 127.0.0.1)"
    echo "  -port, --port                 Specify the port on which LeafWiki will be hosted (default: 8080)"
    echo "  -public-access, --public-access Set public access (true/false, default: false)"
    echo "  -data-dir, --data-dir         Specify the data directory (default: ./data)"
    echo "  -help, --help                 Display this help message"
    exit 1
}

ARCH=""
PUBLIC_ACCESS="false"
DATA_DIR="$PWD/data"
EXEC_NAME=""
PORT="8080"
HOST="127.0.0.1"
INTERACTIVE=1

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

## Check if --non-interactive flag is present
if [[ "$*" == *"--non-interactive"* ]]; then
    INTERACTIVE=0
fi

if [[ "$INTERACTIVE" == 0 ]]; then
    # Parse all arguments in non-interactive mode
    while [[ $# -gt 0 ]]; do
        case "$1" in 
            -non-interactive|--non-interactive)
                shift 1
                ;;
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
            -public-access|--public-access)
                if [[ $2 == "true" || $2 == "True" ]]; then
                    PUBLIC_ACCESS="true"
                else
                    PUBLIC_ACCESS="false"
                fi
                shift 2
                ;;
            -jwt-password|--jwt-password)
                JWT_PASSWORD="$2"
                shift 2
                ;;
            -admin-password|--admin-password)
                ADMIN_PASSWORD="$2"
                shift 2
                ;;
            -data-dir|--data-dir)
                DATA_DIR="$2"
                shift 2
                ;;
            *)
                echo "Error: Unknown option '$1'"
                usage
                ;;
        esac
    done

    # Validate all required flags are present
    if [[ -z "$ARCH" ]]; then
        echo "Error: -arch is required in non-interactive mode."
        usage
    fi
    if [[ -z "$JWT_PASSWORD" ]]; then
        echo "Error: -jwt-password is required in non-interactive mode."
        usage
    fi
    if [[ -z "$ADMIN_PASSWORD" ]]; then
        echo "Error: -admin-password is required in non-interactive mode."
        usage
    fi

    # Validate architecture
    if [[ "$ARCH" != "amd64" && "$ARCH" != "arm64" ]]; then
        echo "Error: Unsupported architecture '$ARCH'. Supported values are: amd64, arm64"
        exit 1
    fi


else
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


    read -rp "Which architecture do you want to use? (amd64/amd64): " ARCH
    echo
    read -rsp "Which JWT password do you want to use: " JWT_PASSWORD
    echo
    read -rsp "Which admin password do you want to use: " ADMIN_PASSWORD
    echo
    read -rp "Which host do you want to use: " HOST
    echo
    read -rp "Which port do you want to use: " PORT
    echo
    read -rp "Should people without an account have read access? (default: n) y/N: " RESPONSE
    if [[ $RESPONSE == "y" || $RESPONSE == "Y" ]]; then
        PUBLIC_ACCESS="true"
    else
        PUBLIC_ACCESS="false"
    fi

    if ! [[ "$PORT" =~ ^[0-9]+$ ]] || [ "$PORT" -lt 1 ] || [ "$PORT" -gt 65535 ]; then
        echo "Error: port requires a valid port number (1-65535)."
        usage
        exit
    fi

    read -p "Where should the data be saved? (default: $DATA_DIR): " RESPONSE_DATA_DIR
    if [[ -n "$RESPONSE_DATA_DIR" ]]; then
        DATA_DIR="$RESPONSE_DATA_DIR"
    fi

fi

    # If no version is specified, get the latest version from GitHub
if [[ -z "$VERSION" ]]; then
    LATEST_VERSION=$(curl -s https://api.github.com/repos/perber/leafwiki/releases/latest \
    | grep '"tag_name":' \
    | sed -E 's/.*"v([^"]+)".*/\1/')
    VERSION=$LATEST_VERSION
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
    echo "======================================"
    echo "== LeafWiki installation completed! =="
    echo "==                                  =="
    echo "== Host: $HOST                   =="
    echo "== Port: $PORT                       =="
    echo "== DataDirectory: $DATA_DIR.           =="
    echo "== Status : $IS_ACTIVE                  =="
    echo "==                                  =="
    echo "======================================"
fi

