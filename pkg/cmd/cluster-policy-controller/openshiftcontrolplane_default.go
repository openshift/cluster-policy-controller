package cluster_policy_controller

import (
	"time"

	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	"github.com/openshift/library-go/pkg/config/configdefaults"
	"github.com/openshift/library-go/pkg/config/helpers"
	leaderelectionconverter "github.com/openshift/library-go/pkg/config/leaderelection"
)

func setRecommendedOpenShiftControllerConfigDefaults(config *openshiftcontrolplanev1.OpenShiftControllerManagerConfig) {
	configdefaults.SetRecommendedHTTPServingInfoDefaults(config.ServingInfo)
	configdefaults.SetRecommendedKubeClientConfigDefaults(&config.KubeClientConfig)
	config.LeaderElection = leaderelectionconverter.LeaderElectionDefaulting(config.LeaderElection, "openshift-kube-controller-manager", "cluster-policy-controller")

	configdefaults.DefaultStringSlice(&config.Controllers, []string{"*"})

	configdefaults.DefaultString(&config.SecurityAllocator.UIDAllocatorRange, "1000000000-1999999999/10000")
	configdefaults.DefaultString(&config.SecurityAllocator.MCSAllocatorRange, "s0:/2")
	if config.SecurityAllocator.MCSLabelsPerProject == 0 {
		config.SecurityAllocator.MCSLabelsPerProject = 5
	}

	if config.ResourceQuota.MinResyncPeriod.Duration == 0 {
		config.ResourceQuota.MinResyncPeriod.Duration = 5 * time.Minute
	}
	if config.ResourceQuota.SyncPeriod.Duration == 0 {
		config.ResourceQuota.SyncPeriod.Duration = 12 * time.Hour
	}
	if config.ResourceQuota.ConcurrentSyncs == 0 {
		config.ResourceQuota.ConcurrentSyncs = 5
	}
}

func getOpenShiftControllerConfigFileReferences(config *openshiftcontrolplanev1.OpenShiftControllerManagerConfig) []*string {
	if config == nil {
		return []*string{}
	}

	refs := []*string{}

	refs = append(refs, helpers.GetHTTPServingInfoFileReferences(config.ServingInfo)...)
	refs = append(refs, helpers.GetKubeClientConfigFileReferences(&config.KubeClientConfig)...)

	return refs
}
