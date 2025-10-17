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

// LeaseParameters are the configurable fields of a Lease.
type LeaseParameters struct {
	// DeploymentRef references the Deployment CR (name/namespace)
	// +kubebuilder:validation:Required
	DeploymentRef DeploymentReference `json:"deploymentRef"`

	// ActiveBidRef references the ActiveBid CR to accept (name/namespace)
	// +kubebuilder:validation:Required
	ActiveBidRef ActiveBidReference `json:"activeBidRef"`
}

// LeaseService represents a running service under a lease
type LeaseService struct {
	// Name is the service name
	Name string `json:"name"`

	// Available indicates if the service is available
	Available bool `json:"available"`

	// URIs contains the service URIs
	URIs []string `json:"uris,omitempty"`

	// Ports contains the service port mappings
	Ports []ServicePort `json:"ports,omitempty"`
}

// ServicePort represents a service port mapping
type ServicePort struct {
	// Port is the internal port
	Port int32 `json:"port"`

	// ExternalPort is the external port (if different)
	ExternalPort int32 `json:"externalPort,omitempty"`

	// Protocol is the port protocol (TCP, UDP)
	Protocol string `json:"protocol,omitempty"`

	// Host is the host for this port
	Host string `json:"host,omitempty"`
}

// LeaseObservation are the observable fields of a Lease.
type LeaseObservation struct {
	// LeaseId is the unique identifier for the lease on Akash network
	LeaseId string `json:"leaseId,omitempty"`

	// Owner is the lease owner address (resolved from deploymentRef)
	Owner string `json:"owner,omitempty"`

	// Dseq is the deployment sequence number (resolved from deploymentRef)
	Dseq string `json:"dseq,omitempty"`

	// Gseq is the group sequence number (resolved from activeBidRef)
	Gseq string `json:"gseq,omitempty"`

	// Oseq is the order sequence number (resolved from activeBidRef)
	Oseq string `json:"oseq,omitempty"`

	// Provider is the provider address (resolved from activeBidRef)
	Provider string `json:"provider,omitempty"`

	// State is the current lease state (active, closed)
	State string `json:"state,omitempty"`

	// Price contains the lease price information (from accepted bid)
	Price *ActiveBidPriceStatus `json:"price,omitempty"`

	// CreatedAt is the lease creation timestamp
	CreatedAt int64 `json:"createdAt,omitempty"`

	// Services contains running services information
	Services []LeaseService `json:"services,omitempty"`
}

// A LeaseSpec defines the desired state of a Lease.
type LeaseSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       LeaseParameters `json:"forProvider"`
}

// A LeaseStatus represents the observed state of a Lease.
type LeaseStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          LeaseObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Lease represents an Akash Network Lease for managing deployments.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="LEASE-ID",type="string",JSONPath=".status.atProvider.leaseId"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.atProvider.status.state"
// +kubebuilder:printcolumn:name="PROVIDER",type="string",JSONPath=".status.atProvider.provider"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,akash}
type Lease struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LeaseSpec   `json:"spec"`
	Status LeaseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LeaseList contains a list of Lease
type LeaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Lease `json:"items"`
}

// Lease type metadata.
var (
	LeaseKind             = reflect.TypeOf(Lease{}).Name()
	LeaseGroupKind        = schema.GroupKind{Group: Group, Kind: LeaseKind}.String()
	LeaseKindAPIVersion   = LeaseKind + "." + SchemeGroupVersion.String()
	LeaseGroupVersionKind = SchemeGroupVersion.WithKind(LeaseKind)
)

func init() {
	SchemeBuilder.Register(&Lease{}, &LeaseList{})
}

// GetCondition of this Lease.
func (mg *Lease) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return mg.Status.GetCondition(ct)
}

// GetDeletionPolicy of this Lease.
func (mg *Lease) GetDeletionPolicy() xpv1.DeletionPolicy {
	return mg.Spec.DeletionPolicy
}

// GetManagementPolicies of this Lease.
func (mg *Lease) GetManagementPolicies() xpv1.ManagementPolicies {
	return mg.Spec.ManagementPolicies
}

// GetProviderConfigReference of this Lease.
func (mg *Lease) GetProviderConfigReference() *xpv1.Reference {
	return mg.Spec.ProviderConfigReference
}

// GetPublishConnectionDetailsTo of this Lease.
func (mg *Lease) GetPublishConnectionDetailsTo() *xpv1.PublishConnectionDetailsTo {
	return mg.Spec.PublishConnectionDetailsTo
}

// GetWriteConnectionSecretToReference of this Lease.
func (mg *Lease) GetWriteConnectionSecretToReference() *xpv1.SecretReference {
	return mg.Spec.WriteConnectionSecretToReference
}

// SetConditions of this Lease.
func (mg *Lease) SetConditions(c ...xpv1.Condition) {
	mg.Status.SetConditions(c...)
}

// SetDeletionPolicy of this Lease.
func (mg *Lease) SetDeletionPolicy(r xpv1.DeletionPolicy) {
	mg.Spec.DeletionPolicy = r
}

// SetManagementPolicies of this Lease.
func (mg *Lease) SetManagementPolicies(r xpv1.ManagementPolicies) {
	mg.Spec.ManagementPolicies = r
}

// SetProviderConfigReference of this Lease.
func (mg *Lease) SetProviderConfigReference(r *xpv1.Reference) {
	mg.Spec.ProviderConfigReference = r
}

// SetPublishConnectionDetailsTo of this Lease.
func (mg *Lease) SetPublishConnectionDetailsTo(r *xpv1.PublishConnectionDetailsTo) {
	mg.Spec.PublishConnectionDetailsTo = r
}

// SetWriteConnectionSecretToReference of this Lease.
func (mg *Lease) SetWriteConnectionSecretToReference(r *xpv1.SecretReference) {
	mg.Spec.WriteConnectionSecretToReference = r
}