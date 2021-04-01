package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

func init() {
	SchemeBuilder.Register(addKubernetesServiceToScheme)
}

func addKubernetesServiceToScheme(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&KubernetesService{},
		&KubernetesServiceList{})
	return nil
}

// +kubebuilder:resource:path=kubernetesservice,shortName=k8s,scope=Namespaced,categories=hypershift-lite
// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// KubernetesService is the Schema for the KubernetesService API
type KubernetesService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubernetesServiceSpec   `json:"spec,omitempty"`
	Status KubernetesServiceStatus `json:"status,omitempty"`
}

// KubernetesServiceSpec defines the desired state of KubernetesService
type KubernetesServiceSpec struct {
	// ReleaseImage is the pull spec of the release image to use for the API server components
	ReleaseImage string `json:"releaseImage"`

	// PullSecret is a local reference to a secret used to pull OpenShift images
	PullSecret corev1.LocalObjectReference `json:"pullSecret"`
}

// KubernetesServiceStatus defines the observed state of KubernetesService
type KubernetesServiceStatus struct {
	// Conditions contains details of the current state of the KubernetesService
	// +kubebuilder:validation:Required
	Conditions []KubernetesServiceCondition `json:"conditions"`
}

type ConditionType string

const (
	Available                      ConditionType = "Available"
	EtcdAvailable                  ConditionType = "EtcdAvailable"
	KubeAPIServerAvailable         ConditionType = "KubeAPIServerAvailable"
	KubeControllerManagerAvailable ConditionType = "KubeControllerManagerAvailable"
)

// KubernetesServiceCondition contains details of a specific status condition
type KubernetesServiceCondition struct {
	// type specifies the aspect reported by this condition.
	// +kubebuilder:validation:Required
	Type ConditionType `json:"type"`

	// status of the condition, one of True, False, Unknown.
	// +kubebuilder:validation:Required
	Status corev1.ConditionStatus `json:"status"`

	// lastTransitionTime is the time of the last update to the current status property.
	// +kubebuilder:validation:Required
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`

	// reason is the CamelCase reason for the condition's current status.
	// +kubebuilder:validation:Optional
	Reason string `json:"reason,omitempty"`

	// message provides additional information about the current condition.
	// This is only to be consumed by humans.  It may contain Line Feed
	// characters (U+000A), which should be rendered as new lines.
	// +kubebuilder:validation:Optional
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// KubernetesServiceList contains a list of KubernetesService.
type KubernetesServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubernetesService `json:"items"`
}
