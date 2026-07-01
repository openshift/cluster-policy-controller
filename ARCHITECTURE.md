# Cluster Policy Controller - Architecture

This document explains the architecture, design decisions, and operational model of the cluster-policy-controller.

## Table of Contents
- [Overview](#overview)
- [System Architecture](#system-architecture)
- [Controller Architecture](#controller-architecture)
- [Startup and Lifecycle](#startup-and-lifecycle)
- [Controller Details](#controller-details)
- [Informer and Client Infrastructure](#informer-and-client-infrastructure)
- [Design Decisions](#design-decisions)
- [Failure Modes and Recovery](#failure-modes-and-recovery)

## Overview

### What is the Cluster Policy Controller?

The cluster-policy-controller is a multi-controller binary that enforces cluster-wide policies required for pod creation in OpenShift. It runs as a container inside the `kube-controller-manager` static pod in the `openshift-kube-controller-manager` namespace.

Unlike an operator that manages its own operand, this controller IS the workload - it runs six independent controllers in a single process, each watching different resources and enforcing different policies.

### Why Does It Exist?

OpenShift extends Kubernetes with security and quota features that have no upstream equivalent:
- **UID Range Allocation**: Every namespace needs a unique UID range for container security isolation (SCC enforcement)
- **SELinux MCS Labels**: Required for SELinux-based container isolation, derived from UID range
- **ClusterResourceQuota**: OpenShift's cluster-scoped quota that aggregates across namespaces
- **Image Stream Quotas**: Resource quota evaluation for OpenShift ImageStreams
- **PSA Label Sync**: Bridges the gap between OpenShift SCCs and Kubernetes Pod Security Admission
- **CSR Approval**: Automated certificate approval for monitoring infrastructure

These policies are foundational - without them, pods cannot be created (missing UID range), quota cannot be enforced (missing evaluators), or monitoring breaks (unsigned certificates).

## System Architecture

### High-Level Architecture

```text
┌─────────────────────────────────────────────────────────────────────┐
│                         OpenShift Cluster                           │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  openshift-kube-controller-manager namespace                  │  │
│  │                                                               │  │
│  │  ┌─────────────────────────────────────────────────────────┐  │  │
│  │  │  kube-controller-manager static pod                     │  │  │
│  │  │                                                         │  │  │
│  │  │  ┌───────────────────────────────────────────────────┐  │  │  │
│  │  │  │  cluster-policy-controller container              │  │  │  │
│  │  │  │                                                   │  │  │  │
│  │  │  │  ┌──────────────┐  ┌──────────────────────────┐  │  │  │  │
│  │  │  │  │ Namespace    │  │ Pod Security Admission   │  │  │  │  │
│  │  │  │  │ SCC          │  │ Label Syncer             │  │  │  │  │
│  │  │  │  │ Allocation   │  │ (+ Privileged NS)        │  │  │  │  │
│  │  │  │  └──────────────┘  └──────────────────────────┘  │  │  │  │
│  │  │  │  ┌──────────────┐  ┌──────────────────────────┐  │  │  │  │
│  │  │  │  │ Resource     │  │ Cluster Quota            │  │  │  │  │
│  │  │  │  │ Quota        │  │ Reconciliation           │  │  │  │  │
│  │  │  │  │ Manager      │  │                          │  │  │  │  │
│  │  │  │  └──────────────┘  └──────────────────────────┘  │  │  │  │
│  │  │  │  ┌──────────────┐                                │  │  │  │
│  │  │  │  │ CSR Approver │                                │  │  │  │
│  │  │  │  └──────────────┘                                │  │  │  │
│  │  │  └───────────────────────────────────────────────────┘  │  │  │
│  │  └─────────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  Resources Watched / Managed:                                       │
│  ┌─────────────┐ ┌────────────┐ ┌────────┐ ┌───────────────────┐   │
│  │ Namespaces  │ │ SCCs       │ │ CSRs   │ │ ClusterResource-  │   │
│  │ (all)       │ │            │ │        │ │ Quotas            │   │
│  └─────────────┘ └────────────┘ └────────┘ └───────────────────┘   │
│  ┌─────────────┐ ┌────────────┐ ┌────────────────────────────────┐ │
│  │ Resource    │ │ Image      │ │ ServiceAccounts, Roles,        │ │
│  │ Quotas      │ │ Streams    │ │ RoleBindings, ClusterRoles,    │ │
│  │             │ │            │ │ ClusterRoleBindings            │ │
│  └─────────────┘ └────────────┘ └────────────────────────────────┘ │
│  ┌────────────────────────────────┐                                 │
│  │ RangeAllocation (UID bitmap)   │                                 │
│  └────────────────────────────────┘                                 │
└─────────────────────────────────────────────────────────────────────┘
```

### Data Flow

```text
Binary starts: cluster-policy-controller start
         │
         ▼
Parse OpenShiftControllerManagerConfig (from ComponentConfig)
         │
         ▼
Wait for API server healthy (/healthz, poll every 1s, timeout 5min)
         │
         ▼
Create EnhancedControllerContext
  ├── KubernetesInformers (10min resync)
  ├── ImageInformers (10min resync)
  ├── QuotaInformers (10min resync)
  ├── SecurityInformers (10min resync)
  ├── MetadataInformers (10min resync)
  └── ClientBuilder (per-SA clients, QPS/10 + 1)
         │
         ▼
For each ControllerInitializer:
  ├── Check IsControllerEnabled (config.Controllers list)
  ├── Call InitFunc(ctx, controllerCtx)
  └── Each controller runs in its own goroutine
         │
         ▼
StartInformers() - starts all informer factories
         │
         ▼
Block on ctx.Done() until shutdown
```

## Controller Architecture

### Controller Registry

All controllers are registered in the `ControllerInitializers` map in `pkg/cmd/controller/config.go`. Each controller:
- Has a unique name (used for enablement and logging)
- Uses a dedicated service account for RBAC isolation
- Gets its own goroutine
- Shares the informer cache (reads are cheap, writes go to API server)

### InitFunc Signature

```go
type InitFunc func(ctx context.Context, controllerCtx *EnhancedControllerContext) (bool, error)
```

Returns:
- `(true, nil)` - controller started successfully
- `(true, error)` - controller attempted to start but failed (fatal)
- `(false, nil)` - controller chose not to start (skipped)

## Startup and Lifecycle

### Process Lifecycle

The binary is managed by the `cluster-kube-controller-manager-operator` as part of the kube-controller-manager static pod. It does not self-update, does not manage its own deployment, and does not have operator status conditions. It relies on the operator for lifecycle management.

### Health Check

Before starting controllers, the binary waits for the API server:

```go
func WaitForHealthyAPIServer(client rest.Interface) error {
    return wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
        healthStatus := 0
        client.Get().AbsPath("/healthz").Do(ctx).StatusCode(&healthStatus)
        return healthStatus == http.StatusOK, nil
    })
}
```

This is critical because the binary starts alongside the API server in the same static pod.

### Controller Enablement

Controllers can be enabled/disabled via the `Controllers` field in `OpenShiftControllerManagerConfig`:
- `["*"]` (default) - all controllers enabled
- `["-openshift.io/resourcequota"]` - disable specific controller
- `["openshift.io/namespace-security-allocation"]` - enable only specific controllers

## Controller Details

### 1. Namespace Security Allocation Controller

**Key**: `openshift.io/namespace-security-allocation`
**Location**: `pkg/security/controller/namespace_scc_allocation_controller.go`
**Service Account**: `namespace-security-allocation-controller`

**Purpose**: Allocate UID ranges and SELinux MCS labels to namespaces that don't have them.

**Resources Watched**: Namespaces, RangeAllocation
**Resources Modified**: Namespaces (annotations), RangeAllocation

**How It Works**:

```text
Namespace created/updated (via informer)
         │
         ▼
Does namespace have securityv1.UIDRangeAnnotation?
  ├── Yes → Skip (already allocated)
  └── No  → Allocate
              │
              ▼
         Read RangeAllocation "scc-uid" from API
              │
              ▼
         Parse bitmap (big.Int) from RangeAllocation.Data
              │
              ▼
         Find next unset bit (linear scan)
              │
              ▼
         Set bit in bitmap, Update RangeAllocation (optimistic lock)
              │
              ▼
         Calculate UID block from bit offset
              │
              ▼
         Calculate MCS label from UID block offset
              │
              ▼
         Apply annotations to namespace (server-side apply):
           - openshift.io/sa.scc.uid-range: "1000010000/10000"
           - openshift.io/sa.scc.supplemental-groups: "1000010000/10000"
           - openshift.io/sa.scc.mcs: "s0:c1,c0"
```

**Key Configuration**:
- UID range: `1000000000-1999999999/10000` (default)
- MCS range: `s0:/2` (default)
- MCS labels per project: 5 (default)

**Repair Cycle**: Every 8 hours, rebuilds the entire bitmap from namespace annotations to fix any inconsistencies. Initial repair runs at startup with retry (10s intervals, 5min timeout).

**Concurrency**: Runs with 1 worker. The bitmap update uses optimistic concurrency (resourceVersion on RangeAllocation).

### 2. Resource Quota Manager

**Key**: `openshift.io/resourcequota`
**Location**: `pkg/cmd/controller/quota.go` (init), Kubernetes `kresourcequota.Controller` (implementation)
**Service Account**: `resourcequota-controller`

**Purpose**: Enforce ResourceQuota with OpenShift-specific image stream evaluators.

**How It Works**:

This wraps the standard Kubernetes ResourceQuota controller and adds image stream quota evaluators:

```text
Standard K8s quota controller
  + Image stream evaluators (from pkg/quota/quotaimageexternal/)
    ├── ImageStreamImport evaluator
    └── ImageStreamTag evaluator
```

**Key Configuration**:
- Concurrent syncs: from config (default 5)
- Sync period: from config (default 12 hours)
- Min resync period: from config (randomized jitter applied)
- Discovery sync: every 30 seconds (finds new API resources)

### 3. Cluster Quota Reconciliation Controller

**Key**: `openshift.io/cluster-quota-reconciliation`
**Location**: `pkg/quota/clusterquotareconciliation/reconciliation_controller.go`
**Service Account**: `cluster-quota-reconciliation-controller`

**Purpose**: Maintain ClusterResourceQuota objects by aggregating usage across all matched namespaces.

**How It Works**:

```text
ClusterResourceQuota defines:
  - Hard limits (e.g., pods: 100 across selected namespaces)
  - Namespace selector (label or annotation based)
         │
         ▼
ClusterQuotaMappingController
  - Maps ClusterResourceQuotas to namespaces
  - Watches Namespaces and ClusterResourceQuotas
         │
         ▼
ClusterQuotaReconciliationController
  - For each mapped namespace: evaluate used resources
  - Aggregate usage across all namespaces
  - Update ClusterResourceQuota.Status with total used
```

**Key Configuration**:
- Resync period: 5 minutes
- Replenishment sync period: 12 hours
- Workers: 5
- Discovery sync: every 30 seconds
- ClusterQuotaMappingController workers: 5

**Components**:
- `ClusterQuotaMappingController` (from library-go): Maps quotas to namespaces
- `ClusterQuotaReconciliationController`: Computes and updates usage
- `WorkQueueBucket`: Prioritized work queue for quota reconciliation

### 4. CSR Approver Controller

**Key**: `openshift.io/cluster-csr-approver`
**Location**: `pkg/cmd/controller/csr.go`
**Service Account**: `cluster-csr-approver-controller`

**Purpose**: Automatically approve CertificateSigningRequests for monitoring components.

**How It Works**:

```text
CertificateSigningRequest created
         │
         ▼
Label filter: metrics.openshift.io/csr.subject ∈ {prometheus, metrics-server}
  ├── No match → Ignore
  └── Match → Validate
              │
              ▼
         Check requester is openshift-monitoring:cluster-monitoring-operator
              │
              ▼
         Check subject is one of:
           - CN=system:serviceaccount:openshift-monitoring:prometheus-k8s
           - CN=system:serviceaccount:openshift-monitoring:metrics-server
              │
              ▼
         Approve CSR
```

**Scope**: Only approves CSRs from the `openshift-monitoring` namespace with specific labels and subjects. This is not a general-purpose CSR approver.

### 5. Pod Security Admission Label Syncer

**Key**: `openshift.io/podsecurity-admission-label-syncer`
**Location**: `pkg/psalabelsyncer/podsecurity_label_sync_controller.go`
**Service Account**: `podsecurity-admission-label-syncer-controller`

**Purpose**: Synchronize Kubernetes Pod Security Admission (PSA) labels on namespaces based on the SecurityContextConstraints (SCCs) available to service accounts in those namespaces.

**How It Works**:

```text
Namespace change (or SA/SCC/RoleBinding change)
         │
         ▼
Is namespace controlled?
  - Not exempted (nsexemptions list)
  - Label security.openshift.io/scc.podSecurityLabelSync != "false"
  - Not an openshift-* NS (unless explicitly opted in with label "true")
  - Controller owns at least one PSA label
         │
         ▼
Wait for UID range annotation (from SCC allocation controller)
         │
         ▼
List all ServiceAccounts in namespace
         │
         ▼
For each SA: determine allowed SCCs via RBAC
  (SAToSCCCache: RoleBindings → Roles → SCC "use" verb)
         │
         ▼
For each allowed SCC: convert to PSA level
  (scctopsamapping.go: SCC fields → privileged/baseline/restricted)
         │
         ▼
Take the most permissive PSA level across all SAs
         │
         ▼
Apply PSA labels (server-side apply):
  - pod-security.kubernetes.io/enforce: <level>
  - pod-security.kubernetes.io/warn: <level>
  - pod-security.kubernetes.io/audit: <level>
  - (+ version labels set to "latest")
  - (+ annotation: security.openshift.io/minimally-sufficient-pod-security-standard)
```

**Two Modes** (controlled by feature gate):
- **Enforcing** (default, `OpenShiftPodSecurityAdmission=true`): Sets enforce + warn + audit labels
- **Advising** (`OpenShiftPodSecurityAdmission=false`): Sets only warn + audit labels (no enforcement)

**SAToSCCCache**: Maintains a mapping of ServiceAccount → accessible SCCs by indexing RoleBindings and ClusterRoleBindings. Watches for RBAC changes and re-enqueues affected namespaces.

**Field Ownership**: Uses server-side apply with field manager `pod-security-admission-label-synchronization-controller`. Includes logic to migrate ownership from the legacy `cluster-policy-controller` field manager.

### 6. Privileged Namespaces PSA Label Syncer

**Key**: `openshift.io/privileged-namespaces-psa-label-syncer`
**Location**: `pkg/psalabelsyncer/privileged_namespaces_controller.go`
**Service Account**: `privileged-namespaces-psa-label-syncer`

**Purpose**: Label system namespaces with PSA privileged level.

**Target Namespaces**: Namespaces matching OpenShift infrastructure patterns (`openshift-*`, `kube-*`, `default`).

**Applied Labels**:
```yaml
pod-security.kubernetes.io/enforce: privileged
pod-security.kubernetes.io/audit: privileged
pod-security.kubernetes.io/warn: privileged
```

This controller is simpler than the PSA label syncer - it doesn't analyze SCCs, it just labels known system namespaces as privileged.

## Informer and Client Infrastructure

### Client QPS Management

The controller context divides API server QPS across clients:

```go
if clientConfig.QPS > 0 {
    clientConfig.QPS = clientConfig.QPS/10 + 1
}
if clientConfig.Burst > 0 {
    clientConfig.Burst = clientConfig.Burst/10 + 1
}
```

This prevents any single controller from monopolizing API server bandwidth.

### GenericResourceInformer

A union informer that tries multiple backends:
1. KubernetesInformers (core K8s resources)
2. ImageInformers (OpenShift images)
3. QuotaInformers (OpenShift quotas)
4. MetadataInformers (fallback for any GVR)

Used primarily by the quota controllers for evaluating resources across different API groups.

### Informer Lifecycle

```text
1. Controllers are initialized (each registers informer event handlers)
2. StartInformers() starts all factories (starts watches)
3. InformersStarted channel is closed (signals controllers can proceed)
4. Controllers process events from their work queues
```

Informers are started AFTER all controllers are initialized to ensure no events are missed.

## Design Decisions

### Why a Multi-Controller Binary?

**Decision**: Run 6 controllers in a single binary instead of 6 separate deployments.

**Rationale**:
- **Shared informers**: All controllers watch namespaces - sharing the informer cache saves significant API server load and memory
- **Operational simplicity**: One binary to deploy, monitor, and debug
- **Historical**: These controllers were originally part of the OpenShift controller manager

**Trade-offs**:
- Shared informers reduce memory and API calls
- Failure in one controller doesn't affect others (each runs in its own goroutine)
- But a crash in the binary takes down all controllers
- Cannot scale controllers independently

### Why Run Inside kube-controller-manager?

**Decision**: Run as a container in the kube-controller-manager static pod, not as a standalone deployment.

**Rationale**:
- These controllers must run exactly once (no replicas)
- They must start alongside the control plane
- The kube-controller-manager-operator already manages the static pod lifecycle
- Health checking is integrated with the existing control plane health

**Trade-offs**:
- Lifecycle tied to kube-controller-manager
- Cannot be independently updated without operator involvement
- Guaranteed to run on control plane nodes

### Why Bitmap-Based UID Allocation?

**Decision**: Use a `big.Int` bitmap stored in a `RangeAllocation` etcd resource for UID allocation.

**Rationale**:
- **Atomic allocation**: The bitmap + resourceVersion provides optimistic concurrency
- **Efficient scan**: Finding the next free UID is a linear bit scan
- **Compact storage**: One bit per UID block, stored in a single etcd object
- **Repairable**: The bitmap can be rebuilt from namespace annotations

**Trade-offs**:
- Simple and efficient
- But the bitmap is a single point of contention (sequential allocation)
- Repair cycle (every 8 hours) adds some API load but ensures consistency
- UID blocks cannot be reclaimed without manual namespace deletion

### Why Server-Side Apply for Namespace Updates?

**Decision**: Use server-side apply (SSA) instead of read-modify-write for namespace annotations and labels.

**Rationale**:
- **Conflict-free**: SSA uses field-level ownership, not object-level resourceVersion
- **Partial updates**: Only touches the fields the controller manages
- **Ownership tracking**: Clear audit trail of which controller owns which fields

**Trade-offs**:
- SSA is the modern Kubernetes pattern
- Requires careful field manager naming
- Migration from legacy Update() to Apply() required ownership transfer logic (see `forceHistoricalLabelsOwnership`)

### Why Separate PSA Label Controllers?

**Decision**: Two controllers for PSA labels instead of one.

**Rationale**:
- **Different logic**: System namespaces are always privileged (simple). User namespaces need SCC/RBAC analysis (complex).
- **Different failure modes**: The privileged controller is critical infrastructure. The syncer is more nuanced and can be wrong without catastrophic impact.
- **Different opt-in models**: System namespaces are always labeled. User namespaces can opt out.

**Trade-offs**:
- Clear separation of concerns
- But requires coordinating namespace exemption lists between them

### Why Per-Controller Service Accounts?

**Decision**: Each controller uses a dedicated service account from `openshift-infra`.

**Rationale**:
- **Least privilege**: Each controller gets only the RBAC permissions it needs
- **Audit trail**: API server audit logs show which controller made each call
- **Blast radius**: Compromising one SA doesn't grant access to all resources

**Trade-offs**:
- More service accounts to manage
- More RBAC resources to maintain
- But much better security posture

## Failure Modes and Recovery

### Controller Crash

**Symptom**: The entire binary crashes (since all controllers run in one process).

**Recovery**:
- The static pod is automatically restarted by kubelet
- Controllers resume from their informer caches
- Work queue items are re-processed
- The UID allocator runs its initial repair on restart

### API Server Unavailable

**Symptom**: Informers can't connect, API calls fail.

**Recovery**:
- The health check (`WaitForHealthyAPIServer`) retries for up to 5 minutes
- After startup, informers automatically reconnect via watch
- Work queue items are requeued on failure with exponential backoff

### UID Range Exhaustion

**Symptom**: `uid range exceeded` error in logs. New namespaces don't get UID annotations.

**Recovery**:
- This is a hard limit - the range `1000000000-1999999999/10000` supports ~100,000 namespaces
- Requires widening the range (config change + operator restart)
- Existing allocations are preserved

### Stale RangeAllocation Bitmap

**Symptom**: Bitmap doesn't match namespace annotations (e.g., after etcd restore).

**Recovery**:
- The 8-hour repair cycle rebuilds the bitmap from namespace annotations
- Can force immediate repair by restarting the binary
- The repair is idempotent and safe to run at any time

### PSA Label Conflicts

**Symptom**: Multiple field managers trying to set the same PSA labels.

**Recovery**:
- The controller detects when another manager owns a label and backs off
- The `forceHistoricalLabelsOwnership` function handles migration from legacy field managers
- Logs a warning: "someone else is already managing the PSA labels"

### Quota Reconciliation Lag

**Symptom**: ClusterResourceQuota status shows stale usage counts.

**Recovery**:
- The 5-minute resync period ensures eventual consistency
- Discovery sync (every 30s) picks up new resource types
- Force immediate reconciliation by editing the ClusterResourceQuota
