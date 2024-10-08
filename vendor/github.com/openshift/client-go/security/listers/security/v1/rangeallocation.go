// Code generated by lister-gen. DO NOT EDIT.

package v1

import (
	v1 "github.com/openshift/api/security/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/listers"
	"k8s.io/client-go/tools/cache"
)

// RangeAllocationLister helps list RangeAllocations.
// All objects returned here must be treated as read-only.
type RangeAllocationLister interface {
	// List lists all RangeAllocations in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.RangeAllocation, err error)
	// Get retrieves the RangeAllocation from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1.RangeAllocation, error)
	RangeAllocationListerExpansion
}

// rangeAllocationLister implements the RangeAllocationLister interface.
type rangeAllocationLister struct {
	listers.ResourceIndexer[*v1.RangeAllocation]
}

// NewRangeAllocationLister returns a new RangeAllocationLister.
func NewRangeAllocationLister(indexer cache.Indexer) RangeAllocationLister {
	return &rangeAllocationLister{listers.New[*v1.RangeAllocation](indexer, v1.Resource("rangeallocation"))}
}
