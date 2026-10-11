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

# verify_checksum <file> <url>: compare <file> against the hash published at
# <url>.sha256. Only the hash field is compared, so checksum files that name
# the binary with a "releases/" prefix (older releases) work too.
verify_checksum(){
    local file="$1" url="$2" expected actual
    expected=$(wget -qO- "${url}.sha256" | awk '{print $1}')
    if [[ -z "$expected" ]]; then
        echo "Error: Could not download checksum from ${url}.sha256" >&2
        return 1
    fi
    actual=$(sha256sum "$file" | awk '{print $1}')
    if [[ "$expected" != "$actual" ]]; then
        echo "Error: Checksum mismatch for $(basename "$file") (expected ${expected}, got ${actual})" >&2
        return 1
    fi
}

download_binary(){
    TMP_FILE="/tmp/leafwiki-v${VERSION}-linux-${ARCH}"
    URL="${RELEASE_LINK}releases/download/v${VERSION}/leafwiki-v${VERSION}-linux-${ARCH}"
    wget -O "$TMP_FILE" "$URL" || {
        echo "Download failed. Aborting update." >&2
        exit 1
    }
    
    if [ ! -s "$TMP_FILE" ]; then
        echo "Downloaded file is empty. Aborting update." >&2
        rm -f "$TMP_FILE"
        exit 1
    fi

    if ! verify_checksum "$TMP_FILE" "$URL"; then
        echo "Aborting update, current binary left untouched." >&2
        rm -f "$TMP_FILE"
        exit 1
    fi

    # Backup current binary
    mv "$PATH_TO_BINARY" "${PATH_TO_BINARY}.bak"
    
    # Install new binary
    mv "$TMP_FILE" "$PATH_TO_BINARY"
    chmod +x "$PATH_TO_BINARY"
}

if ! command -v sha256sum &> /dev/null; then
    echo "Error: sha256sum is not installed. Please install coreutils and try again." >&2
    exit 1
fi

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