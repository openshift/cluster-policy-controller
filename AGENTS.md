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
- If PSA enforcement is turned on, synchronize Pod Security Admission (PSA) labels based on SCC assignments
- If PSA enforcement is turned off, synchronize only warn and audit PSA labels based on SCC assignments
- Label privileged system namespaces with PSA privileged level

### Controllers

Six controllers are registered in `pkg/cmd/controller/config.go`. See [ARCHITECTURE.md — Controller Details](./ARCHITECTURE.md#controller-details) for each controller's purpose, resources watched, and data flow.

## Architecture Patterns

### OpenShift library-go Pattern

This binary follows the **OpenShift library-go controller pattern**:

**Key Characteristics**:
1. **Factory-based Controllers**: Use `factory.New()` pattern from library-go
2. **Informer-driven**: React to cluster changes via Kubernetes informers
3. **Event Recording**: Use `events.Recorder` for audit trail
4. **Shared Context**: All controllers share `EnhancedControllerContext` for informers and clients
5. **Controller Enablement**: Controllers can be enabled/disabled via `OpenShiftControllerManagerConfig.Controllers`

For controller initialization patterns, the `EnhancedControllerContext`, and the startup flow, see [ARCHITECTURE.md — Controller Architecture](./ARCHITECTURE.md#controller-architecture) and [Startup and Lifecycle](./ARCHITECTURE.md#startup-and-lifecycle).

## Development Guidelines

### What to Always Do

1. Use Informers and Listers (Almost Never Direct API Calls in Sync Loops)
2. Use library-go Factory Pattern
3. Use Server-Side Apply for Updates
4. Record Events for Significant Actions
5. Handle Conflicts Gracefully
6. Respect Controller Enablement

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

1. Almost Never Make Direct API Calls in Sync Loops
2. Never Bypass the ControllerInitializers Registry
3. Never Ignore Error Returns
4. Never Modify the Vendor Directory Manually

#### 5. Never Skip the Health Check
The binary waits for API server health before starting controllers (`WaitForHealthyAPIServer`). Don't remove or bypass this - it prevents controllers from starting before the API server is ready.

#### 6. Never Break the UID Allocation Bitmap
The `RangeAllocation` resource stores a bitmap of allocated UID offsets. Operations on this bitmap must be:
- Atomic (update with optimistic concurrency via resourceVersion)
- Consistent with namespace annotations
- Repaired periodically (every 8 hours)

#### 7. Never Assume Feature Gate State
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

- **Don't forget the service account constant** - Every controller should use a dedicated service account name constant in `config.go` for `ClientBuilder.Client()`. Missing this means the controller can't authenticate.

- **Don't modify `nsexemptions/`** - The namespace exemption list determines which namespaces are never synced by the PSA label syncer. Changes here affect cluster security posture.

- **Don't use table-driven tests without the `t.Run()` pattern** - All tests should use subtests for better failure isolation:

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
