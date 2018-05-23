#!/usr/bin/env bash

if ! which docker > /dev/null; then
	echo "docker needs to be installed"
	exit 1
fi

: ${IMAGE:?"Need to set IMAGE"}

echo "building container..."
docker build --tag "${IMAGE}" -f hack/build/Dockerfile . 1>/dev/null
