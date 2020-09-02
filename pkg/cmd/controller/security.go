package controller

import (
	"fmt"

	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"

	sccallocation "github.com/openshift/cluster-policy-controller/pkg/security/controller"
	"github.com/openshift/cluster-policy-controller/pkg/security/mcs"
	"github.com/openshift/library-go/pkg/security/uid"
)

func RunNamespaceSecurityAllocationController(ctx *ControllerContext) (bool, error) {
	uidRange, err := uid.ParseRange(ctx.OpenshiftControllerConfig.SecurityAllocator.UIDAllocatorRange)
	if err != nil {
		return true, fmt.Errorf("unable to describe UID range: %v", err)
	}
	mcsRange, err := mcs.ParseRange(ctx.OpenshiftControllerConfig.SecurityAllocator.MCSAllocatorRange)
	if err != nil {
		return true, fmt.Errorf("unable to describe MCS category range: %v", err)
	}

	kubeClient, err := ctx.ClientBuilder.Client(infraNamespaceSecurityAllocationControllerServiceAccountName)
	if err != nil {
		return true, err
	}
	securityClient, err := ctx.ClientBuilder.OpenshiftSecurityClient(infraNamespaceSecurityAllocationControllerServiceAccountName)
	if err != nil {
		return true, err
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	controller := sccallocation.NewNamespaceSCCAllocationController(
		ctx.KubernetesInformers.Core().V1().Namespaces(),
		kubeClient.CoreV1().Namespaces(),
		securityClient.SecurityV1(),
		uidRange,
		sccallocation.DefaultMCSAllocation(uidRange, mcsRange, ctx.OpenshiftControllerConfig.SecurityAllocator.MCSLabelsPerProject),
		eventBroadcaster,
	)
	go controller.Run(ctx.Stop)

	return true, nil
}
