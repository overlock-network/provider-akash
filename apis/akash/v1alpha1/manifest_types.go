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

// ManifestParameters are the configurable fields of a Manifest.
type ManifestParameters struct {
	// LeaseRef references the Lease CR (name/namespace)
	// +kubebuilder:validation:Required
	LeaseRef ManifestLeaseReference `json:"leaseRef"`

	// +kubebuilder:validation:Required
	CertificateSecretRef ManifestSecretReference `json:"certificateSecretRef"`
}

type ManifestSecretReference struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	Namespace string `json:"namespace,omitempty"`
}

// ManifestLeaseReference represents a simple reference to a Lease resource
type ManifestLeaseReference struct {
	// Name is the name of the Lease resource
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace is the namespace of the Lease resource
	Namespace string `json:"namespace,omitempty"`
}

// ManifestService represents a service defined in the manifest
type ManifestService struct {
	// Name is the service name
	Name string `json:"name"`

	// Image is the container image
	Image string `json:"image,omitempty"`

	// Command is the container command
	Command []string `json:"command,omitempty"`

	// Args are the container arguments
	Args []string `json:"args,omitempty"`

	// Env contains environment variables
	Env []ManifestEnvVar `json:"env,omitempty"`

	// Resources contains resource requirements
	Resources *ManifestResources `json:"resources,omitempty"`

	// Expose contains port exposure configuration
	Expose []ManifestExpose `json:"expose,omitempty"`
}

// ManifestEnvVar represents an environment variable
type ManifestEnvVar struct {
	// Name is the environment variable name
	Name string `json:"name"`

	// Value is the environment variable value
	Value string `json:"value"`
}

// ManifestResources represents resource requirements
type ManifestResources struct {
	// CPU resource requirement
	CPU string `json:"cpu,omitempty"`

	// Memory resource requirement
	Memory string `json:"memory,omitempty"`

	// Storage resource requirement
	Storage string `json:"storage,omitempty"`

	// GPU resource requirement
	GPU string `json:"gpu,omitempty"`
}

// ManifestExpose represents port exposure configuration
type ManifestExpose struct {
	// Port is the container port
	Port int32 `json:"port"`

	// Proto is the protocol (TCP, UDP)
	Proto string `json:"proto,omitempty"`

	// Service is the service type
	Service string `json:"service,omitempty"`

	// Global indicates if the port should be globally accessible
	Global bool `json:"global,omitempty"`

	// Hosts contains the hostnames for this port
	Hosts []string `json:"hosts,omitempty"`
}

// ManifestValidationError represents a validation error from the provider
type ManifestValidationError struct {
	// Field is the field that caused the error
	Field string `json:"field,omitempty"`

	// Message is the error message
	Message string `json:"message"`

	// Code is the error code
	Code string `json:"code,omitempty"`
}

// ManifestObservation are the observable fields of a Manifest.
type ManifestObservation struct {
	// Owner is the deployment owner (resolved from leaseRef)
	Owner string `json:"owner,omitempty"`

	// Dseq is the deployment sequence number (resolved from leaseRef)
	Dseq string `json:"dseq,omitempty"`

	// Gseq is the group sequence number (resolved from leaseRef)
	Gseq string `json:"gseq,omitempty"`

	// Oseq is the order sequence number (resolved from leaseRef)
	Oseq string `json:"oseq,omitempty"`

	// Provider is the provider address (resolved from leaseRef)
	Provider string `json:"provider,omitempty"`

	// SdlContent is the rendered SDL content sent to provider
	SdlContent string `json:"sdlContent,omitempty"`

	// ManifestVersion is the version/hash of deployed manifest
	ManifestVersion string `json:"manifestVersion,omitempty"`

	// State is the manifest state (pending, deployed, failed)
	State string `json:"state,omitempty"`

	// DeployedAt is when manifest was sent to provider
	DeployedAt int64 `json:"deployedAt,omitempty"`

	// Services contains the list of services defined in manifest
	Services []ManifestService `json:"services,omitempty"`

	// ValidationErrors contains any validation errors from provider
	ValidationErrors []ManifestValidationError `json:"validationErrors,omitempty"`

	// ProviderResponse contains raw response from provider
	ProviderResponse string `json:"providerResponse,omitempty"`
}

