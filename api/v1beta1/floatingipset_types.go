/*
Copyright 2020 Ryo TAKAISHI.

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FloatingIPSetSpec defines the desired state of FloatingIPSet
type FloatingIPSetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	NodeSelector map[string]string `json:"nodeSelector"`
	Network      string            `json:"network"`
}

// FloatingIPSetStatus defines the observed state of FloatingIPSet
type FloatingIPSetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Nodes []string `json:"nodes"`
}

// +kubebuilder:object:root=true

// FloatingIPSet is the Schema for the floatingipsets API
type FloatingIPSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FloatingIPSetSpec   `json:"spec,omitempty"`
	Status FloatingIPSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FloatingIPSetList contains a list of FloatingIPSet
type FloatingIPSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FloatingIPSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FloatingIPSet{}, &FloatingIPSetList{})
}