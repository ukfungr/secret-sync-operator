/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// SecretSyncSpec defines the desired state of SecretSync
type SecretSyncSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	Provider ProviderSpec `json:"provider"`
	Remote   RemoteSpec   `json:"remote"`
	Target   TargetSpec   `json:"target"`
}

type ProviderSpec struct {
	// +kubebuilder:validation:MinLength=1
	Type string `json:"type"`
}

type RemoteSpec struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

type TargetSpec struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// SecretSyncStatus defines the observed state of SecretSync.
type SecretSyncStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the SecretSync resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SecretSync is the Schema for the secretsyncs API
type SecretSync struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of SecretSync
	// +required
	Spec SecretSyncSpec `json:"spec"`

	// status defines the observed state of SecretSync
	// +optional
	Status SecretSyncStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// SecretSyncList contains a list of SecretSync
type SecretSyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []SecretSync `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &SecretSync{}, &SecretSyncList{})
		return nil
	})
}
