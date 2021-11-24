package controller

import (
	"fmt"
	"math/big"
	"strings"

	v1 "github.com/openshift/api/securityinternal/v1"
)

const (
	namespaceNameKeyPrefix = "ns"
)

func asNamespaceNameKey(namespace string) (namespaceNameKey string) {
	return namespaceNameKeyPrefix + "/" + namespace
}

func parseNamespaceNameKey(key string) (namespace string, err error) {
	parts := strings.Split(key, "/")
	if len(parts) != 2 || parts[0] != namespaceNameKeyPrefix || parts[1] == "" {
		return "", fmt.Errorf("unexpected namespace name key format: %q", key)
	}

	return parts[1], nil
}

type fakeNamespaceSCCAllocMetrics struct {
	uidRanges           []v1.RangeAllocation
	namespacesProcessed []int
}

func newFakeNamespaceSCCAllocMetrics() *fakeNamespaceSCCAllocMetrics {
	return &fakeNamespaceSCCAllocMetrics{}
}

func (r *fakeNamespaceSCCAllocMetrics) SetNamespacesProcessed(count int) {
	r.namespacesProcessed = append(r.namespacesProcessed, count)
}

func (r *fakeNamespaceSCCAllocMetrics) AddNamespacesProcessed(delta int) {
	last := 0
	if len(r.namespacesProcessed) > 0 {
		last = r.namespacesProcessed[0]
	}
	r.namespacesProcessed = append(r.namespacesProcessed, last+delta)
}

func (r *fakeNamespaceSCCAllocMetrics) UIDRangeUpdated(uidRange *v1.RangeAllocation) {
	r.uidRanges = append(r.uidRanges, *uidRange)
}

func (r *fakeNamespaceSCCAllocMetrics) getLatestNamespaceProcessed() int {
	if len(r.namespacesProcessed) == 0 {
		return -1
	}
	return r.namespacesProcessed[len(r.namespacesProcessed)-1]
}

func (r *fakeNamespaceSCCAllocMetrics) getLatestUidRange() v1.RangeAllocation {
	if len(r.uidRanges) == 0 {
		return v1.RangeAllocation{}
	}
	return r.uidRanges[len(r.uidRanges)-1]
}

func (r *fakeNamespaceSCCAllocMetrics) getLatestAllocatedUid() uint64 {
	latestRange := r.getLatestUidRange()

	actualAllocatedInt := big.NewInt(0).SetBytes(latestRange.Data)
	return actualAllocatedInt.Uint64()
}
