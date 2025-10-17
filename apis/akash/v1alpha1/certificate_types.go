/*
Copyright 2024 The Akash Provider Authors.

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
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// CertificateParameters are the configurable fields of a Certificate.
type CertificateParameters struct {
	// Domains is the list of domain names for the certificate
	// +kubebuilder:validation:Required
	Domains []string `json:"domains"`

	// DeploymentRef references an optional associated Deployment CR (name/namespace)
	DeploymentRef *CertificateDeploymentReference `json:"deploymentRef,omitempty"`

	// AutoRenew enables automatic renewal of the certificate
	// +kubebuilder:default=true
	AutoRenew *bool `json:"autoRenew,omitempty"`

	// ValidityDays specifies the certificate validity period in days
	// +kubebuilder:default=365
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3650
	ValidityDays *int32 `json:"validityDays,omitempty"`
}

// CertificateDeploymentReference represents a reference to a Deployment resource
type CertificateDeploymentReference struct {
	// Name is the name of the Deployment resource
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace is the namespace of the Deployment resource
	Namespace string `json:"namespace,omitempty"`
}

// CertificateObservation are the observable fields of a Certificate.
type CertificateObservation struct {
	// Serial is the certificate serial number
	Serial string `json:"serial,omitempty"`

	// Owner is the certificate owner address (from ProviderConfig)
	Owner string `json:"owner,omitempty"`

	// Issuer contains certificate issuer information
	Issuer string `json:"issuer,omitempty"`

	// Subject contains certificate subject information
	Subject string `json:"subject,omitempty"`

	// NotBefore is the certificate validity start date (Unix timestamp)
	NotBefore int64 `json:"notBefore,omitempty"`

	// NotAfter is the certificate expiration date (Unix timestamp)
	NotAfter int64 `json:"notAfter,omitempty"`

	// State is the certificate state (valid, expired, revoked)
	State string `json:"state,omitempty"`

	// Fingerprint is the certificate fingerprint
	Fingerprint string `json:"fingerprint,omitempty"`

	// PEM is the certificate PEM content
	PEM string `json:"pem,omitempty"`

	// CreatedAt is when the certificate was created (Unix timestamp)
	CreatedAt int64 `json:"createdAt,omitempty"`

	// ExpiresAt is when the certificate expires (Unix timestamp)
	ExpiresAt int64 `json:"expiresAt,omitempty"`

	// LastRenewed is when the certificate was last renewed (Unix timestamp)
	LastRenewed int64 `json:"lastRenewed,omitempty"`
}

// A CertificateSpec defines the desired state of a Certificate.
type CertificateSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       CertificateParameters `json:"forProvider"`
}

// A CertificateStatus represents the observed state of a Certificate.
type CertificateStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          CertificateObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Certificate represents an Akash Network Certificate for TLS certificate management.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.atProvider.state"
// +kubebuilder:printcolumn:name="SERIAL",type="string",JSONPath=".status.atProvider.serial"
// +kubebuilder:printcolumn:name="DOMAINS",type="string",JSONPath=".spec.forProvider.domains[*]"
// +kubebuilder:printcolumn:name="EXPIRES",type="date",JSONPath=".status.atProvider.expiresAt"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,akash}
type Certificate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CertificateSpec   `json:"spec"`
	Status CertificateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CertificateList contains a list of Certificate
type CertificateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Certificate `json:"items"`
}

// Certificate type metadata.
var (
	CertificateKind             = reflect.TypeOf(Certificate{}).Name()
	CertificateGroupKind        = schema.GroupKind{Group: Group, Kind: CertificateKind}.String()
	CertificateKindAPIVersion   = CertificateKind + "." + SchemeGroupVersion.String()
	CertificateGroupVersionKind = SchemeGroupVersion.WithKind(CertificateKind)
)

func init() {
	SchemeBuilder.Register(&Certificate{}, &CertificateList{})
}

// GetCondition of this Certificate.
func (mg *Certificate) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return mg.Status.GetCondition(ct)
}

// GetDeletionPolicy of this Certificate.
func (mg *Certificate) GetDeletionPolicy() xpv1.DeletionPolicy {
	return mg.Spec.DeletionPolicy
}

// GetManagementPolicies of this Certificate.
func (mg *Certificate) GetManagementPolicies() xpv1.ManagementPolicies {
	return mg.Spec.ManagementPolicies
}

// GetProviderConfigReference of this Certificate.
func (mg *Certificate) GetProviderConfigReference() *xpv1.Reference {
	return mg.Spec.ProviderConfigReference
}

// GetPublishConnectionDetailsTo of this Certificate.
func (mg *Certificate) GetPublishConnectionDetailsTo() *xpv1.PublishConnectionDetailsTo {
	return mg.Spec.PublishConnectionDetailsTo
}

// GetWriteConnectionSecretToReference of this Certificate.
func (mg *Certificate) GetWriteConnectionSecretToReference() *xpv1.SecretReference {
	return mg.Spec.WriteConnectionSecretToReference
}

// SetConditions of this Certificate.
func (mg *Certificate) SetConditions(c ...xpv1.Condition) {
	mg.Status.SetConditions(c...)
}

// SetDeletionPolicy of this Certificate.
func (mg *Certificate) SetDeletionPolicy(r xpv1.DeletionPolicy) {
	mg.Spec.DeletionPolicy = r
}

// SetManagementPolicies of this Certificate.
func (mg *Certificate) SetManagementPolicies(r xpv1.ManagementPolicies) {
	mg.Spec.ManagementPolicies = r
}

// SetProviderConfigReference of this Certificate.
func (mg *Certificate) SetProviderConfigReference(r *xpv1.Reference) {
	mg.Spec.ProviderConfigReference = r
}

// SetPublishConnectionDetailsTo of this Certificate.
func (mg *Certificate) SetPublishConnectionDetailsTo(r *xpv1.PublishConnectionDetailsTo) {
	mg.Spec.PublishConnectionDetailsTo = r
}

// SetWriteConnectionSecretToReference of this Certificate.
func (mg *Certificate) SetWriteConnectionSecretToReference(r *xpv1.SecretReference) {
	mg.Spec.WriteConnectionSecretToReference = r
}