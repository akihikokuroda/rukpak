#!/usr/bin/env bash

NODE_PORT=$(kubectl get service docker-registry -n rukpak-system  -o jsonpath='{ .spec.ports[0].nodePort }')

# push test bundle image into in-cluster docker registry
kubectl exec nerdctl -n rukpak-system -- sh -c 'nerdctl login -u myuser -p mypasswd docker-registry:5000 --insecure-registry'

for x in $(docker images --format "{{.Repository}}:{{.Tag}}" | grep testdata); do
    kubectl exec nerdctl -n rukpak-system -- sh -c "nerdctl -n k8s.io tag $x docker-registry:5000${x##testdata}"
    kubectl exec nerdctl -n rukpak-system -- sh -c "nerdctl -n k8s.io push docker-registry:5000${x##testdata} --insecure-registry"
done

