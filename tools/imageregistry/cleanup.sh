#!/usr/bin/env bash

rm -rf /tmp/var/imageregistry/certs

# for host configuration updates
#rm -rf /etc/docker/certs.d/$REGISTRY_NAME:$NODE_PORT
#sed -i /$REGISTRY_NAME/d  /etc/hosts
