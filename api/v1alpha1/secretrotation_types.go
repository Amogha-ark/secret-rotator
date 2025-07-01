/*
Copyright 2025.

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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// WorkloadReference defines a workload that should be updated when secrets change
type WorkloadReference struct {
	// Kind is the workload kind (e.g., Deployment, StatefulSet, DaemonSet)
	Kind string `json:"kind"`
	// Name is the name of the workload
	Name string `json:"name"`
	// Namespace is the namespace of the workload (optional, defaults to SecretRotation namespace)
	Namespace string `json:"namespace,omitempty"`
}

// SecretRotationSpec defines desired state
type SecretRotationSpec struct {
	VaultPath    string `json:"vaultPath"`
	TargetSecret string `json:"targetSecret"`
	// TargetWorkloads are the workloads that should be updated when the secret changes
	TargetWorkloads []WorkloadReference `json:"targetWorkloads,omitempty"`
	// AnnotationPrefix is the prefix for the checksum annotation (defaults to "secrets.github.com/")
	AnnotationPrefix string `json:"annotationPrefix,omitempty"`
}

// SecretRotationStatus defines observed state (optional)
type SecretRotationStatus struct {
	LastRotation metav1.Time `json:"lastRotation,omitempty"`
	// SecretChecksum is the checksum of the current secret data
	SecretChecksum string `json:"secretChecksum,omitempty"`
	// UpdatedWorkloads tracks which workloads were successfully updated
	UpdatedWorkloads []string `json:"updatedWorkloads,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SecretRotation is the Schema for the secretrotations API
type SecretRotation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecretRotationSpec   `json:"spec,omitempty"`
	Status SecretRotationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SecretRotationList contains a list of SecretRotation
type SecretRotationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecretRotation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SecretRotation{}, &SecretRotationList{})
}
