package metrics

import (
	"github.com/openshift/library-go/pkg/security/uid"
	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
	"math/big"
	"math/bits"
	"sync"

	v1 "github.com/openshift/api/securityinternal/v1"
)

const (
	namespaceSCCAllocSubsystem = "openshift_namespace_scc_allocation"

	namespaceProcessedCountKey = "namespace_processed_total"
	uidBlocksCountKey          = "uid_blocks_total"
	uidBlocksAllocatedCountKey = "uid_blocks_allocated_total"
)

var (
	namespacesProcessedCount = metrics.NewGauge(&metrics.GaugeOpts{
		Subsystem:      namespaceSCCAllocSubsystem,
		Name:           namespaceProcessedCountKey,
		Help:           "Number of namespaces that have assigned Security Context Constraints (SCC) annotations",
		StabilityLevel: metrics.ALPHA,
	})
	uidBlocksCount = metrics.NewGauge(&metrics.GaugeOpts{
		Subsystem:      namespaceSCCAllocSubsystem,
		Name:           uidBlocksCountKey,
		Help:           "Total number of UID blocks for allocation.",
		StabilityLevel: metrics.ALPHA,
	})
	uidBlocksAllocatedCount = metrics.NewGauge(&metrics.GaugeOpts{
		Subsystem:      namespaceSCCAllocSubsystem,
		Name:           uidBlocksAllocatedCountKey,
		Help:           "Allocated number of UID blocks.",
		StabilityLevel: metrics.ALPHA,
	})
)

var registerMetrics sync.Once

// Register registers NamespaceSCCAllocation metrics.
func Register() {
	registerMetrics.Do(func() {
		legacyregistry.MustRegister(namespacesProcessedCount)
		legacyregistry.MustRegister(uidBlocksCount)
		legacyregistry.MustRegister(uidBlocksAllocatedCount)
	})
}

type NamespaceSCCAllocationMetrics interface {
	UIDRangeUpdated(uidRange *v1.RangeAllocation)
	SetNamespacesProcessed(count int)
	AddNamespacesProcessed(delta int)
}

type namespaceSCCAllocMetrics struct {
}

// NewNamespaceSCCAllocMetrics creates a NamespaceSCCAllocationMetrics
func NewNamespaceSCCAllocMetrics() NamespaceSCCAllocationMetrics {
	return &namespaceSCCAllocMetrics{}
}

func (r *namespaceSCCAllocMetrics) SetNamespacesProcessed(count int) {
	namespacesProcessedCount.Set(float64(count))
}

func (r *namespaceSCCAllocMetrics) AddNamespacesProcessed(delta int) {
	namespacesProcessedCount.Add(float64(delta))
}

func (r *namespaceSCCAllocMetrics) UIDRangeUpdated(uidRange *v1.RangeAllocation) {
	blocksCount := -1.0
	uRange, err := uid.ParseRange(uidRange.Range)
	if err == nil {
		blocksCount = float64(uRange.Size())
	}
	uidBlocksCount.Set(blocksCount)

	allocatedBitMapInt := big.NewInt(0).SetBytes(uidRange.Data)
	uidBlocksAllocated := float64(countBits(allocatedBitMapInt))
	uidBlocksAllocatedCount.Set(uidBlocksAllocated)
}

// countBits returns the number of set bits in n
func countBits(n *big.Int) int {
	count := 0
	for _, b := range n.Bytes() {
		count += bits.OnesCount8(b)
	}
	return count
}
