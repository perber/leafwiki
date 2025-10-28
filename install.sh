#!/bin/bash

ARCHI="amd64"
PUBLIC_ACCESS="false"
DATA_DIR="$PWD/data"
EXEC_NAME=""

while [[ $# -gt 0 ]]; do
    case "$1" in 
        -archi|--archi)
            ARCHI="$2"
            shift 2
            ;;
        *)
            echo "Invalid options"
            exit 1
            ;;
    esac
done

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

case "$ARCHI" in 
    "amd64")
        wget https://github.com/perber/leafwiki/releases/download/v0.4.8/leafwiki-v0.4.8-linux-amd64 || exit 1
        chmod +x ./leafwiki-v0.4.8-linux-amd64
        EXEC_NAME="leafwiki-v0.4.8-linux-amd64"
        ;;
    "arm64")
        wget https://github.com/perber/leafwiki/releases/download/v0.4.8/leafwiki-v0.4.8-linux-arm64 || exit 1
        chmod +x ./leafwiki-v0.4.8-linux-arm64
        EXEC_NAME="leafwiki-v0.4.8-linux-arm64"
        ;;
    *)
        echo "The architecture $ARCHI is not supported"
        exit 1
        ;;
esac

mkdir -p "$DATA_DIR"
RUN_USER="${SUDO_USER:-$USER}"
sudo chown -R "$RUN_USER:$RUN_USER" "$DATA_DIR"
echo "[Unit]
Description=LeafWiki
After=network.target

[Service]
User=$RUN_USER
ExecStart=$PWD/$EXEC_NAME --data-dir=$DATA_DIR --jwt-secret=\"$JWT_PASSWORD\" --public-access=$PUBLIC_ACCESS --admin-password=$ADMIN_PASSWORD
Restart=on-failure

[Install]
WantedBy=multi-user.target
" > /etc/systemd/system/leafwiki.service

systemctl enable leafwiki
systemctl start leafwiki
