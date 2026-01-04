#!/bin/bash


EXEC_NAME=""
INTERACTIVE=1
ARCH=""
PUBLIC_ACCESS="false"
DATA_DIR="$PWD/data"
PORT="8080"
HOST="127.0.0.1"
VERSION=""
JWT_PASSWORD=""
ADMIN_PASSWORD=""

# Usage function
usage() {
    echo "Usage: $0 [--non-interactive [options]] | [interactive mode]"
    echo ""
    echo "Interactive mode (default):"
    echo "  $0"
    echo ""
    echo "Non-interactive mode:"
    echo "  $0 --non-interactive -env-file|--env-file <path to an env file> "
    echo ""
    echo "Environment file:"
    echo "  You can specify an env file with --env-file."
    echo "  See .env.example for available variables."
    echo ""
    echo "  -help, --help                 Display this help message"
    exit 1
}

check_dependencies() {
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
}

validate_architecture(){
    if [[ "$ARCH" != "amd64" && "$ARCH" != "arm64" ]]; then
        echo "Error: Unsupported architecture '$ARCH'. Supported values are: amd64, arm64"
        exit 1
    fi
}

validate_port(){
    if ! [[ "$PORT" =~ ^[0-9]+$ ]] || [ "$PORT" -lt 1 ] || [ "$PORT" -gt 65535 ]; then
        echo "Error: --port requires a valid port number (1-65535)."
        usage
        exit 1
    fi
}

validate_requirements_non_interactive(){
    if [[ -z "$ARCH" ]]; then
        echo "Error: ARCH environment variable is required in non-interactive mode (set it in the env file used with --env-file)."
        usage
        exit 1
    fi
    if [[ -z "$JWT_PASSWORD" ]]; then
        echo "Error: JWT_PASSWORD environment variable is required in non-interactive mode (set it in the env file used with --env-file)."
        usage
        exit 1
    fi
    if [[ -z "$ADMIN_PASSWORD" ]]; then
        echo "Error: ADMIN_PASSWORD environment variable is required in non-interactive mode (set it in the env file used with --env-file)."
        usage
        exit 1
    fi
    validate_port
}

validate_version(){
    if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        echo "Error: Invalid VERSION format '$VERSION'. Expected format: X.Y.Z"
        exit 1
    fi
}

get_version(){
    if [[ -z "$VERSION" ]]; then
        LATEST_VERSION=$(curl -s https://api.github.com/repos/perber/leafwiki/releases/latest \
        | grep '"tag_name":' \
        | sed -E 's/.*"v([^"]+)".*/\1/')
        VERSION=$LATEST_VERSION
    fi
    validate_version
}

download_binary(){
    wget "https://github.com/perber/leafwiki/releases/download/v${VERSION}/leafwiki-v${VERSION}-linux-${ARCH}" || exit 1
    chmod +x "./leafwiki-v${VERSION}-linux-${ARCH}"
    EXEC_NAME="leafwiki-v${VERSION}-linux-${ARCH}"
}

check_dependencies

# Check if --non-interactive flag is present
if [[ "$*" == *"--non-interactive"*  ||  "$*" == *"-non-interactive"* ]]; then
    INTERACTIVE=0
fi

if [[ "$INTERACTIVE" == 0 ]]; then

    while [[ $# -gt 0 ]]; do
        case "$1" in 
            -non-interactive|--non-interactive)
                shift
                ;;
            -env-file|--env-file)
                if [[ -z "$2" ]]; then
                    echo "Error: --env-file requires a path argument"
                    usage
                fi
                if [[ ! -f "$2" ]]; then
                    echo "Error: Environment file '$2' does not exist or is not a regular file"
                    exit 1
                fi
                env_file="$2"
                if [[ ! -r "$env_file" ]]; then
                    echo "Error: Cannot read env file '$env_file'"
                    exit 1
                fi
                # Source the environment file and parse each variable
                while IFS= read -r line || [[ -n "$line" ]]; do
                    # Skip empty lines and comments
                    [[ "$line" =~ ^[[:space:]]*$ || "$line" =~ ^[[:space:]]*# ]] && continue
                    
                    # Extract key=value pairs
                    [[ "$line" =~ ^([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]] || continue
                    
                    key="${BASH_REMATCH[1]}"
                    value="${BASH_REMATCH[2]}"
                    
                    # Remove surrounding quotes (both single and double)
                    value="${value%\"}"
                    value="${value#\"}"
                    value="${value%\'}"
                    value="${value#\'}"
                    
                    # Export the variable
                    export "$key=$value"
                done < "$env_file"
                shift 2
                ;;
            *)
                echo "Error: Unknown option '$1'"
                usage
                ;;
        esac
    done
    
    ARCH="${LEAFWIKI_ARCH:-$ARCH}"
    PUBLIC_ACCESS="${LEAFWIKI_PUBLIC_ACCESS:-$PUBLIC_ACCESS}"
    DATA_DIR="${LEAFWIKI_DATA_DIR:-$DATA_DIR}"
    PORT="${LEAFWIKI_PORT:-$PORT}"
    HOST="${LEAFWIKI_HOST:-$HOST}"
    VERSION="${LEAFWIKI_VERSION:-$VERSION}"
    JWT_PASSWORD="${LEAFWIKI_JWT_PASSWORD:-$JWT_PASSWORD}"
    ADMIN_PASSWORD="${LEAFWIKI_ADMIN_PASSWORD:-$ADMIN_PASSWORD}"

    validate_architecture
    validate_requirements_non_interactive

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

    read -rp "Which architecture do you want to use? (amd64/arm64): " ARCH
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

    read -p "Where should the data be saved? (default: $DATA_DIR): " RESPONSE_DATA_DIR
    if [[ -n "$RESPONSE_DATA_DIR" ]]; then
        DATA_DIR="$RESPONSE_DATA_DIR"
    fi

    validate_port
    validate_architecture

fi

get_version

download_binary

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
    echo "== Host: $HOST                      =="
    echo "== Port: $PORT                      =="
    echo "== DataDirectory: $DATA_DIR         =="
    echo "== Status : $IS_ACTIVE              =="
    echo "==                                  =="
    echo "======================================"
fi
