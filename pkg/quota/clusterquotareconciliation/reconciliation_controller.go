package clusterquotareconciliation

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"k8s.io/klog/v2"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	utilquota "k8s.io/apiserver/pkg/quota/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/cache"
	"k8s.io/controller-manager/pkg/informerfactory"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/resourcequota"

	quotav1 "github.com/openshift/api/quota/v1"
	quotatypedclient "github.com/openshift/client-go/quota/clientset/versioned/typed/quota/v1"
	quotainformer "github.com/openshift/client-go/quota/informers/externalversions/quota/v1"
	quotalister "github.com/openshift/client-go/quota/listers/quota/v1"
	"github.com/openshift/library-go/pkg/quota/clusterquotamapping"
	quotautil "github.com/openshift/library-go/pkg/quota/quotautil"
)

type ClusterQuotaReconcilationControllerOptions struct {
	ClusterQuotaInformer quotainformer.ClusterResourceQuotaInformer
	ClusterQuotaMapper   clusterquotamapping.ClusterQuotaMapper
	ClusterQuotaClient   quotatypedclient.ClusterResourceQuotaInterface

	// Knows how to calculate usage
	Registry utilquota.Registry
	// Controls full recalculation of quota usage
	ResyncPeriod time.Duration
	// Discover list of supported resources on the server.
	DiscoveryFunc resourcequota.NamespacedResourcesFunc
	// A function that returns the list of resources to ignore
	IgnoredResourcesFunc func() map[schema.GroupResource]struct{}
	// InformersStarted knows if informers were started.
	InformersStarted <-chan struct{}
	// InformerFactory interfaces with informers.
	InformerFactory informerfactory.InformerFactory
	// Controls full resync of objects monitored for replenihsment.
	ReplenishmentResyncPeriod controller.ResyncPeriodFunc
}

type ClusterQuotaReconcilationController struct {
	clusterQuotaLister quotalister.ClusterResourceQuotaLister
	clusterQuotaMapper clusterquotamapping.ClusterQuotaMapper
	clusterQuotaClient quotatypedclient.ClusterResourceQuotaInterface
	// A list of functions that return true when their caches have synced
	informerSyncedFuncs []cache.InformerSynced

	resyncPeriod time.Duration

	// queue tracks which clusterquotas to update along with a list of namespaces for that clusterquota
	queue BucketingWorkQueue

	// knows how to calculate usage
	registry utilquota.Registry
	// knows how to monitor all the resources tracked by quota and trigger replenishment
	quotaMonitor *resourcequota.QuotaMonitor
	// controls the workers that process quotas
	// this lock is acquired to control write access to the monitors and ensures that all
	// monitors are synced before the controller can process quotas.
	workerLock sync.RWMutex
}

type workItem struct {
	namespaceName      string
	forceRecalculation bool
}

func NewClusterQuotaReconcilationController(options ClusterQuotaReconcilationControllerOptions) (*ClusterQuotaReconcilationController, error) {
	c := &ClusterQuotaReconcilationController{
		clusterQuotaLister:  options.ClusterQuotaInformer.Lister(),
		clusterQuotaMapper:  options.ClusterQuotaMapper,
		clusterQuotaClient:  options.ClusterQuotaClient,
		informerSyncedFuncs: []cache.InformerSynced{options.ClusterQuotaInformer.Informer().HasSynced},

		resyncPeriod: options.ResyncPeriod,
		registry:     options.Registry,

		queue: NewBucketingWorkQueue("controller_clusterquotareconcilationcontroller"),
	}

	// we need to trigger every time
	options.ClusterQuotaInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addClusterQuota,
		UpdateFunc: c.updateClusterQuota,
	})

	qm := resourcequota.NewMonitor(
		options.InformersStarted,
		options.InformerFactory,
		options.IgnoredResourcesFunc(),
		options.ReplenishmentResyncPeriod,
		c.replenishQuota,
		c.registry,
	)

	c.quotaMonitor = qm

	// do initial quota monitor setup.  If we have a discovery failure here, it's ok. We'll discover more resources when a later sync happens.
	resources, err := resourcequota.GetQuotableResources(options.DiscoveryFunc)
	if discovery.IsGroupDiscoveryFailedError(err) {
		utilruntime.HandleError(fmt.Errorf("initial discovery check failure, continuing and counting on future sync update: %v", err))
	} else if err != nil {
		return nil, err
	}

	if err = qm.SyncMonitors(resources); err != nil {
		utilruntime.HandleError(fmt.Errorf("initial monitor sync has error: %v", err))
	}

	// only start quota once all informers synced
	c.informerSyncedFuncs = append(c.informerSyncedFuncs, qm.IsSynced)

	return c, nil
}

