FROM registry.svc.ci.openshift.org/ocp/builder:golang-1.12 AS builder
WORKDIR /go/src/github.com/openshift/cluster-policy-controller
RUN yum install -y gpgme-devel libassuan-devel
COPY . .
RUN make build --warn-undefined-variables

FROM registry.svc.ci.openshift.org/openshift/origin-v4.0:base
COPY --from=builder /go/src/github.com/openshift/cluster-policy-controller/cluster-policy-controller /usr/bin/
LABEL io.k8s.display-name="OpenShift Cluster Policy Controller Command" \
      io.k8s.description="OpenShift is a platform for developing, building, and deploying containerized applications." \
      io.openshift.tags="openshift,cluster-policy-controller"
