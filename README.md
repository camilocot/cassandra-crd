Cassandra Custom Resource
=========================

Cassandra Custom Resource -  This will be a Kubernetes Custom Resource for Cassandra

The goal of this Custom Resource Definition (CRD) is to support various life-cycle actions
for a Cassandra instance, such as:

- Decommission a C* instance
- Bootstrap a C* instance
- Configuring authentication

## Running

**Prerequisite**: Since the sample-controller uses `apps/v1` statefulset, the Kubernetes cluster version should be greater than 1.9.

```sh
# assumes you have a working kubeconfig, not required if operating in-cluster
$ go run *.go -kubeconfig=$HOME/.kube/local -logtostderr=true

# create a CustomResourceDefinition
$ kubectl create -f examples/crd.yaml

# create a custom resource of type CassandraCluster
$ kubectl create -f examples/cassandra-cluster.yaml

# check statefulset created through the custom resource
$ kubectl get statefulset
```