// Run begins quota controller using the specified number of workers
func (c *ClusterQuotaReconcilationController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	klog.Infof("Starting the cluster quota reconciliation controller")

	// the controllers that replenish other resources to respond rapidly to state changes
	go c.quotaMonitor.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.informerSyncedFuncs...) {
		return
	}

	// the workers that chug through the quota calculation backlog
	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	// the timer for how often we do a full recalculation across all quotas
	go wait.Until(func() { c.calculateAll() }, c.resyncPeriod, stopCh)

	<-stopCh
	klog.Infof("Shutting down ClusterQuotaReconcilationController")
	c.queue.ShutDown()
}

// Sync periodically resyncs the controller when new resources are observed from discovery.
func (c *ClusterQuotaReconcilationController) Sync(discoveryFunc resourcequota.NamespacedResourcesFunc, period time.Duration, stopCh <-chan struct{}) {
	klog.Infof("Running ClusterQuotaReconcilationController.Sync")
	// Something has changed, so track the new state and perform a sync.
	oldResources := make(map[schema.GroupVersionResource]struct{})
	wait.Until(func() {
		for k, v := range oldResources {
			klog.Infof("oldResources[%v]=%v", k, v)
		}
		// Get the current resource list from discovery.
		klog.Infof("Running ClusterQuotaReconcilationController.Sync:resourcequota.GetQuotableResources")
		newResources, err := resourcequota.GetQuotableResources(discoveryFunc)
		klog.Infof("Running ClusterQuotaReconcilationController.Sync:resourcequota.GetQuotableResources.err=%v", err)
		if err != nil {
			utilruntime.HandleError(err)

			klog.Infof("Running ClusterQuotaReconcilationController.Sync: len(newResources)=%v", len(newResources))
			if discovery.IsGroupDiscoveryFailedError(err) && len(newResources) > 0 {
				// In partial discovery cases, don't remove any existing informers, just add new ones
				for k, v := range oldResources {
					klog.Infof("oldResources -> newResources[%v]=%v", k, v)
					newResources[k] = v
				}
			} else {
				// short circuit in non-discovery error cases or if discovery returned zero resources
				klog.Infof("Running ClusterQuotaReconcilationController.Sync: short circuit in non-discovery error cases or if discovery returned zero resources")
				return
			}
		}

		// Decide whether discovery has reported a change.
		if reflect.DeepEqual(oldResources, newResources) {
			klog.Infof("no resource updates from discovery, skipping resource quota sync")
			klog.V(4).Infof("no resource updates from discovery, skipping resource quota sync")
			return
		}

		// Something has changed, so track the new state and perform a sync.
		klog.V(2).Infof("syncing resource quota controller with updated resources from discovery: %v", newResources)
		oldResources = newResources

		// Ensure workers are paused to avoid processing events before informers
		// have resynced.
		c.workerLock.Lock()
		defer c.workerLock.Unlock()

		// Perform the monitor resync and wait for controllers to report cache sync.
		if err := c.resyncMonitors(newResources); err != nil {
			klog.Infof("failed to sync resource monitors: %v", err)
			utilruntime.HandleError(fmt.Errorf("failed to sync resource monitors: %v", err))
			return
		}
		if c.quotaMonitor != nil && !cache.WaitForCacheSync(stopCh, c.quotaMonitor.IsSynced) {
			klog.Infof("timed out waiting for quota monitor sync")
			utilruntime.HandleError(fmt.Errorf("timed out waiting for quota monitor sync"))
		}
	}, period, stopCh)
}

