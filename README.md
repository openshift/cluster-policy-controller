# cluster-policy-controller
The cluster-policy-controller is responsible for maintaining policy resources necessary to create pods in a cluster. 
Controllers managed by cluster-policy-controller are:
* cluster quota reconcilion - manages cluster quota usage
* namespace SCC allocation controller - allocates UIDs and SELinux labels for namespaces     
* cluster csr approver controller - csr approver for monitoring scraping
* podsecurity admission label syncer controller - configure the PodSecurity admission namespace label for namespaces with "security.openshift.io/scc.podSecurityLabelSync: true" label

## Run
The `cluster-policy-controller` runs as a container in the `openshift-kube-controller-manager namespace`, in the kube-controller-manager static pod.
This pod is defined and managed by the [`kube-controller-manager`](https://github.com/openshift/cluster-kube-controller-manager-operator/)
[OpenShift ClusterOperator](https://github.com/openshift/enhancements/blob/master/dev-guide/operators.md#what-is-an-openshift-clusteroperator).
that installs and maintains the KubeControllerManager [Custom Resource](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) in a cluster.  It can be viewed with:     
```
oc get clusteroperator kube-controller-manager -o yaml
```

## Test
Many OpenShift ClusterOperators and Operands share common build, test, deployment, and update methods, see [How do I build|update|verify|run unit-tests](https://github.com/openshift/enhancements/blob/master/dev-guide/operators.md#how-do-i-buildupdateverifyrun-unit-tests).

See [How can I test changes to an OpenShift operator/operand/release component?](https://github.com/openshift/enhancements/blob/master/dev-guide/operators.md#how-can-i-test-changes-to-an-openshift-operatoroperandrelease-component) to deploy OpenShift with your test `cluster-kube-controller-manager-operator` and `cluster-policy-controller` images.

## Rebase
Follow this checklist and copy into the PR:

- [ ] Select the desired [kubernetes release branch](https://github.com/kubernetes/kubernetes/branches), and use its `go.mod` and `CHANGELOG` as references for the rest of the work.
- [ ] Bump go version, all `k8s.io/`, `github.com/openshift/`, and any other relevant dependencies as needed.
- [ ] Run `go mod vendor && go mod tidy`, commit that separately from all other changes.
- [ ] Bump image versions (Dockerfile, ci...) if needed.
- [ ] Run `make build verify test`.
- [ ] Make code changes as needed until the above pass.
- [ ] Any other minor update, like documentation.
