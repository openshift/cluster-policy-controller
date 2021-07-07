package controller

import (
	"github.com/openshift/library-go/pkg/operator/csr"
	"github.com/openshift/library-go/pkg/operator/events"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

const (
	controllerName                    = "csr-approver-controller"
	monitoringServiceAccountNamespace = "openshift-monitoring"
	monitoringServiceAccountName      = "cluster-monitoring-operator"
	monitoringCertificateSubject      = "/CN=system:serviceaccount:openshift-monitoring:prometheus-k8s"
	monitoringLabelKey                = "metrics.openshift.io/csr.subject"
	monitoringLabelValue              = "prometheus"
)

func RunCSRApproverController(ctx *ControllerContext) (bool, error) {
	kubeClient, err := ctx.ClientBuilder.Client(infraClusterCSRApproverControllerServiceAccountName)
	if err != nil {
		return true, err
	}

	eventRecorder := events.NewRecorder(kubeClient.CoreV1().Events("test"), infraClusterCSRApproverControllerServiceAccountName, &v1.ObjectReference{})

	selector := labels.NewSelector()
	labelsRequirement, err := labels.NewRequirement(monitoringLabelKey, selection.Equals, []string{monitoringLabelValue})
	if err != nil {
		return true, err
	}
	selector = selector.Add(*labelsRequirement)

	controller := csr.NewCSRApproverController(
		controllerName,
		nil,
		kubeClient.CertificatesV1().CertificateSigningRequests(),
		ctx.KubernetesInformers.Certificates().V1().CertificateSigningRequests(),
		csr.NewLabelFilter(selector),
		csr.NewServiceAccountApprover(monitoringServiceAccountNamespace, monitoringServiceAccountName, monitoringCertificateSubject),
		eventRecorder)

	go controller.Run(ctx.Context, 1)

	return true, nil
}
