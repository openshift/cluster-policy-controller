module github.com/openshift/cluster-policy-controller

go 1.16

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/go-bindata/go-bindata v3.1.2+incompatible
	github.com/google/btree v1.0.1-0.20190326150332-20236160a414 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.1.0 // indirect
	github.com/onsi/gomega v1.8.1 // indirect
	github.com/openshift/api v0.0.0-20210706092853-b63d499a70ce
	github.com/openshift/build-machinery-go v0.0.0-20210614124016-792d61687197
	github.com/openshift/client-go v0.0.0-20210521082421-73d9475a9142
	github.com/openshift/library-go v0.0.0-20210720151324-cfbfc8feace0
	github.com/prometheus/client_golang v1.7.1
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	go.uber.org/zap v1.13.0 // indirect
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/apiserver v0.21.2
	k8s.io/client-go v0.21.2
	k8s.io/component-base v0.21.2
	k8s.io/controller-manager v0.21.2
	k8s.io/klog/v2 v2.9.0
	k8s.io/kubernetes v1.21.2
)

replace (
	k8s.io/api => k8s.io/api v0.21.2
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.21.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.2
	k8s.io/apiserver => k8s.io/apiserver v0.21.2
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.21.2
	k8s.io/client-go => k8s.io/client-go v0.21.2
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.21.2
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.21.2
	k8s.io/code-generator => k8s.io/code-generator v0.21.2
	k8s.io/component-base => k8s.io/component-base v0.21.2
	k8s.io/component-helpers => k8s.io/component-helpers v0.21.2
	k8s.io/controller-manager => k8s.io/controller-manager v0.21.2
	k8s.io/cri-api => k8s.io/cri-api v0.21.2
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.21.2
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.21.2
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.21.2
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.21.2
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.21.2
	k8s.io/kubectl => k8s.io/kubectl v0.21.2
	k8s.io/kubelet => k8s.io/kubelet v0.21.2
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.21.2
	k8s.io/metrics => k8s.io/metrics v0.21.2
	k8s.io/mount-utils => k8s.io/mount-utils v0.21.2
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.21.2
)
