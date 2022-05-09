#!/usr/bin/env bash

# push test bundle image into in-cluster docker registry
kubectl exec nerdctl -n rukpak-system -- sh -c 'nerdctl login -u myuser -p mypasswd docker-registry:5000 --insecure-registry'

docker build testdata/bundles/plain-v0/valid -t testdata/bundles/plain-v0:valid
kind load docker-image testdata/bundles/plain-v0:valid --name kind
kubectl exec nerdctl -n rukpak-system -- sh -c 'nerdctl -n k8s.io tag testdata/bundles/plain-v0:valid docker-registry:5000/bundles/plain-v0:valid'
kubectl exec nerdctl -n rukpak-system -- sh -c 'nerdctl -n k8s.io push docker-registry:5000/bundles/plain-v0:valid --insecure-registry'

# create bundle
kubectl apply -f tools/imageregistry/bundle_local_image.yaml
