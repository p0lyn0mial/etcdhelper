# Etcd helper

A helper tool for getting Kubernetes data directly from Etcd. Based on https://github.com/openshift/origin/tree/release-3.11/tools/etcdhelper

## How to build

    $ go build .

## Basic Usage

This requires setting the following flags:

* `-key` - points to `master.etcd-client.key`
* `-cert` - points to `master.etcd-client.crt`
* `-cacert` - points to `ca.crt`

Once these are set properly, one can invoke the following actions:

* `ls` - list all keys starting with prefix
* `get` - get the specific value of a key
* `put` - put the specific value under a key
* `dump` - dump the entire contents of the etcd

## Sample Usage

List all keys starting with `/openshift.io`:

```
etcdhelper -key master.etcd-client.key -cert master.etcd-client.crt -cacert ca.crt ls /openshift.io
```

Get JSON-representation of `deployment/openshift-apiserver-operator` from `openshift-apiserver-operator` namespace:

```
etcdhelper -key master.etcd-client.key -cert master.etcd-client.crt -cacert ca.crt get /kubernetes.io/deployments/openshift-apiserver-operator/openshift-apiserver-operator
```

Dump the contents of etcd to stdout:

```
etcdhelper -key master.etcd-client.key -cert master.etcd-client.crt -cacert ca.crt dump
```


Put Protobuf-representation of `deployment/openshift-apiserver-operator` from `openshift-apiserver-operator` namespace:

```
etcdhelper -key master.etcd-client.key -cert master.etcd-client.crt -cacert ca.crt -resfile res.json put /kubernetes.io/deployments/openshift-apiserver-operator/openshift-apiserver-operator
```
