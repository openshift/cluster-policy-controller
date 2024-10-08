// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1

// ManagedBootImagesApplyConfiguration represents a declarative configuration of the ManagedBootImages type for use
// with apply.
type ManagedBootImagesApplyConfiguration struct {
	MachineManagers []MachineManagerApplyConfiguration `json:"machineManagers,omitempty"`
}

// ManagedBootImagesApplyConfiguration constructs a declarative configuration of the ManagedBootImages type for use with
// apply.
func ManagedBootImages() *ManagedBootImagesApplyConfiguration {
	return &ManagedBootImagesApplyConfiguration{}
}

// WithMachineManagers adds the given value to the MachineManagers field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the MachineManagers field.
func (b *ManagedBootImagesApplyConfiguration) WithMachineManagers(values ...*MachineManagerApplyConfiguration) *ManagedBootImagesApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithMachineManagers")
		}
		b.MachineManagers = append(b.MachineManagers, *values[i])
	}
	return b
}
