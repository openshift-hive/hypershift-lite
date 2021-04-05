// Package v1alpha1 contains API Schema definitions for the hypershiftlite.openshift.io v1alpha1 API group
// +kubebuilder:object:generate=true
// +groupName=hypershiftlite.openshift.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupName is the name of this API group
	GroupName = "network.operator.openshift.io"
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "hypershiftlite.openshift.io", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = runtime.NewSchemeBuilder()

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func init() {
	SchemeBuilder.Register(addGroupVersionToScheme)
}

func addGroupVersionToScheme(scheme *runtime.Scheme) error {
	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}
