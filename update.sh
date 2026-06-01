#!/bin/bash 

VERSION=""
PATH_TO_BINARY="/usr/local/bin/leafwiki"
RELEASE_LINK="https://github.com/perber/leafwiki/"

ARCH=$(uname -m)
 case "$ARCH" in
     x86_64)
         ARCH="amd64"
         ;;
     arm64|aarch64)
         ARCH="arm64"
         ;;
     *)
         echo "Unsupported architecture: $ARCH" >&2
         exit 1
         ;;
 esac

get_version(){
    if [[ -z "$VERSION" ]]; then
        LATEST_VERSION=$(curl -s https://api.github.com/repos/perber/leafwiki/releases/latest \
        | grep '"tag_name":' \
        | sed -E 's/.*"v([^"]+)".*/\1/')
        VERSION=$LATEST_VERSION
        	
        if [[ -z "$LATEST_VERSION" || "$LATEST_VERSION" == "null" ]]; then
            echo "Failed to determine the latest LeafWiki version from GitHub." >&2
            exit 1
        fi
    fi
}

download_binary(){
    TMP_FILE="/tmp/leafwiki-v${VERSION}-linux-${ARCH}"
    wget -O "$TMP_FILE" "${RELEASE_LINK}releases/download/v${VERSION}/leafwiki-v${VERSION}-linux-${ARCH}" || {
        echo "Download failed. Aborting update." >&2
        exit 1
    }
    
    if [ ! -s "$TMP_FILE" ]; then
        echo "Downloaded file is empty. Aborting update." >&2
        rm -f "$TMP_FILE"
        exit 1
    fi

    # Backup current binary
    mv "$PATH_TO_BINARY" "${PATH_TO_BINARY}.bak"
    
    # Install new binary
    mv "$TMP_FILE" "$PATH_TO_BINARY"
    chmod +x "$PATH_TO_BINARY"
}

EXIST=$(test -x $PATH_TO_BINARY && echo "true" || echo "false")

if [[ $EXIST == "false" ]]; then
    echo "leafwiki is not present in /usr/local/bin/"
    exit 1
fi

echo "Update in progress..."
get_version
download_binary

systemctl daemon-reload
systemctl restart leafwiki

echo "======================================="
echo "== LeafWiki update completed!        =="
echo "==                                   =="
printf "== %-33s ==\n" "New Version: $VERSION"
echo "==                                   =="
echo "======================================="