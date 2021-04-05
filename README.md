# Hypershift Lite
Lightweight operator that creates a simple Kubernetes API server with no workers.

## Getting Started

### Requirements
- An existing Kubernetes or OpenShift cluster
- The OpenShift cli client

### Install
- Clone this repository
- Install CRDs
  ```sh
  oc apply  -f ./config/crds
  ```
- Install Operator
  ```sh
  oc apply -f ./config/operator
  ```

### Create a KubernetesService

- Create a namespace for your k8s service
  ```sh
  oc create namespace mykube
  ```
- Create a pull secret for OpenShift images
  ```sh
   oc create secret generic pull-secret -n mykube \
      --from-file .dockerconfigjson=PATH_TO_YOUR_PULLSECRET \
      --type=kubernetes.io/dockerconfigjson
  ```
- Create a KubernetesService resource
  ```sh
  oc create -n mykube -f example/myk8s.yaml
  ```

- Wait for Kubernetes API server to come up. The KubernetesService resource will report an `Available` condition as `True`. You can also monitor the pods in the namespace where the resource was created.

### Use the KubernetesService
- Download a localhost kubeconfig from the `localhost-kubeconfig` secret
  ```
  oc get secret localhost-kubeconfig -n mykube -o jsonpath='{ .data.kubeconfig }' | base64 -d > /tmp/mykubeconfig
  ```
- In a separate window, port-forward the kubernetes-apiserver service to your machine
  ```sh
  oc port-forward -n mykube svc/kube-apiserver 6443:6443
  ```
- Point the KUBECONFIG env var to the new kubeconfig
  ```
  export KUBECONFIG=/tmp/mykubeconfig
  ```
- Start creating/querying resources on the API server with `oc` or `kubectl`

### Use the KubernetesService from another pod
- The KubernetesService resource generates a secret named `kubeconfig` that points to the service
- You can mount that secret from another pod and simply set that pod's `KUBECONFIG` environment variable to point to the mounted kubeconfig.
