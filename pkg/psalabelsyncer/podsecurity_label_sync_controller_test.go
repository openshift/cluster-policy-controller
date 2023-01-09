package psalabelsyncer

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	rbacv1informers "k8s.io/client-go/informers/rbac/v1"
	fake "k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	psapi "k8s.io/pod-security-admission/api"

	securityv1 "github.com/openshift/api/security/v1"
	securityv1informers "github.com/openshift/client-go/security/informers/externalversions/security/v1"
	securityv1listers "github.com/openshift/client-go/security/listers/security/v1"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
)

var (
	testNamespaces = []*corev1.Namespace{
		{ObjectMeta: metav1.ObjectMeta{Name: "controlled-namespace", Labels: map[string]string{"security.openshift.io/scc.podSecurityLabelSync": "true"}, Annotations: map[string]string{securityv1.UIDRangeAnnotation: "1000/1052"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "controlled-namespace-too", Annotations: map[string]string{securityv1.UIDRangeAnnotation: "1000/1050"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "controlled-namespace-terminating", Annotations: map[string]string{securityv1.UIDRangeAnnotation: "1000/1052"}}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceTerminating}},
		{ObjectMeta: metav1.ObjectMeta{Name: "controlled-namespace-without-uid-annotation"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "controlled-namespace-previous-enforce-labels", Annotations: map[string]string{securityv1.UIDRangeAnnotation: "1000/1052"}, Labels: map[string]string{psapi.EnforceLevelLabel: "bogus value", psapi.EnforceVersionLabel: "bogus version value"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "controlled-namespace-previous-warn-labels", Annotations: map[string]string{securityv1.UIDRangeAnnotation: "1000/1052"}, Labels: map[string]string{psapi.WarnLevelLabel: "bogus value", psapi.WarnVersionLabel: "bogus version value"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "non-controlled-namespace", Labels: map[string]string{"security.openshift.io/scc.podSecurityLabelSync": "false"}, Annotations: map[string]string{securityv1.UIDRangeAnnotation: "1000/1052"}}},
	}
)

func syncSCCLister(t *testing.T) securityv1listers.SecurityContextConstraintsLister {
	pBool := func(b bool) *bool {
		return &b
	}

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, scc := range []*securityv1.SecurityContextConstraints{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "scc_restricted",
			},
			AllowHostDirVolumePlugin: false,
			AllowHostIPC:             false,
			AllowHostNetwork:         false,
			AllowHostPID:             false,
			AllowHostPorts:           false,
			AllowPrivilegeEscalation: pBool(false),
			AllowPrivilegedContainer: false,
			FSGroup:                  securityv1.FSGroupStrategyOptions{Type: securityv1.FSGroupStrategyMustRunAs},
			ReadOnlyRootFilesystem:   false,
			RequiredDropCapabilities: []corev1.Capability{"ALL"},
			RunAsUser:                securityv1.RunAsUserStrategyOptions{Type: securityv1.RunAsUserStrategyMustRunAsRange},
			SELinuxContext:           securityv1.SELinuxContextStrategyOptions{Type: securityv1.SELinuxStrategyMustRunAs},
			SeccompProfiles:          []string{"runtime/default"},
			SupplementalGroups:       securityv1.SupplementalGroupsStrategyOptions{Type: securityv1.SupplementalGroupsStrategyRunAsAny},
			Volumes:                  []securityv1.FSType{securityv1.FSTypeConfigMap, securityv1.FSTypeDownwardAPI, securityv1.FSTypeEmptyDir, securityv1.FSTypePersistentVolumeClaim, securityv1.FSProjected, securityv1.FSTypeSecret},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "scc_baseline",
			},
			AllowHostDirVolumePlugin: false,
			AllowHostIPC:             false,
			AllowHostNetwork:         false,
			AllowHostPID:             false,
			AllowHostPorts:           false,
			AllowPrivilegeEscalation: pBool(true),
			AllowPrivilegedContainer: false,
			AllowedCapabilities:      []corev1.Capability{"NET_BIND_SERVICE"},
			FSGroup:                  securityv1.FSGroupStrategyOptions{Type: securityv1.FSGroupStrategyMustRunAs},
			ReadOnlyRootFilesystem:   false,
			RequiredDropCapabilities: []corev1.Capability{"KILL", "MKNOD", "SETUID", "SETGID"},
			RunAsUser:                securityv1.RunAsUserStrategyOptions{Type: securityv1.RunAsUserStrategyMustRunAs},
			SELinuxContext:           securityv1.SELinuxContextStrategyOptions{Type: securityv1.SELinuxStrategyMustRunAs},
			SupplementalGroups:       securityv1.SupplementalGroupsStrategyOptions{Type: securityv1.SupplementalGroupsStrategyRunAsAny},
			Volumes:                  []securityv1.FSType{securityv1.FSTypeConfigMap, securityv1.FSTypeDownwardAPI, securityv1.FSTypeEmptyDir, securityv1.FSTypePersistentVolumeClaim, securityv1.FSProjected, securityv1.FSTypeSecret},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "scc_privileged",
			},
			AllowPrivilegedContainer: true,
			FSGroup:                  securityv1.FSGroupStrategyOptions{Type: securityv1.FSGroupStrategyRunAsAny},
			RunAsUser:                securityv1.RunAsUserStrategyOptions{Type: securityv1.RunAsUserStrategyRunAsAny},
			SELinuxContext:           securityv1.SELinuxContextStrategyOptions{Type: securityv1.SELinuxStrategyRunAsAny},
			SupplementalGroups:       securityv1.SupplementalGroupsStrategyOptions{Type: securityv1.SupplementalGroupsStrategyRunAsAny},
		},
	} {
		require.NoError(t, indexer.Add(scc))
	}

	return securityv1listers.NewSecurityContextConstraintsLister(indexer)
}

