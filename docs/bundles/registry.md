# Registry Bundle Spec

## Overview

This document is meant to define the registry bundle format as a reference for those publishing registry bundles for use with
RukPak. A bundle is a collection of Kubernetes resources that are packaged together for the purposes of installing onto
a Kubernetes cluster. A bundle can be unpacked onto a running cluster, where controllers can then create the underlying
content embedded in the bundle. The bundle can be used as the underlying `spec.source` for
a [Bundle](https://github.com/operator-framework/rukpak#bundle) resource.

The `registry+v1` bundles, or `registry` bundles, contains a set of static Kubernetes YAML
manifests organized in the legacy Operator Lifecycle Manger (OLM) format. For more information on the `registry+v1` format, see
the [OLM packaging doc](https://olm.operatorframework.io/docs/tasks/creating-operator-manifests/).