// resyncMonitors starts or stops quota monitors as needed to ensure that all
// (and only) those resources present in the map are monitored.
func (c *ClusterQuotaReconcilationController) resyncMonitors(resources map[schema.GroupVersionResource]struct{}) error {
	klog.Infof("Running ClusterQuotaReconcilationController.resyncMonitors")
	// SyncMonitors can only fail if there was no Informer for the given gvr
	err := c.quotaMonitor.SyncMonitors(resources)
	// this is no-op for already running monitors
	c.quotaMonitor.StartMonitors()
	klog.Infof("Running ClusterQuotaReconcilationController.resyncMonitors: err=%v", err)
	return err
}

func (c *ClusterQuotaReconcilationController) calculate(quotaName string, namespaceNames ...string) {
	klog.Infof("Running ClusterQuotaReconcilationController.calculate")
	if len(namespaceNames) == 0 {
		return
	}
	items := make([]interface{}, 0, len(namespaceNames))
	for _, name := range namespaceNames {
		items = append(items, workItem{namespaceName: name, forceRecalculation: false})
	}

	klog.Infof("Running ClusterQuotaReconcilationController.calculate: quotaName=%v, items=%v", quotaName, items)
	c.queue.AddWithData(quotaName, items...)
}

func (c *ClusterQuotaReconcilationController) forceCalculation(quotaName string, namespaceNames ...string) {
	klog.Infof("Running ClusterQuotaReconcilationController.forceCalculation")
	if len(namespaceNames) == 0 {
		return
	}
	items := make([]interface{}, 0, len(namespaceNames))
	for _, name := range namespaceNames {
		items = append(items, workItem{namespaceName: name, forceRecalculation: true})
	}

	klog.Infof("Running ClusterQuotaReconcilationController.forceCalculation: quotaName=%v, items=%v", quotaName, items)
	c.queue.AddWithData(quotaName, items...)
}