// A ManifestSpec defines the desired state of a Manifest.
type ManifestSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       ManifestParameters `json:"forProvider"`
}

// A ManifestStatus represents the observed state of a Manifest.
type ManifestStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          ManifestObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Manifest represents an Akash Network Manifest for deployment configuration management.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.atProvider.state"
// +kubebuilder:printcolumn:name="PROVIDER",type="string",JSONPath=".status.atProvider.provider"
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".status.atProvider.manifestVersion"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,akash}
type Manifest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManifestSpec   `json:"spec"`
	Status ManifestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ManifestList contains a list of Manifest
type ManifestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Manifest `json:"items"`
}

// Manifest type metadata.
var (
	ManifestKind             = reflect.TypeOf(Manifest{}).Name()
	ManifestGroupKind        = schema.GroupKind{Group: Group, Kind: ManifestKind}.String()
	ManifestKindAPIVersion   = ManifestKind + "." + SchemeGroupVersion.String()
	ManifestGroupVersionKind = SchemeGroupVersion.WithKind(ManifestKind)
)

func init() {
	SchemeBuilder.Register(&Manifest{}, &ManifestList{})
}

// GetCondition of this Manifest.
func (mg *Manifest) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return mg.Status.GetCondition(ct)
}

// GetDeletionPolicy of this Manifest.
func (mg *Manifest) GetDeletionPolicy() xpv1.DeletionPolicy {
	return mg.Spec.DeletionPolicy
}

// GetManagementPolicies of this Manifest.
func (mg *Manifest) GetManagementPolicies() xpv1.ManagementPolicies {
	return mg.Spec.ManagementPolicies
}

// GetProviderConfigReference of this Manifest.
func (mg *Manifest) GetProviderConfigReference() *xpv1.Reference {
	return mg.Spec.ProviderConfigReference
}

// GetPublishConnectionDetailsTo of this Manifest.
func (mg *Manifest) GetPublishConnectionDetailsTo() *xpv1.PublishConnectionDetailsTo {
	return mg.Spec.PublishConnectionDetailsTo
}

// GetWriteConnectionSecretToReference of this Manifest.
func (mg *Manifest) GetWriteConnectionSecretToReference() *xpv1.SecretReference {
	return mg.Spec.WriteConnectionSecretToReference
}

// SetConditions of this Manifest.
func (mg *Manifest) SetConditions(c ...xpv1.Condition) {
	mg.Status.SetConditions(c...)
}

// SetDeletionPolicy of this Manifest.
func (mg *Manifest) SetDeletionPolicy(r xpv1.DeletionPolicy) {
	mg.Spec.DeletionPolicy = r
}

// SetManagementPolicies of this Manifest.
func (mg *Manifest) SetManagementPolicies(r xpv1.ManagementPolicies) {
	mg.Spec.ManagementPolicies = r
}

// SetProviderConfigReference of this Manifest.
func (mg *Manifest) SetProviderConfigReference(r *xpv1.Reference) {
	mg.Spec.ProviderConfigReference = r
}

// SetPublishConnectionDetailsTo of this Manifest.
func (mg *Manifest) SetPublishConnectionDetailsTo(r *xpv1.PublishConnectionDetailsTo) {
	mg.Spec.PublishConnectionDetailsTo = r
}

// SetWriteConnectionSecretToReference of this Manifest.
func (mg *Manifest) SetWriteConnectionSecretToReference(r *xpv1.SecretReference) {
	mg.Spec.WriteConnectionSecretToReference = r
}