func TestPodSecurityAdmissionLabelSynchronizationController_isNSControlled(t *testing.T) {
	namespaces := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, ns := range []*corev1.Namespace{
		{ObjectMeta: metav1.ObjectMeta{Name: "openshift"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "openshift-config-managed"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "openshift-user-created"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "openshift-user-created-controlled", Labels: map[string]string{"security.openshift.io/scc.podSecurityLabelSync": "true"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "tested-ns", Labels: map[string]string{"security.openshift.io/scc.podSecurityLabelSync": "false"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "willing-tested-ns", Labels: map[string]string{"security.openshift.io/scc.podSecurityLabelSync": "true"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "nihilistic-tested-ns"}},
	} {
		require.NoError(t, namespaces.Add(ns))
	}

	nsLister := corev1listers.NewNamespaceLister(namespaces)

	tests := []struct {
		name    string
		nsName  string
		want    bool
		wantErr bool
	}{
		{
			name:    "unknown namespace",
			nsName:  "unknown",
			want:    false,
			wantErr: true,
		},
		{
			name:    "openshift-prefixed namespace",
			nsName:  "openshift-config-managed",
			want:    false,
			wantErr: false,
		},
		{
			name:    "openshift NS",
			nsName:  "openshift",
			want:    false,
			wantErr: false,
		},
		{
			name:    "kube-system NS",
			nsName:  "kube-system",
			want:    false,
			wantErr: false,
		},
		{
			name:    "NS that does not want to be synced",
			nsName:  "tested-ns",
			want:    false,
			wantErr: false,
		},
		{
			name:    "NS that wants to be synced",
			nsName:  "willing-tested-ns",
			want:    true,
			wantErr: false,
		},
		{
			name:    "NS that does not care",
			nsName:  "nihilistic-tested-ns",
			want:    true,
			wantErr: false,
		},
		{
			name:    "NS created by a user who needs to know what they are doing - no sync",
			nsName:  "openshift-user-created",
			want:    false,
			wantErr: false,
		},
		{
			name:    "NS created by a user who needs to know what they are doing - sync label",
			nsName:  "openshift-user-created-controlled",
			want:    true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &PodSecurityAdmissionLabelSynchronizationController{
				namespaceLister: nsLister,
			}
			got, err := c.isNSControlled(tt.nsName)
			if (err != nil) != tt.wantErr {
				t.Errorf("PodSecurityAdmissionLabelSynchronizationController.isNSControlled() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("PodSecurityAdmissionLabelSynchronizationController.isNSControlled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPodSecurityAdmissionLabelSynchronizationController_saToSCCCAcheEnqueueFunc(t *testing.T) {
	namespaces := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})

	controlledNSLen := len(testNamespaces) - 1

	for _, ns := range testNamespaces {
		ns := ns
		require.NoError(t, namespaces.Add(ns))
	}
	nsLister := corev1listers.NewNamespaceLister(namespaces)
	labelSelector, err := controlledNamespacesLabelSelector()
	require.NoError(t, err)

	roleObjOrDie := func(obj interface{}) RoleInterface {
		r, err := NewRoleObj(obj)
		if err != nil {
			t.Fatal(err.Error())
		}
		return r
	}

	tests := []struct {
		name           string
		incomingObj    interface{}
		expectedKeyNum int
	}{
		{
			name:           "incoming SCC",
			incomingObj:    &securityv1.SecurityContextConstraints{},
			expectedKeyNum: controlledNSLen,
		},
		{
			name:           "incoming clusterrole",
			incomingObj:    roleObjOrDie(&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "clusterrole"}}),
			expectedKeyNum: controlledNSLen,
		},
		{
			name:           "incoming role from a controlled namespace",
			incomingObj:    roleObjOrDie(&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "role", Namespace: "controlled-namespace"}}),
			expectedKeyNum: 1,
		},
		{
			name:           "incoming role from a non-controlled namespace",
			incomingObj:    roleObjOrDie(&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "role", Namespace: "non-controlled-namespace"}}),
			expectedKeyNum: 0,
		},
		{
			name:           "incoming role from a non-existent namespace",
			incomingObj:    roleObjOrDie(&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "role", Namespace: "unknown"}}),
			expectedKeyNum: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

			c := &PodSecurityAdmissionLabelSynchronizationController{
				namespaceLister: nsLister,
				nsLabelSelector: labelSelector,
				workQueue:       queue,
			}
			c.saToSCCCAcheEnqueueFunc(tt.incomingObj)

			require.Equal(t, tt.expectedKeyNum, queue.Len())

		})
	}
}
func TestEnforcingPodSecurityAdmissionLabelSynchronizationController_sync(t *testing.T) {
	labelSelector, err := controlledNamespacesLabelSelector()
	require.NoError(t, err)

	nsObjectSlice := []runtime.Object{}
	nsIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, ns := range testNamespaces {
		require.NoError(t, nsIndexer.Add(ns))
		nsObjectSlice = append(nsObjectSlice, ns)
	}

	nsClient := fake.NewSimpleClientset(nsObjectSlice...)
	nsLister := corev1listers.NewNamespaceLister(nsIndexer)

	mockCache := &mockSAToSCCCache{
		mockCache: map[string]sets.String{
			"controlled-namespace/testspecificsa":                          sets.NewString("scc_restricted", "scc_baseline"),
			"controlled-namespace/testspecificsa2":                         sets.NewString("scc_restricted", "scc_privileged"),
			"controlled-namespace/testspecificsa3":                         sets.NewString("scc_restricted"),
			"controlled-namespace-previous-enforce-labels/testspecificsa3": sets.NewString("scc_restricted"),
			"controlled-namespace-previous-warn-labels/testspecificsa3":    sets.NewString("scc_restricted"),
		},
	}

	tests := []struct {
		name             string
		serviceAccounts  []*corev1.ServiceAccount
		nsName           string
		wantErr          bool
		expectNSUpdate   bool
		expectedPSaLevel string
	}{
		{
			name:    "non-existent ns",
			nsName:  "unknown",
			wantErr: true,
		},
		{
			name:   "terminating ns",
			nsName: "controlled-namespace-terminating",
		},
		{
			name:   "controlled NS w/o UID annotation",
			nsName: "controlled-namespace-without-uid-annotation",
		},
		{
			name:   "no SAs in the namespace",
			nsName: "controlled-namespace",
			serviceAccounts: []*corev1.ServiceAccount{
				{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "other-ns"}},
			},
			wantErr: false,
		},
		{
			name:   "SA without any assigned SCCs",
			nsName: "controlled-namespace",
			serviceAccounts: []*corev1.ServiceAccount{
				{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "controlled-namespace"}},
			},
			wantErr: true,
		},
		{
			name:   "SA with restricted and baseline SCCs assigned",
			nsName: "controlled-namespace",
			serviceAccounts: []*corev1.ServiceAccount{
				{ObjectMeta: metav1.ObjectMeta{Name: "testspecificsa", Namespace: "controlled-namespace"}},
			},
			wantErr:          false,
			expectNSUpdate:   true,
			expectedPSaLevel: "baseline",
		},
		{
			name:   "SA with restricted and privileged SCCs assigned",
			nsName: "controlled-namespace",
			serviceAccounts: []*corev1.ServiceAccount{
				{ObjectMeta: metav1.ObjectMeta{Name: "testspecificsa2", Namespace: "controlled-namespace"}},
			},
			wantErr:          false,
			expectNSUpdate:   true,
			expectedPSaLevel: "privileged",
		},
		{
			name:   "SA with restricted SCC assigned",
			nsName: "controlled-namespace",
			serviceAccounts: []*corev1.ServiceAccount{
				{ObjectMeta: metav1.ObjectMeta{Name: "testspecificsa3", Namespace: "controlled-namespace"}},
			},
			wantErr:          false,
			expectNSUpdate:   true,
			expectedPSaLevel: "restricted",
		},
		{
			name:   "SA with restricted SCC assigned in am NS with previous enforce labels",
			nsName: "controlled-namespace-previous-enforce-labels",
			serviceAccounts: []*corev1.ServiceAccount{
				{ObjectMeta: metav1.ObjectMeta{Name: "testspecificsa3", Namespace: "controlled-namespace-previous-enforce-labels"}},
			},
			wantErr:          false,
			expectNSUpdate:   true,
			expectedPSaLevel: "restricted",
		},
		{
			name:   "SA with restricted SCC assigned in am NS with previous warn labels",
			nsName: "controlled-namespace-previous-warn-labels",
			serviceAccounts: []*corev1.ServiceAccount{
				{ObjectMeta: metav1.ObjectMeta{Name: "testspecificsa3", Namespace: "controlled-namespace-previous-warn-labels"}},
			},
			wantErr:          false,
			expectNSUpdate:   true,
			expectedPSaLevel: "restricted",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCtx := context.Background()

			saIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
				cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
			})
			for _, sa := range tt.serviceAccounts {
				require.NoError(t, saIndexer.Add(sa))
			}

			c := &PodSecurityAdmissionLabelSynchronizationController{
				shouldEnforce: true,

				namespaceClient: nsClient.CoreV1().Namespaces(),

				namespaceLister:      nsLister,
				serviceAccountLister: corev1listers.NewServiceAccountLister(saIndexer),
				sccLister:            syncSCCLister(t),

				nsLabelSelector: labelSelector,
				saToSCCsCache:   mockCache,
			}

			nsWatcher, err := nsClient.CoreV1().Namespaces().Watch(testCtx, metav1.ListOptions{})
			require.NoError(t, err)
			if nsWatcher != nil {
				defer nsWatcher.Stop()
			}
			var nsModified *corev1.Namespace
			finished := make(chan bool)
			timedCtx, timedCtxCancel := context.WithTimeout(context.Background(), 1500*time.Second)
			go func() {
				nsChan := nsWatcher.ResultChan()
				for {
					select {
					case nsEvent := <-nsChan:
						ns, ok := nsEvent.Object.(*corev1.Namespace)
						require.True(t, ok)
						if ns.Name == tt.nsName && nsEvent.Type == watch.Modified {
							nsModified = ns
						}
						// check nsEvent.Type is watch.Modified
					case <-timedCtx.Done():
						finished <- true
						return
					}
				}
			}()

			go func() {
				if err := c.sync(testCtx, &mockedSyncContext{key: tt.nsName}); (err != nil) != tt.wantErr {
					t.Errorf("PodSecurityAdmissionLabelSynchronizationController.sync() error = %v, wantErr %v", err, tt.wantErr)
				}

				time.Sleep(1 * time.Second)
				timedCtxCancel()
			}()

			<-finished
			require.Equal(t, tt.expectNSUpdate, (nsModified != nil), "expected NS update to be %v, but it was %v", tt.expectNSUpdate, nsModified)

			if nsModified != nil && len(tt.expectedPSaLevel) > 0 {
				require.Equal(t, tt.expectedPSaLevel, nsModified.Labels[psapi.EnforceLevelLabel], "unexpected PSa enforcement level")
				require.Equal(t, "", nsModified.Labels[psapi.WarnLevelLabel], "unexpected PSa warn level")
				require.Equal(t, "", nsModified.Labels[psapi.AuditLevelLabel], "unexpected PSa audit level")
			}
		})
	}
}

