// Copyright 2022 VaultOperator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:validation:Enum=string;bytes;password;rsa;ecdsa;uuid
type VaultSecretGeneratorName string

// Configuration of secret generation
type VaultSecretGenerator struct {
	//
	// +kubebuilder:validation:Required
	Name VaultSecretGeneratorName `json:"name"`
	//
	// +kubebuilder:validation:Required
	Args []int32 `json:"args"`
}

type VaultSecretLocation struct {
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Path string `json:"path"`
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Field string `json:"field"`
	//
	// +optional
	Version int `json:"version"`
	//
	// +optional
	IsBinary bool `json:"isBinary"`
}

type VaultSecretVariable struct {
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	//
	// +optional
	Generator *VaultSecretGenerator `json:"generator,omitempty"`
	//
	// +kubebuilder:validation:Required
	Location *VaultSecretLocation `json:"location,omitempty"`
}

// Definition of a single data definiton
type VaultSecretData struct {
	// Associated key name for the created secret data.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	//
	// +optional
	Generator *VaultSecretGenerator `json:"generator,omitempty"`
	//
	// +optional
	Location *VaultSecretLocation `json:"location,omitempty"`
	//
	// +optional
	Variables []VaultSecretVariable `json:"variables,omitempty"`
	//
	// +optional
	Template string `json:"template,omitempty"`
}

// +kubebuilder:validation:Enum=Ignore;Overwrite;Error
type FieldCollisionStrategy string

const (
	// Errors if a field on this vault secret already exists on the resulting K8s secret.
	ErrorOnCollision FieldCollisionStrategy = "Error"

	// Value from this vault secret will be ignored if the same field already exists on resulting K8s secret.
	IgnoreCollision FieldCollisionStrategy = "Ignore"

	// Value from this vault secret will override an already existing field on the resulting K8s secret.
	OverwriteCollision FieldCollisionStrategy = "Overwrite"
)

// Definition of a vault path reference to gather secrets from.
type VaultSecretDataRef struct {
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Path string `json:"path"`
	//
	// +optional
	Version int `json:"version"`
	// Define how collisions with secrets from other vault references should be handled.
	// Valid values are:
	// - "Error" (default): Errors if a field on this vault secret already exists on the resulting K8s secret;
	// - "Ignore": Value from this vault secret will be ignored if the same field already exists on resulting K8s secret;
	// - "Overwrite": Value from this vault secret will override an already existing field on the resulting K8s secret
	// +optional
	// +kubebuilder:validation:Enum=Error;Ignore;Overwrite
	CollisionStrategy FieldCollisionStrategy `json:"collisionStrategy,omitempty"`
}

// VaultSecretSpec defines the desired state of VaultSecret
type VaultSecretSpec struct {
	// Optional name of secret which is created by this object.
	// +optional
	SecretName string `json:"secretName,omitempty"`
	// Optional type of secret which is created by this object.
	// +optional
	SecretType corev1.SecretType `json:"secretType,omitempty"`
	// Array of data definitions for the secret.
	// +optional
	Data []VaultSecretData `json:"data,omitempty"`
	// Array of vault path references where to gather data from for the secret.
	// +optional
	DataFrom []VaultSecretDataRef `json:"dataFrom,omitempty"`
}

// VaultSecretStatus defines the observed state of VaultSecret
type VaultSecretStatus struct {
	// Reference to the created secret object.
	// +optional
	SecretObject *corev1.ObjectReference `json:"active,omitempty"`
}

// +kubebuilder:object:root=true

// VaultSecret is the Schema for the vaultsecrets API
type VaultSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VaultSecretSpec   `json:"spec,omitempty"`
	Status VaultSecretStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VaultSecretList contains a list of VaultSecret
type VaultSecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VaultSecret `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VaultSecret{}, &VaultSecretList{})
}
