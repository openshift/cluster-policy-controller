package controller

import (
	"math/rand"
	"time"

	"github.com/openshift/cluster-policy-controller/pkg/quota/clusterquotareconciliation"
	image "github.com/openshift/cluster-policy-controller/pkg/quota/quotaimageexternal"
	"github.com/openshift/library-go/pkg/quota/clusterquotamapping"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/controller"
	kresourcequota "k8s.io/kubernetes/pkg/controller/resourcequota"
	kquota "k8s.io/kubernetes/pkg/quota/v1"
	"k8s.io/kubernetes/pkg/quota/v1/generic"
	quotainstall "k8s.io/kubernetes/pkg/quota/v1/install"
)

func RunResourceQuotaManager(ctx *ControllerContext) (bool, error) {
	concurrentResourceQuotaSyncs := int(ctx.OpenshiftControllerConfig.ResourceQuota.ConcurrentSyncs)
	resourceQuotaSyncPeriod := ctx.OpenshiftControllerConfig.ResourceQuota.SyncPeriod.Duration
	replenishmentSyncPeriodFunc := calculateResyncPeriod(ctx.OpenshiftControllerConfig.ResourceQuota.MinResyncPeriod.Duration)
	saName := "resourcequota-controller"
	listerFuncForResource := generic.ListerFuncForResourceFunc(ctx.GenericResourceInformer.ForResource)
	quotaConfiguration := quotainstall.NewQuotaConfigurationForControllers(listerFuncForResource)
	resourceQuotaControllerClient := ctx.ClientBuilder.ClientOrDie(saName)
	discoveryFunc := resourceQuotaControllerClient.Discovery().ServerPreferredNamespacedResources

	imageEvaluators := image.NewReplenishmentEvaluators(
		listerFuncForResource,
		ctx.ImageInformers.Image().V1().ImageStreams(),
		ctx.ClientBuilder.OpenshiftImageClientOrDie(saName).ImageV1())
	resourceQuotaRegistry := generic.NewRegistry(imageEvaluators)

	resourceQuotaControllerOptions := &kresourcequota.ResourceQuotaControllerOptions{
		QuotaClient:               resourceQuotaControllerClient.CoreV1(),
		ResourceQuotaInformer:     ctx.KubernetesInformers.Core().V1().ResourceQuotas(),
		ResyncPeriod:              controller.StaticResyncPeriodFunc(resourceQuotaSyncPeriod),
		Registry:                  resourceQuotaRegistry,
		ReplenishmentResyncPeriod: replenishmentSyncPeriodFunc,
		IgnoredResourcesFunc:      quotaConfiguration.IgnoredResources,
		InformersStarted:          ctx.InformersStarted,
		InformerFactory:           ctx.GenericResourceInformer,
		DiscoveryFunc:             resourceQuotaDiscoveryWrapper(resourceQuotaRegistry, discoveryFunc),
	}
	controller, err := kresourcequota.NewResourceQuotaController(resourceQuotaControllerOptions)
	if err != nil {
		return true, err
	}
	go controller.Run(concurrentResourceQuotaSyncs, ctx.Stop)

	return true, nil
}

func resourceQuotaDiscoveryWrapper(registry kquota.Registry, discoveryFunc kresourcequota.NamespacedResourcesFunc) kresourcequota.NamespacedResourcesFunc {
	return func() ([]*metav1.APIResourceList, error) {
		discoveryResources, discoveryErr := discoveryFunc()
		if discoveryErr != nil && len(discoveryResources) == 0 {
			return nil, discoveryErr
		}

		interestingResources := []*metav1.APIResourceList{}
		for _, resourceList := range discoveryResources {
			gv, err := schema.ParseGroupVersion(resourceList.GroupVersion)
			if err != nil {
				return nil, err
			}
			for i := range resourceList.APIResources {
				gr := schema.GroupResource{
					Group:    gv.Group,
					Resource: resourceList.APIResources[i].Name,
				}
				if evaluator := registry.Get(gr); evaluator != nil {
					interestingResources = append(interestingResources, resourceList)
				}
			}
		}
		return interestingResources, nil
	}
}

func RunClusterQuotaReconciliationController(ctx *ControllerContext) (bool, error) {
	defaultResyncPeriod := 5 * time.Minute
	defaultReplenishmentSyncPeriod := 12 * time.Hour

	saName := infraClusterQuotaReconciliationControllerServiceAccountName

	clusterQuotaMappingController := clusterquotamapping.NewClusterQuotaMappingController(
		ctx.KubernetesInformers.Core().V1().Namespaces(),
		ctx.QuotaInformers.Quota().V1().ClusterResourceQuotas())
	resourceQuotaControllerClient := ctx.ClientBuilder.ClientOrDie("resourcequota-controller")
	discoveryFunc := resourceQuotaControllerClient.Discovery().ServerPreferredNamespacedResources
	listerFuncForResource := generic.ListerFuncForResourceFunc(ctx.GenericResourceInformer.ForResource)
	quotaConfiguration := quotainstall.NewQuotaConfigurationForControllers(listerFuncForResource)

	// TODO make a union registry
	resourceQuotaRegistry := generic.NewRegistry(quotaConfiguration.Evaluators())
	imageEvaluators := image.NewReplenishmentEvaluators(
		listerFuncForResource,
		ctx.ImageInformers.Image().V1().ImageStreams(),
		ctx.ClientBuilder.OpenshiftImageClientOrDie(saName).ImageV1())
	for i := range imageEvaluators {
		resourceQuotaRegistry.Add(imageEvaluators[i])
	}

	options := clusterquotareconciliation.ClusterQuotaReconcilationControllerOptions{
		ClusterQuotaInformer: ctx.QuotaInformers.Quota().V1().ClusterResourceQuotas(),
		ClusterQuotaMapper:   clusterQuotaMappingController.GetClusterQuotaMapper(),
		ClusterQuotaClient:   ctx.ClientBuilder.OpenshiftQuotaClientOrDie(saName).QuotaV1().ClusterResourceQuotas(),

		Registry:                  resourceQuotaRegistry,
		ResyncPeriod:              defaultResyncPeriod,
		ReplenishmentResyncPeriod: controller.StaticResyncPeriodFunc(defaultReplenishmentSyncPeriod),
		DiscoveryFunc:             discoveryFunc,
		IgnoredResourcesFunc:      quotaConfiguration.IgnoredResources,
		InformersStarted:          ctx.InformersStarted,
		InformerFactory:           ctx.GenericResourceInformer,
	}
	clusterQuotaReconciliationController, err := clusterquotareconciliation.NewClusterQuotaReconcilationController(options)
	if err != nil {
		return true, err
	}
	clusterQuotaMappingController.GetClusterQuotaMapper().AddListener(clusterQuotaReconciliationController)

	go clusterQuotaMappingController.Run(5, ctx.Stop)
	go clusterQuotaReconciliationController.Run(5, ctx.Stop)

	return true, nil
}

func calculateResyncPeriod(period time.Duration) func() time.Duration {
	return func() time.Duration {
		factor := rand.Float64() + 1
		return time.Duration(float64(period.Nanoseconds()) * factor)
	}
}
