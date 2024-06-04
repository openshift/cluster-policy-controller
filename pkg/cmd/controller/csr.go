package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/openshift/library-go/pkg/operator/csr"
)

const (
	controllerNamePrefix                  = "csr-approver-controller"
	monitoringNamespace                   = "openshift-monitoring"
	monitoringRequesterServiceAccountName = "cluster-monitoring-operator"
	monitoringSubjectNameLabelKey         = "metrics.openshift.io/csr.subject"
)

type monitoringCSR struct {
	subject     string
	certSubject string
	selector    labels.Selector
}

func newMonitoringCSR(subjectName, subjectServiceAccount string) (*monitoringCSR, error) {
	labelsRequirement, err := labels.NewRequirement(monitoringSubjectNameLabelKey, selection.Equals, []string{subjectName})
	if err != nil {
		return nil, err
	}
	return &monitoringCSR{
		subject:     subjectName,
		certSubject: fmt.Sprintf("CN=system:serviceaccount:%s:%s", monitoringNamespace, subjectServiceAccount),
		selector:    labels.NewSelector().Add(*labelsRequirement),
	}, nil
}

// RunCSRApproverController approves the CSRs for prometheus and metrics-server client certs that are used for authn against various /metrics endpoints.
func RunCSRApproverController(ctx context.Context, controllerCtx *EnhancedControllerContext) (bool, error) {
	kubeClient, err := controllerCtx.ClientBuilder.Client(infraClusterCSRApproverControllerServiceAccountName)
	if err != nil {
		return true, err
	}

	csrs := []*monitoringCSR{}
	var curr *monitoringCSR

	prometheus := "prometheus"
	// Ideally, these two should be the same.
	curr, err = newMonitoringCSR(prometheus, fmt.Sprintf("%s-k8s", prometheus))
	if err != nil {
		return true, err
	}
	csrs = append(csrs, curr)

	metricsServer := "metrics-server"
	curr, err = newMonitoringCSR(metricsServer, metricsServer)
	if err != nil {
		return true, err
	}
	csrs = append(csrs, curr)

	for _, c := range csrs {
		go csr.NewCSRApproverController(
			fmt.Sprintf("%s-%s", controllerNamePrefix, c.subject),
			nil,
			kubeClient.CertificatesV1().CertificateSigningRequests(),
			controllerCtx.KubernetesInformers.Certificates().V1().CertificateSigningRequests(),
			csr.NewLabelFilter(c.selector),
			csr.NewServiceAccountApprover(monitoringNamespace, monitoringRequesterServiceAccountName, c.certSubject),
			controllerCtx.EventRecorder,
		).Run(ctx, 1)
	}

	return true, nil
}
