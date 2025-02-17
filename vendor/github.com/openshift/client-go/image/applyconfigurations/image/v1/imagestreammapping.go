// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1

import (
	imagev1 "github.com/openshift/api/image/v1"
	internal "github.com/openshift/client-go/image/applyconfigurations/internal"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	managedfields "k8s.io/apimachinery/pkg/util/managedfields"
	metav1 "k8s.io/client-go/applyconfigurations/meta/v1"
)

// ImageStreamMappingApplyConfiguration represents a declarative configuration of the ImageStreamMapping type for use
// with apply.
type ImageStreamMappingApplyConfiguration struct {
	metav1.TypeMetaApplyConfiguration    `json:",inline"`
	*metav1.ObjectMetaApplyConfiguration `json:"metadata,omitempty"`
	Image                                *ImageApplyConfiguration `json:"image,omitempty"`
	Tag                                  *string                  `json:"tag,omitempty"`
}

// ImageStreamMapping constructs a declarative configuration of the ImageStreamMapping type for use with
// apply.
func ImageStreamMapping(name, namespace string) *ImageStreamMappingApplyConfiguration {
	b := &ImageStreamMappingApplyConfiguration{}
	b.WithName(name)
	b.WithNamespace(namespace)
	b.WithKind("ImageStreamMapping")
	b.WithAPIVersion("image.openshift.io/v1")
	return b
}

// ExtractImageStreamMapping extracts the applied configuration owned by fieldManager from
// imageStreamMapping. If no managedFields are found in imageStreamMapping for fieldManager, a
// ImageStreamMappingApplyConfiguration is returned with only the Name, Namespace (if applicable),
// APIVersion and Kind populated. It is possible that no managed fields were found for because other
// field managers have taken ownership of all the fields previously owned by fieldManager, or because
// the fieldManager never owned fields any fields.
// imageStreamMapping must be a unmodified ImageStreamMapping API object that was retrieved from the Kubernetes API.
// ExtractImageStreamMapping provides a way to perform a extract/modify-in-place/apply workflow.
// Note that an extracted apply configuration will contain fewer fields than what the fieldManager previously
// applied if another fieldManager has updated or force applied any of the previously applied fields.
// Experimental!
func ExtractImageStreamMapping(imageStreamMapping *imagev1.ImageStreamMapping, fieldManager string) (*ImageStreamMappingApplyConfiguration, error) {
	return extractImageStreamMapping(imageStreamMapping, fieldManager, "")
}

// ExtractImageStreamMappingStatus is the same as ExtractImageStreamMapping except
// that it extracts the status subresource applied configuration.
// Experimental!
func ExtractImageStreamMappingStatus(imageStreamMapping *imagev1.ImageStreamMapping, fieldManager string) (*ImageStreamMappingApplyConfiguration, error) {
	return extractImageStreamMapping(imageStreamMapping, fieldManager, "status")
}

func extractImageStreamMapping(imageStreamMapping *imagev1.ImageStreamMapping, fieldManager string, subresource string) (*ImageStreamMappingApplyConfiguration, error) {
	b := &ImageStreamMappingApplyConfiguration{}
	err := managedfields.ExtractInto(imageStreamMapping, internal.Parser().Type("com.github.openshift.api.image.v1.ImageStreamMapping"), fieldManager, b, subresource)
	if err != nil {
		return nil, err
	}
	b.WithName(imageStreamMapping.Name)
	b.WithNamespace(imageStreamMapping.Namespace)

	b.WithKind("ImageStreamMapping")
	b.WithAPIVersion("image.openshift.io/v1")
	return b, nil
}

// WithKind sets the Kind field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Kind field is set to the value of the last call.
func (b *ImageStreamMappingApplyConfiguration) WithKind(value string) *ImageStreamMappingApplyConfiguration {
	b.TypeMetaApplyConfiguration.Kind = &value
	return b
}

// WithAPIVersion sets the APIVersion field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the APIVersion field is set to the value of the last call.
func (b *ImageStreamMappingApplyConfiguration) WithAPIVersion(value string) *ImageStreamMappingApplyConfiguration {
	b.TypeMetaApplyConfiguration.APIVersion = &value
	return b
}

// WithName sets the Name field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Name field is set to the value of the last call.
func (b *ImageStreamMappingApplyConfiguration) WithName(value string) *ImageStreamMappingApplyConfiguration {
	b.ensureObjectMetaApplyConfigurationExists()
	b.ObjectMetaApplyConfiguration.Name = &value
	return b
}

// WithGenerateName sets the GenerateName field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the GenerateName field is set to the value of the last call.
func (b *ImageStreamMappingApplyConfiguration) WithGenerateName(value string) *ImageStreamMappingApplyConfiguration {
	b.ensureObjectMetaApplyConfigurationExists()
	b.ObjectMetaApplyConfiguration.GenerateName = &value
	return b
}

// WithNamespace sets the Namespace field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Namespace field is set to the value of the last call.
func (b *ImageStreamMappingApplyConfiguration) WithNamespace(value string) *ImageStreamMappingApplyConfiguration {
	b.ensureObjectMetaApplyConfigurationExists()
	b.ObjectMetaApplyConfiguration.Namespace = &value
	return b
}

// WithUID sets the UID field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the UID field is set to the value of the last call.
func (b *ImageStreamMappingApplyConfiguration) WithUID(value types.UID) *ImageStreamMappingApplyConfiguration {
	b.ensureObjectMetaApplyConfigurationExists()
	b.ObjectMetaApplyConfiguration.UID = &value
	return b
}

