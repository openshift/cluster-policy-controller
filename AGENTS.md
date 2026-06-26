# Cluster Policy Controller - AI Agent Development Guide

> **For AI Agents:** This file contains critical context for understanding and working with the cluster-policy-controller codebase. Read this fully before making changes. Pay special attention to "Never Do" and "Common Agent Mistakes" sections.

The cluster-policy-controller maintains policy resources necessary to create pods in an OpenShift cluster: UID/SELinux allocation, quota management, CSR approval, and Pod Security Admission label synchronization.

## Quick Reference

### Essential Commands
```bash
# Build
make build                    # Build binary

# Test
make test                     # Run unit tests (./pkg/... ./cmd/...)
make verify                   # Run all verification checks

# Image
make images                   # Build container image

# Clean
make clean                    # Remove built binary
```

**Note**: This lists the most commonly used commands. Consult the [Makefile](./Makefile) for
additional build, test, and verification targets.

### Additional Documentation
- **[ARCHITECTURE.md](./ARCHITECTURE.md)** - System design, controller architecture, data flow
- **[CONTRIBUTING.md](./CONTRIBUTING.md)** - Development workflow, testing, PR process
- **[README.md](./README.md)** - Quick start and deployment context

## Overview

The cluster-policy-controller is a multi-controller binary that runs inside the `kube-controller-manager` static pod in the `openshift-kube-controller-manager` namespace. It is managed by the [`cluster-kube-controller-manager-operator`](https://github.com/openshift/cluster-kube-controller-manager-operator/).

**Purpose**: Maintain policy resources (UID ranges, SELinux labels, quotas, PSA labels) that are prerequisites for pod creation in an OpenShift cluster.

**Key Capabilities**:
- Allocate UID ranges and SELinux MCS labels for namespaces
- Manage ResourceQuota with OpenShift-specific image stream quota support
- Reconcile ClusterResourceQuota usage across namespaces
- Approve CertificateSigningRequests for monitoring components
- Synchronize Pod Security Admission labels based on SCC assignments
- Label privileged system namespaces with PSA privileged level

### Controllers

Six controllers are registered in `pkg/cmd/controller/config.go`. See [ARCHITECTURE.md — Controller Details](./ARCHITECTURE.md#controller-details) for each controller's purpose, resources watched, and data flow.

## Architecture Patterns

### OpenShift library-go Pattern

This binary follows the **OpenShift library-go controller pattern**:

```go
import (
    "github.com/openshift/library-go/pkg/controller/factory"
    "github.com/openshift/library-go/pkg/controller/controllercmd"
    "github.com/openshift/library-go/pkg/operator/events"
)
```

**Key Characteristics**:
1. **Factory-based Controllers**: Use `factory.New()` pattern from library-go
2. **Informer-driven**: React to cluster changes via Kubernetes informers
3. **Event Recording**: Use `events.Recorder` for audit trail
4. **Shared Context**: All controllers share `EnhancedControllerContext` for informers and clients
5. **Controller Enablement**: Controllers can be enabled/disabled via `OpenShiftControllerManagerConfig.Controllers`

For controller initialization patterns, the `EnhancedControllerContext`, and the startup flow, see [ARCHITECTURE.md — Controller Architecture](./ARCHITECTURE.md#controller-architecture) and [Startup and Lifecycle](./ARCHITECTURE.md#startup-and-lifecycle).

## Common Workflows

### Adding a New Controller

1. **Create the controller package** under `pkg/`:
   ```go
   func NewMyController(
       namespaceInformer corev1informers.NamespaceInformer,
       client corev1client.NamespaceInterface,
       eventRecorder events.Recorder,
   ) factory.Controller {
       c := &myController{...}
       return factory.New().
           WithSync(c.sync).
           WithInformers(namespaceInformer.Informer()).
           ToController("my-controller", eventRecorder)
   }
   ```

2. **Create an init function** in `pkg/cmd/controller/`:
   ```go
   func RunMyController(ctx context.Context, controllerCtx *EnhancedControllerContext) (bool, error) {
       kubeClient, err := controllerCtx.ClientBuilder.Client(myServiceAccountName)
       if err != nil {
           return true, err
       }
       controller := mypkg.NewMyController(
           controllerCtx.KubernetesInformers.Core().V1().Namespaces(),
           kubeClient.CoreV1().Namespaces(),
           controllerCtx.EventRecorder,
       )
       go controller.Run(ctx, 1)
       return true, nil
   }
   ```

3. **Register in `config.go`**:
   ```go
   var ControllerInitializers = map[string]InitFunc{
       // ... existing controllers
       "openshift.io/my-controller": RunMyController,
   }
   ```

4. **Add a service account constant** in `config.go`.

5. **Add tests** following existing patterns.

### Modifying an Existing Controller

1. **Find the init function** in `pkg/cmd/controller/` to understand what informers and clients it uses.
2. **Find the controller implementation** in its package under `pkg/`.
3. **Check the sync function** - this is where the main logic lives.
4. **Run `make build test`** to verify changes.

### Adding New Informers

If a controller needs to watch a new resource type:

1. **Check if the informer factory exists** in `EnhancedControllerContext` (`pkg/cmd/controller/interfaces.go`).
2. If not, add a new informer factory to the context and start it in `StartInformers()`.
3. **Wire the informer** in the controller init function.
4. The informer will be started automatically when `StartInformers()` is called.

## Development Guidelines

### What to Always Do

#### 1. Use Informers and Listers (Never Direct API Calls in Sync Loops)
```go
// GOOD: Use lister (in-memory, fast)
ns, err := c.nsLister.Get(namespaceName)

// BAD: Direct API call in sync loop
ns, err := c.kubeClient.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
```

#### 2. Use library-go Factory Pattern
```go
// GOOD: factory.New() for controller creation
return factory.New().
    WithSync(c.sync).
    WithInformers(informer.Informer()).
    ToController(controllerName, eventRecorder)

// BAD: Manual work queue management
```

#### 3. Use Server-Side Apply for Updates
```go
// GOOD: Server-side apply
patch := &applycorev1.NamespaceApplyConfiguration{...}
nsClient.Apply(ctx, patch, metav1.ApplyOptions{FieldManager: controllerName})

// BAD: Read-modify-write cycle
ns, _ := client.Get(ctx, name, metav1.GetOptions{})
ns.Labels["key"] = "value"
client.Update(ctx, ns, metav1.UpdateOptions{})
```

#### 4. Record Events for Significant Actions
```go
syncCtx.Recorder().Eventf("CreatedSCCRanges", "created SCC ranges for %v namespace", ns.Name)
```

#### 5. Handle Conflicts Gracefully
```go
if apierrors.IsConflict(err) {
    return factory.SyntheticRequeueError  // Let the queue retry
}
```

#### 6. Respect Controller Enablement
Controllers are enabled/disabled via `OpenShiftControllerManagerConfig.Controllers`. The `IsControllerEnabled` check happens in `startControllers()` - don't bypass it.

### What to Ask First

#### 1. Before Adding New Dependencies
**Ask**: "Is this already available in library-go or the Kubernetes packages we vendor?"
- Prefer library-go patterns over custom implementations
- Check `go.mod` for existing dependencies
- New dependencies must be vendored (`go mod vendor`)

#### 2. Before Changing Informer Resync Periods
**Ask**: "What is the impact on API server load?"
- Default is 10 minutes for most informers
- The namespace SCC allocation controller uses 8-hour repair cycles
- Shorter resync = more API server load

#### 3. Before Modifying UID/MCS Allocation Logic
**Ask**: "Does this change affect existing clusters with allocated ranges?"
- UID ranges are permanent once assigned to namespaces
- The bitmap-based allocator in `RangeAllocation` is a critical data structure
- Changes can break existing namespace security configurations

#### 4. Before Changing PSA Label Sync Behavior
**Ask**: "Does this affect the `security.openshift.io/scc.podSecurityLabelSync` contract?"
- Namespaces opt in/out via this label
- System namespaces (`openshift-*`) have special handling
- Exempted namespaces are hardcoded in `nsexemptions/`

#### 5. Before Adding New Service Accounts
**Ask**: "Does the service account exist in the `openshift-infra` namespace?"
- Controllers use dedicated service accounts via `ClientBuilder`
- These must be provisioned by the kube-controller-manager-operator

### What to Never Do

#### 1. Never Make Direct API Calls in Sync Loops
```go
// BAD: Direct API call on every sync
ns, err := kubeClient.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})

// GOOD: Use cached lister
ns, err := c.nsLister.Get(name)
```

**Why**: Direct API calls in sync loops cause performance issues and increased API server load. Informers provide a local cache that is kept in sync via watches.

#### 2. Never Bypass the ControllerInitializers Registry
All controllers must be registered in `pkg/cmd/controller/config.go`:
```go
var ControllerInitializers = map[string]InitFunc{
    "openshift.io/my-controller": RunMyController,
}
```

**Why**: The registry is how controllers are discovered, enabled/disabled, and started. A controller file alone won't run.

#### 3. Never Hard-Code Namespace Names
```go
// BAD
namespace := "openshift-infra"

// GOOD: Use the configured constant
namespace := defaultOpenShiftInfraNamespace
```

**Exception**: `openshift-monitoring` is hard-coded in the CSR approver because it's an external namespace with a fixed name.

#### 4. Never Ignore Error Returns
```go
// BAD
_, _, _ = c.syncNamespace(ctx, syncCtx, ns)

// GOOD
if err := c.syncNamespace(ctx, syncCtx, ns); err != nil {
    return err
}
```

#### 5. Never Modify the Vendor Directory Manually
```bash
# BAD: Manual edits to vendor/
vim vendor/k8s.io/...

# GOOD: Update go.mod and re-vendor
go mod tidy && go mod vendor
```

#### 6. Never Skip the Health Check
The binary waits for API server health before starting controllers (`WaitForHealthyAPIServer`). Don't remove or bypass this - it prevents controllers from starting before the API server is ready.

#### 7. Never Break the UID Allocation Bitmap
The `RangeAllocation` resource stores a bitmap of allocated UID offsets. Operations on this bitmap must be:
- Atomic (update with optimistic concurrency via resourceVersion)
- Consistent with namespace annotations
- Repaired periodically (every 8 hours)

#### 8. Never Assume Feature Gate State
```go
// GOOD: Check feature gates explicitly
featureGates := sets.NewString(controllerCtx.OpenshiftControllerConfig.FeatureGates...)
if featureGates.Has("OpenShiftPodSecurityAdmission=false") {
    // advisory mode
}
```

## Common Agent Mistakes to Avoid

These are specific mistakes AI agents frequently make in this codebase:

- **Don't suggest `client.Get()` or `client.List()` in sync functions** - Use informers and listers instead. Direct API calls in controller sync loops cause performance issues and are against the library-go pattern. Always use the cached listers from informers.

- **Don't propose adding controllers without mentioning `pkg/cmd/controller/config.go`** - All controllers must be registered in the `ControllerInitializers` map, or they won't run. Simply creating a controller file isn't enough.

- **Don't forget the service account constant** - Every controller uses a dedicated service account name constant in `config.go` for `ClientBuilder.Client()`. Missing this means the controller can't authenticate.

- **Don't confuse the two PSA label sync controllers** - There are two distinct controllers: `podsecurity-admission-label-syncer` (syncs labels based on SCC/RBAC analysis) and `privileged-namespaces-psa-label-syncer` (just labels system namespaces as privileged). They serve different purposes.

- **Don't modify `nsexemptions/` without understanding the implications** - The namespace exemption list determines which namespaces are never synced by the PSA label syncer. Changes here affect cluster security posture.

- **Don't propose changes to the UID allocator without understanding the bitmap** - The `RangeAllocation` resource uses a `big.Int` bitmap to track allocated UID blocks. The repair cycle every 8 hours rebuilds this from namespace annotations. Changes must preserve this consistency.

- **Don't use table-driven tests without the `t.Run()` pattern** - All tests should use subtests for better failure isolation:
  ```go
  for _, tt := range tests {
      t.Run(tt.name, func(t *testing.T) {
          // test implementation
      })
  }
  ```

- **Don't assume the PSA label syncer controls all namespaces** - It only syncs namespaces where `security.openshift.io/scc.podSecurityLabelSync` is not `"false"` and the namespace is not exempted. System namespaces starting with `openshift-` require explicit opt-in.

- **Don't recommend changes to quota evaluators without considering image streams** - The resource quota controllers extend standard Kubernetes quota with OpenShift `ImageStream` evaluators from `pkg/quota/quotaimageexternal/`. Breaking this integration silently disables image stream quota enforcement.

- **Don't change the field manager string** - Controllers use specific field manager names (e.g., `"cluster-policy-controller"`, `"pod-security-admission-label-synchronization-controller"`) for server-side apply. Changing these causes ownership conflicts and can break label management.

## Security Notes

- **UID Range Allocation** - Each namespace gets a unique UID block from `1000000000-1999999999/10000`. This is a finite resource that cannot be reclaimed without manual intervention.
- **SELinux MCS Labels** - Each namespace gets unique MCS labels derived from its UID range offset. These must be unique to maintain SELinux isolation.
- **PSA Label Sync** - The PSA labels determine what security profiles pods in a namespace must adhere to. Incorrect labels can either block legitimate workloads or allow privileged access.
- **CSR Approval** - The CSR approver only handles monitoring-related CSRs with specific labels and subject names. It does not approve arbitrary CSRs.
- **RBAC** - Each controller uses a separate service account with minimal permissions.

**Security-Critical Code**:
- `pkg/security/controller/` - UID and MCS allocation (affects container isolation)
- `pkg/psalabelsyncer/` - PSA label computation (affects admission control)
- `pkg/psalabelsyncer/scctopsamapping.go` - SCC to PSA level mapping
- `pkg/cmd/controller/csr.go` - CSR approval subjects

## Testing

Unit tests are co-located with code in `pkg/` directories. Use `find . -name '*_test.go'` to discover test files.

**Framework**: Go `testing` (some test files also use testify assertions)

**Running**:
```bash
make test            # Run all unit tests
go test ./pkg/...    # Run package tests directly
```

**CI/CD**: Tests run via OpenShift CI (Prow). Configuration is in the [openshift/release](https://github.com/openshift/release) repository.

## Questions?

- **Slack**: #forum-ocp-apiserver (OpenShift internal)
- **Component**: see OWNERS file
- **Approvers**: see OWNERS file