func (c *ClusterQuotaReconcilationController) calculateAll() {
	klog.Infof("Running ClusterQuotaReconcilationController.calculateAll")
	quotas, err := c.clusterQuotaLister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	klog.Infof("Listing %q quota objects", len(quotas))
	for _, quota := range quotas {
		// If we have namespaces we map to, force calculating those namespaces
		namespaces, _ := c.clusterQuotaMapper.GetNamespacesFor(quota.Name)
		klog.Infof("Namespaces for quota %q: %v", quota, namespaces)
		if len(namespaces) > 0 {
			c.forceCalculation(quota.Name, namespaces...)
			continue
		}

		// If the quota status has namespaces when our mapper doesn't think it should,
		// add it directly to the queue without any work items
		klog.Infof("Namespaces in status for quota %q: %v", quota, quota.Status.Namespaces)
		if len(quota.Status.Namespaces) > 0 {
			c.queue.AddWithData(quota.Name)
			continue
		}
	}
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (c *ClusterQuotaReconcilationController) worker() {
	workFunc := func() bool {
		klog.Infof("ClusterQuotaReconcilationController.worker(): picking up with data item")
		uncastKey, uncastData, quit := c.queue.GetWithData()
		klog.Infof("ClusterQuotaReconcilationController.worker(): uncastKey=%v, uncastData=%v, quit=%v", uncastKey, uncastData, quit)
		if quit {
			return true
		}
		defer c.queue.Done(uncastKey)

		c.workerLock.RLock()
		defer c.workerLock.RUnlock()

		quotaName := uncastKey.(string)
		klog.Infof("ClusterQuotaReconcilationController.worker(): quotaName=%v", quotaName)
		quota, err := c.clusterQuotaLister.Get(quotaName)
		klog.Infof("ClusterQuotaReconcilationController.worker(): quotaName=%v err=%v, quota=%v", quotaName, err, quota)
		if apierrors.IsNotFound(err) {
			c.queue.Forget(uncastKey)
			return false
		}
		if err != nil {
			utilruntime.HandleError(err)
			klog.Infof("ClusterQuotaReconcilationController.worker(): AddWithDataRateLimited, uncastKey=%v, uncastData=%v", uncastKey, uncastData)
			c.queue.AddWithDataRateLimited(uncastKey, uncastData...)
			return false
		}

		workItems := make([]workItem, 0, len(uncastData))
		for _, dataElement := range uncastData {
			workItems = append(workItems, dataElement.(workItem))
		}
		klog.Infof("ClusterQuotaReconcilationController.worker(): workItems=%v", workItems)
		err, retryItems := c.syncQuotaForNamespaces(quota, workItems)
		klog.Infof("ClusterQuotaReconcilationController.worker().syncQuotaForNamespaces: workItems=%v, quota=%v, err=%v, retryItems=%v", workItems, quota, err, retryItems)
		if err == nil {
			c.queue.Forget(uncastKey)
			return false
		}
		utilruntime.HandleError(err)

		items := make([]interface{}, 0, len(retryItems))
		for _, item := range retryItems {
			items = append(items, item)
		}
		klog.Infof("ClusterQuotaReconcilationController.worker().AddWithDataRateLimited: uncastKey=%v, items=%v", uncastKey, items)
		c.queue.AddWithDataRateLimited(uncastKey, items...)
		return false
	}

	for {
		if quit := workFunc(); quit {
			klog.Infof("resource quota controller worker shutting down")
			return
		}
	}
}

// syncResourceQuotaFromKey syncs a quota key
func (c *ClusterQuotaReconcilationController) syncQuotaForNamespaces(originalQuota *quotav1.ClusterResourceQuota, workItems []workItem) (error, []workItem /* to retry */) {
	klog.Infof("Running ClusterQuotaReconcilationController.syncQuotaForNamespaces")
	quota := originalQuota.DeepCopy()

	// get the list of namespaces that match this cluster quota
	matchingNamespaceNamesList, quotaSelector := c.clusterQuotaMapper.GetNamespacesFor(quota.Name)
	klog.Infof("Running ClusterQuotaReconcilationController.syncQuotaForNamespaces: matchingNamespaceNamesList=%v, quotaSelector=%v", matchingNamespaceNamesList, quotaSelector)
	if !equality.Semantic.DeepEqual(quotaSelector, quota.Spec.Selector) {
		klog.Infof("mapping not up to date, have=%v need=%v", quotaSelector, quota.Spec.Selector)
		return fmt.Errorf("mapping not up to date, have=%v need=%v", quotaSelector, quota.Spec.Selector), workItems
	}
	matchingNamespaceNames := sets.NewString(matchingNamespaceNamesList...)
	klog.Infof("Running ClusterQuotaReconcilationController.syncQuotaForNamespaces: matchingNamespaceNames=%v", matchingNamespaceNames)
	reconcilationErrors := []error{}
	retryItems := []workItem{}
	for _, item := range workItems {
		klog.Infof("Running ClusterQuotaReconcilationController.syncQuotaForNamespaces: item=%v", item)
		namespaceName := item.namespaceName
		namespaceTotals, namespaceLoaded := quotautil.GetResourceQuotasStatusByNamespace(quota.Status.Namespaces, namespaceName)
		klog.Infof("Running ClusterQuotaReconcilationController.syncQuotaForNamespaces: namespaceTotals=%v, namespaceLoaded=%v", namespaceTotals, namespaceLoaded)
		if !matchingNamespaceNames.Has(namespaceName) {
			if namespaceLoaded {
				// remove this item from all totals
				quota.Status.Total.Used = utilquota.Subtract(quota.Status.Total.Used, namespaceTotals.Used)
				klog.Infof("Running ClusterQuotaReconcilationController.syncQuotaForNamespaces: quota.Status.Total.Used=%v", quota.Status.Total.Used)
				quotautil.RemoveResourceQuotasStatusByNamespace(&quota.Status.Namespaces, namespaceName)
			}
			continue
		}

		// if there's no work for us to do, do nothing
		if !item.forceRecalculation && namespaceLoaded && equality.Semantic.DeepEqual(namespaceTotals.Hard, quota.Spec.Quota.Hard) {
			klog.Infof("Running ClusterQuotaReconcilationController.syncQuotaForNamespaces: if there's no work for us to do, do nothing")
			continue
		}

		actualUsage, err := quotaUsageCalculationFunc(namespaceName, quota.Spec.Quota.Scopes, quota.Spec.Quota.Hard, c.registry, quota.Spec.Quota.ScopeSelector)
		klog.Infof("Running ClusterQuotaReconcilationController.syncQuotaForNamespaces: quotaUsageCalculationFunc, err=%v", err)
		if err != nil {
			// tally up errors, but calculate everything you can
			reconcilationErrors = append(reconcilationErrors, err)
			retryItems = append(retryItems, item)
			continue
		}
		recalculatedStatus := corev1.ResourceQuotaStatus{
			Used: actualUsage,
			Hard: quota.Spec.Quota.Hard,
		}

		// subtract old usage, add new usage
		quota.Status.Total.Used = utilquota.Subtract(quota.Status.Total.Used, namespaceTotals.Used)
		quota.Status.Total.Used = utilquota.Add(quota.Status.Total.Used, recalculatedStatus.Used)
		quotautil.InsertResourceQuotasStatus(&quota.Status.Namespaces, quotav1.ResourceQuotaStatusByNamespace{
			Namespace: namespaceName,
			Status:    recalculatedStatus,
		})
	}

	// Remove any namespaces from quota.status that no longer match.
	// Needed because we will never get workitems for namespaces that no longer exist if we missed the delete event (e.g. on startup)
	// range on a copy so that we don't mutate our original
	statusCopy := quota.Status.Namespaces.DeepCopy()
	klog.Infof("Running ClusterQuotaReconcilationController.syncQuotaForNamespaces: statusCopy=%v", statusCopy)
	for _, namespaceTotals := range statusCopy {
		namespaceName := namespaceTotals.Namespace
		klog.Infof("Running ClusterQuotaReconcilationController.syncQuotaForNamespaces: namespaceName=%v, matchingNamespaceNames=%v", namespaceName, matchingNamespaceNames)
		if !matchingNamespaceNames.Has(namespaceName) {
			quota.Status.Total.Used = utilquota.Subtract(quota.Status.Total.Used, namespaceTotals.Status.Used)
			quotautil.RemoveResourceQuotasStatusByNamespace(&quota.Status.Namespaces, namespaceName)
		}
	}

	quota.Status.Total.Hard = quota.Spec.Quota.Hard

	// if there's no change, no update, return early.  NewAggregate returns nil on empty input
	if equality.Semantic.DeepEqual(quota, originalQuota) {
		klog.Infof("Running ClusterQuotaReconcilationController.syncQuotaForNamespaces: no change, no update, return early, quota=%v", quota)
		return kutilerrors.NewAggregate(reconcilationErrors), retryItems
	}

	if _, err := c.clusterQuotaClient.UpdateStatus(context.TODO(), quota, metav1.UpdateOptions{}); err != nil {
		klog.Infof("Running ClusterQuotaReconcilationController.syncQuotaForNamespaces: c.clusterQuotaClient.UpdateStatus, err=%v", err)
		return kutilerrors.NewAggregate(append(reconcilationErrors, err)), workItems
	}

	return kutilerrors.NewAggregate(reconcilationErrors), retryItems
}

// replenishQuota is a replenishment function invoked by a controller to notify that a quota should be recalculated
func (c *ClusterQuotaReconcilationController) replenishQuota(groupResource schema.GroupResource, namespace string) {
	klog.Infof("Running ClusterQuotaReconcilationController.replenishQuota")
	// check if the quota controller can evaluate this kind, if not, ignore it altogether...
	releventEvaluators := []utilquota.Evaluator{}
	evaluators := c.registry.List()
	for i := range evaluators {
		evaluator := evaluators[i]
		klog.Infof("Running ClusterQuotaReconcilationController.replenishQuota: evaluator=%v", evaluator)
		if evaluator.GroupResource() == groupResource {
			releventEvaluators = append(releventEvaluators, evaluator)
		}
	}
	klog.Infof("Running ClusterQuotaReconcilationController.replenishQuota: releventEvaluators=%v", releventEvaluators)
	if len(releventEvaluators) == 0 {
		return
	}

	quotaNames, _ := c.clusterQuotaMapper.GetClusterQuotasFor(namespace)
	klog.Infof("Running ClusterQuotaReconcilationController.replenishQuota: GetClusterQuotasFor, quotaNames=%v", quotaNames)

	// only queue those quotas that are tracking a resource associated with this kind.
	for _, quotaName := range quotaNames {
		klog.Infof("Running ClusterQuotaReconcilationController.replenishQuota: GetClusterQuotasFor, quotaName=%v", quotaName)
		quota, err := c.clusterQuotaLister.Get(quotaName)
		klog.Infof("Running ClusterQuotaReconcilationController.replenishQuota: GetClusterQuotasFor, c.clusterQuotaLister.Get, err=%v", err)
		if err != nil {
			// replenishment will be delayed, but we'll get back around to it later if it matters
			continue
		}

		resourceQuotaResources := utilquota.ResourceNames(quota.Status.Total.Hard)
		for _, evaluator := range releventEvaluators {
			matchedResources := evaluator.MatchingResources(resourceQuotaResources)
			klog.Infof("Running ClusterQuotaReconcilationController.replenishQuota: evaluator.MatchingResources, evaluator=%v, matchedResources=%v", evaluator, matchedResources)
			if len(matchedResources) > 0 {
				// TODO: make this support targeted replenishment to a specific kind, right now it does a full recalc on that quota.
				c.forceCalculation(quotaName, namespace)
				break
			}
		}
	}
}

func (c *ClusterQuotaReconcilationController) addClusterQuota(cur interface{}) {
	c.enqueueClusterQuota(cur)
}
func (c *ClusterQuotaReconcilationController) updateClusterQuota(old, cur interface{}) {
	c.enqueueClusterQuota(cur)
}
func (c *ClusterQuotaReconcilationController) enqueueClusterQuota(obj interface{}) {
	quota, ok := obj.(*quotav1.ClusterResourceQuota)
	if !ok {
		utilruntime.HandleError(fmt.Errorf("not a ClusterResourceQuota %v", obj))
		return
	}

	namespaces, _ := c.clusterQuotaMapper.GetNamespacesFor(quota.Name)
	c.calculate(quota.Name, namespaces...)
}

func (c *ClusterQuotaReconcilationController) AddMapping(quotaName, namespaceName string) {
	c.calculate(quotaName, namespaceName)

}
func (c *ClusterQuotaReconcilationController) RemoveMapping(quotaName, namespaceName string) {
	c.calculate(quotaName, namespaceName)
}

// quotaUsageCalculationFunc is a function to calculate quota usage.  It is only configurable for easy unit testing
// NEVER CHANGE THIS OUTSIDE A TEST
var quotaUsageCalculationFunc = utilquota.CalculateUsage