// WithResourceVersion sets the ResourceVersion field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ResourceVersion field is set to the value of the last call.
func (b *ImageStreamMappingApplyConfiguration) WithResourceVersion(value string) *ImageStreamMappingApplyConfiguration {
	b.ensureObjectMetaApplyConfigurationExists()
	b.ObjectMetaApplyConfiguration.ResourceVersion = &value
	return b
}

// WithGeneration sets the Generation field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Generation field is set to the value of the last call.
func (b *ImageStreamMappingApplyConfiguration) WithGeneration(value int64) *ImageStreamMappingApplyConfiguration {
	b.ensureObjectMetaApplyConfigurationExists()
	b.ObjectMetaApplyConfiguration.Generation = &value
	return b
}

// WithCreationTimestamp sets the CreationTimestamp field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the CreationTimestamp field is set to the value of the last call.
func (b *ImageStreamMappingApplyConfiguration) WithCreationTimestamp(value apismetav1.Time) *ImageStreamMappingApplyConfiguration {
	b.ensureObjectMetaApplyConfigurationExists()
	b.ObjectMetaApplyConfiguration.CreationTimestamp = &value
	return b
}

// WithDeletionTimestamp sets the DeletionTimestamp field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the DeletionTimestamp field is set to the value of the last call.
func (b *ImageStreamMappingApplyConfiguration) WithDeletionTimestamp(value apismetav1.Time) *ImageStreamMappingApplyConfiguration {
	b.ensureObjectMetaApplyConfigurationExists()
	b.ObjectMetaApplyConfiguration.DeletionTimestamp = &value
	return b
}

// WithDeletionGracePeriodSeconds sets the DeletionGracePeriodSeconds field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the DeletionGracePeriodSeconds field is set to the value of the last call.
func (b *ImageStreamMappingApplyConfiguration) WithDeletionGracePeriodSeconds(value int64) *ImageStreamMappingApplyConfiguration {
	b.ensureObjectMetaApplyConfigurationExists()
	b.ObjectMetaApplyConfiguration.DeletionGracePeriodSeconds = &value
	return b
}

// WithLabels puts the entries into the Labels field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, the entries provided by each call will be put on the Labels field,
// overwriting an existing map entries in Labels field with the same key.
func (b *ImageStreamMappingApplyConfiguration) WithLabels(entries map[string]string) *ImageStreamMappingApplyConfiguration {
	b.ensureObjectMetaApplyConfigurationExists()
	if b.ObjectMetaApplyConfiguration.Labels == nil && len(entries) > 0 {
		b.ObjectMetaApplyConfiguration.Labels = make(map[string]string, len(entries))
	}
	for k, v := range entries {
		b.ObjectMetaApplyConfiguration.Labels[k] = v
	}
	return b
}

// WithAnnotations puts the entries into the Annotations field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, the entries provided by each call will be put on the Annotations field,
// overwriting an existing map entries in Annotations field with the same key.
func (b *ImageStreamMappingApplyConfiguration) WithAnnotations(entries map[string]string) *ImageStreamMappingApplyConfiguration {
	b.ensureObjectMetaApplyConfigurationExists()
	if b.ObjectMetaApplyConfiguration.Annotations == nil && len(entries) > 0 {
		b.ObjectMetaApplyConfiguration.Annotations = make(map[string]string, len(entries))
	}
	for k, v := range entries {
		b.ObjectMetaApplyConfiguration.Annotations[k] = v
	}
	return b
}

// WithOwnerReferences adds the given value to the OwnerReferences field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the OwnerReferences field.
func (b *ImageStreamMappingApplyConfiguration) WithOwnerReferences(values ...*metav1.OwnerReferenceApplyConfiguration) *ImageStreamMappingApplyConfiguration {
	b.ensureObjectMetaApplyConfigurationExists()
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithOwnerReferences")
		}
		b.ObjectMetaApplyConfiguration.OwnerReferences = append(b.ObjectMetaApplyConfiguration.OwnerReferences, *values[i])
	}
	return b
}

// WithFinalizers adds the given value to the Finalizers field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Finalizers field.
func (b *ImageStreamMappingApplyConfiguration) WithFinalizers(values ...string) *ImageStreamMappingApplyConfiguration {
	b.ensureObjectMetaApplyConfigurationExists()
	for i := range values {
		b.ObjectMetaApplyConfiguration.Finalizers = append(b.ObjectMetaApplyConfiguration.Finalizers, values[i])
	}
	return b
}

func (b *ImageStreamMappingApplyConfiguration) ensureObjectMetaApplyConfigurationExists() {
	if b.ObjectMetaApplyConfiguration == nil {
		b.ObjectMetaApplyConfiguration = &metav1.ObjectMetaApplyConfiguration{}
	}
}

// WithImage sets the Image field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Image field is set to the value of the last call.
func (b *ImageStreamMappingApplyConfiguration) WithImage(value *ImageApplyConfiguration) *ImageStreamMappingApplyConfiguration {
	b.Image = value
	return b
}

// WithTag sets the Tag field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Tag field is set to the value of the last call.
func (b *ImageStreamMappingApplyConfiguration) WithTag(value string) *ImageStreamMappingApplyConfiguration {
	b.Tag = &value
	return b
}

// GetName retrieves the value of the Name field in the declarative configuration.
func (b *ImageStreamMappingApplyConfiguration) GetName() *string {
	b.ensureObjectMetaApplyConfigurationExists()
	return b.ObjectMetaApplyConfiguration.Name
}
