#!/bin/bash

VERSION=""

get_version(){
    if [[ -z "$VERSION" ]]; then
        LATEST_VERSION=$(curl -s https://api.github.com/repos/perber/leafwiki/releases/latest \
        | grep '"tag_name":' \
        | sed -E 's/.*"v([^"]+)".*/\1/')
        VERSION=$LATEST_VERSION
    fi
}

if ! command -v docker &> /dev/null; then
    echo "Error: docker is not installed. Please install docker and try again."
    exit 1
fi

docker ps | grep leafwiki > docker.txt
if [ ! -s docker.txt ]; then
  echo "Error: no container found"
  exit 1
fi
DOCKER_ID=$(cat docker.txt | cut -d ' ' -f1)
rm docker.txt

MOUNT=$(docker inspect --format '{{json .Mounts}}' $DOCKER_ID)
CONTAINER_NAME=$(docker inspect --format '{{.Name}}' "$DOCKER_ID" | sed 's/^\///')

VOLUME_ARGS=()
if [ "$MOUNT" = "[]" ] || [ "$MOUNT" = "null" ]; then
  read -rp "No volume associated. Are you sure you want to continue? (y/N): " RESPONSE_MOUNT

  if [[ "$RESPONSE_MOUNT" != "y" && "$RESPONSE_MOUNT" != "Y" ]]; then
    echo "Update service interrupted"
    exit 1
  fi
else
  # Retrieve volumes cleanly via docker inspect
  SOURCE=$(docker inspect --format '{{if .Mounts}}{{ (index .Mounts 0).Source }}{{end}}' "$DOCKER_ID")
  DESTINATION=$(docker inspect --format '{{if .Mounts}}{{ (index .Mounts 0).Destination }}{{end}}' "$DOCKER_ID")
  if [ -n "$SOURCE" ] && [ -n "$DESTINATION" ]; then
    VOLUME_ARGS=(-v "$SOURCE:$DESTINATION")
  fi
fi

CONTAINER_UUID=$(docker inspect --format '{{.Config.User}}' "$DOCKER_ID")

get_version

PORT_MAPPING=$(docker inspect --format='{{(index (index .NetworkSettings.Ports "8080/tcp") 0).HostPort}}' "$DOCKER_ID")

if [ -z "$PORT_MAPPING" ]; then
  echo "Error: no 8080 port exposed on this container, defaulting to 8080"
  PORT_MAPPING=8080
fi

ARGS=$(docker inspect --format '{{range .Args}}{{.}} {{end}}' "$DOCKER_ID")

docker stop "$DOCKER_ID"
docker rm "$DOCKER_ID"

RUN_USER_ARGS=()
if [ -n "$CONTAINER_UUID" ]; then
  RUN_USER_ARGS=(-u "$CONTAINER_UUID")
fi

echo "Starting updated container: ${CONTAINER_NAME:-leafwiki} (version v${VERSION})..."
docker run -d --name "${CONTAINER_NAME:-leafwiki}" -p "${PORT_MAPPING}:8080" "${VOLUME_ARGS[@]}" "${RUN_USER_ARGS[@]}" ghcr.io/perber/leafwiki:v${VERSION} ${ARGS}