func TestPodSecurityAdmissionLabelSynchronizationController_sync(t *testing.T) {
	labelSelector, err := controlledNamespacesLabelSelector()
	require.NoError(t, err)

	nsObjectSlice := []runtime.Object{}
	nsIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, ns := range testNamespaces {
		require.NoError(t, nsIndexer.Add(ns))
		nsObjectSlice = append(nsObjectSlice, ns)
	}

	nsClient := fake.NewSimpleClientset(nsObjectSlice...)
	nsLister := corev1listers.NewNamespaceLister(nsIndexer)

	mockCache := &mockSAToSCCCache{
		mockCache: map[string]sets.String{
			"controlled-namespace/testspecificsa":  sets.NewString("scc_restricted", "scc_baseline"),
			"controlled-namespace/testspecificsa2": sets.NewString("scc_restricted", "scc_privileged"),
			"controlled-namespace/testspecificsa3": sets.NewString("scc_restricted"),
		},
	}

	tests := []struct {
		name             string
		serviceAccounts  []*corev1.ServiceAccount
		nsName           string
		wantErr          bool
		expectNSUpdate   bool
		expectedPSaLevel string
	}{
		{
			name:    "non-existent ns",
			nsName:  "unknown",
			wantErr: true,
		},
		{
			name:   "terminating ns",
			nsName: "controlled-namespace-terminating",
		},
		{
			name:   "controlled NS w/o UID annotation",
			nsName: "controlled-namespace-without-uid-annotation",
		},
		{
			name:   "no SAs in the namespace",
			nsName: "controlled-namespace",
			serviceAccounts: []*corev1.ServiceAccount{
				{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "other-ns"}},
			},
			wantErr: false,
		},
		{
			name:   "SA without any assigned SCCs",
			nsName: "controlled-namespace",
			serviceAccounts: []*corev1.ServiceAccount{
				{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "controlled-namespace"}},
			},
			wantErr: true,
		},
		{
			name:   "SA with restricted and baseline SCCs assigned",
			nsName: "controlled-namespace",
			serviceAccounts: []*corev1.ServiceAccount{
				{ObjectMeta: metav1.ObjectMeta{Name: "testspecificsa", Namespace: "controlled-namespace"}},
			},
			wantErr:          false,
			expectNSUpdate:   true,
			expectedPSaLevel: "baseline",
		},
		{
			name:   "SA with restricted and privileged SCCs assigned",
			nsName: "controlled-namespace",
			serviceAccounts: []*corev1.ServiceAccount{
				{ObjectMeta: metav1.ObjectMeta{Name: "testspecificsa2", Namespace: "controlled-namespace"}},
			},
			wantErr:          false,
			expectNSUpdate:   true,
			expectedPSaLevel: "privileged",
		},
		{
			name:   "SA with restricted SCC assigned",
			nsName: "controlled-namespace",
			serviceAccounts: []*corev1.ServiceAccount{
				{ObjectMeta: metav1.ObjectMeta{Name: "testspecificsa3", Namespace: "controlled-namespace"}},
			},
			wantErr:          false,
			expectNSUpdate:   true,
			expectedPSaLevel: "restricted",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCtx := context.Background()

			saIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
				cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
			})
			for _, sa := range tt.serviceAccounts {
				require.NoError(t, saIndexer.Add(sa))
			}

			c := &PodSecurityAdmissionLabelSynchronizationController{
				namespaceClient: nsClient.CoreV1().Namespaces(),

				namespaceLister:      nsLister,
				serviceAccountLister: corev1listers.NewServiceAccountLister(saIndexer),
				sccLister:            syncSCCLister(t),

				nsLabelSelector: labelSelector,
				saToSCCsCache:   mockCache,
			}

			nsWatcher, err := nsClient.CoreV1().Namespaces().Watch(testCtx, metav1.ListOptions{})
			require.NoError(t, err)
			if nsWatcher != nil {
				defer nsWatcher.Stop()
			}
			var nsModified *corev1.Namespace
			finished := make(chan bool)
			timedCtx, timedCtxCancel := context.WithTimeout(context.Background(), 1500*time.Second)
			go func() {
				nsChan := nsWatcher.ResultChan()
				for {
					select {
					case nsEvent := <-nsChan:
						ns, ok := nsEvent.Object.(*corev1.Namespace)
						require.True(t, ok)
						if ns.Name == tt.nsName && nsEvent.Type == watch.Modified {
							nsModified = ns
						}
						// check nsEvent.Type is watch.Modified
					case <-timedCtx.Done():
						finished <- true
						return
					}
				}
			}()

			go func() {
				if err := c.sync(testCtx, &mockedSyncContext{key: tt.nsName}); (err != nil) != tt.wantErr {
					t.Errorf("PodSecurityAdmissionLabelSynchronizationController.sync() error = %v, wantErr %v", err, tt.wantErr)
				}

				time.Sleep(1 * time.Second)
				timedCtxCancel()
			}()

			<-finished
			require.Equal(t, tt.expectNSUpdate, (nsModified != nil), "expected NS update to be %v, but it was %v", tt.expectNSUpdate, nsModified)

			if nsModified != nil && len(tt.expectedPSaLevel) > 0 {
				require.Equal(t, tt.expectedPSaLevel, nsModified.Labels[psapi.WarnLevelLabel], "unexpected PSa warn level")
				require.Equal(t, tt.expectedPSaLevel, nsModified.Labels[psapi.AuditLevelLabel], "unexpected PSa audit level")
			}
		})
	}
}

type mockSAToSCCCache struct {
	mockCache map[string]sets.String // as a shortcut, this is just a mapping of sa->SCCs
}

func (m *mockSAToSCCCache) AddEventHandlers(rbacv1informers.Interface, securityv1informers.SecurityContextConstraintsInformer) {
}

func (m *mockSAToSCCCache) WithExternalQueueEnqueue(func(interface{})) SAToSCCCache {
	return m
}

func (m *mockSAToSCCCache) IsRoleBindingRelevant(_ interface{}) bool {
	panic("not implemented")
}

func (m *mockSAToSCCCache) SCCsFor(sa *corev1.ServiceAccount) (sets.String, error) {
	return m.mockCache[fmt.Sprintf("%s/%s", sa.Namespace, sa.Name)], nil
}

type mockedSyncContext struct {
	key string

	factory.SyncContext
}

func (c *mockedSyncContext) Queue() workqueue.RateLimitingInterface {
	return nil
}

func (c *mockedSyncContext) QueueKey() string {
	return c.key
}

func (c *mockedSyncContext) Recorder() events.Recorder {
	return nil
}
