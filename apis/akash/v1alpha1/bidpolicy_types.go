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

// BidPolicyParameters are the configurable fields of a BidPolicy.
type BidPolicyParameters struct {
	// Selector defines label selector to match Deployment CRs
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// DeploymentRef references a specific deployment (overrides selector)
	// +optional
	DeploymentRef *DeploymentReference `json:"deploymentRef,omitempty"`

	// AutoAccept enables automatic lease creation when bids are selected
	// +kubebuilder:default=false
	AutoAccept bool `json:"autoAccept,omitempty"`

	// MaxPrice is the maximum acceptable price per block in uAKT
	// +kubebuilder:validation:Minimum=1
	// +optional
	MaxPrice *int64 `json:"maxPrice,omitempty"`

	// MinProviderScore is the minimum provider reputation score (0-100)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	MinProviderScore *int32 `json:"minProviderScore,omitempty"`

	// RequiredAttributes is a list of required provider attributes
	// +optional
	RequiredAttributes []ProviderAttribute `json:"requiredAttributes,omitempty"`

	// ExcludedProviders is a list of provider addresses to exclude
	// +optional
	ExcludedProviders []string `json:"excludedProviders,omitempty"`

	// PreferredProviders is a list of preferred provider addresses (higher priority)
	// +optional
	PreferredProviders []string `json:"preferredProviders,omitempty"`

	// SelectionStrategy defines how to select from qualifying bids
	// +kubebuilder:validation:Enum=lowest-price;best-score;preferred-first
	// +kubebuilder:default="lowest-price"
	SelectionStrategy string `json:"selectionStrategy,omitempty"`

	// WaitTime is deprecated. Bid collection timing is now managed automatically
	// using Kubernetes controller-runtime mechanisms based on deployment creation time.
	// +kubebuilder:validation:Minimum=10
	// +kubebuilder:validation:Maximum=3600
	// +kubebuilder:default=120
	// +deprecated
	WaitTime *int32 `json:"waitTime,omitempty"`

	// MaxBids is the maximum number of bids to wait for before selection
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=10
	MaxBids *int32 `json:"maxBids,omitempty"`
}

// ProviderAttribute defines a required provider attribute
type ProviderAttribute struct {
	// Key is the attribute key (e.g., "region", "tier", "features")
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// Value is the required attribute value
	// +kubebuilder:validation:Required
	Value string `json:"value"`
}

// BidPolicyObservation are the observable fields of a BidPolicy.
type BidPolicyObservation struct {
	// MatchedDeployments is a list of Deployment CRs matched by selector
	MatchedDeployments []DeploymentReference `json:"matchedDeployments,omitempty"`

	// ActiveBidsManaged is a list of ActiveBid CRs being managed
	ActiveBidsManaged []ActiveBidReference `json:"activeBidsManaged,omitempty"`

	// TotalBidsReceived is the number of bids received across all deployments
	TotalBidsReceived int32 `json:"totalBidsReceived,omitempty"`

	// EligibleBids is the number of bids meeting criteria
	EligibleBids int32 `json:"eligibleBids,omitempty"`

	// SelectedBids is a map of deployment to selected ActiveBid CR reference
	SelectedBids map[string]ActiveBidReference `json:"selectedBids,omitempty"`

	// CreatedLeases is a map of deployment to created Lease CR reference
	CreatedLeases map[string]LeaseReference `json:"createdLeases,omitempty"`

	// SelectionReasons is a map of deployment to reason for bid selection
	SelectionReasons map[string]string `json:"selectionReasons,omitempty"`

	// RejectedBids is a list of rejected bids with reasons
	RejectedBids []RejectedBidInfo `json:"rejectedBids,omitempty"`

	// State is the policy state (active, paused, failed)
	State string `json:"state,omitempty"`

	// LastEvaluated is the timestamp of last evaluation
	LastEvaluated *metav1.Time `json:"lastEvaluated,omitempty"`
}

// ActiveBidReference contains a reference to an ActiveBid resource
type ActiveBidReference struct {
	// Name is the name of the ActiveBid resource
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace is the namespace of the ActiveBid resource
	Namespace string `json:"namespace,omitempty"`

	// BidId is the bid ID associated with this ActiveBid
	BidId string `json:"bidId,omitempty"`

	// Provider is the provider address for this bid
	Provider string `json:"provider,omitempty"`

	// Price is the bid price in uAKT
	Price int64 `json:"price,omitempty"`

	// CreatedAt is when this ActiveBid was created
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`
}

// LeaseReference contains a reference to a Lease resource
type LeaseReference struct {
	// Name is the name of the Lease resource
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace is the namespace of the Lease resource
	Namespace string `json:"namespace,omitempty"`

	// LeaseId is the lease ID from Akash
	LeaseId string `json:"leaseId,omitempty"`

	// CreatedAt is when this Lease was created
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`
}

// RejectedBidInfo contains information about a rejected bid
type RejectedBidInfo struct {
	// BidId is the unique identifier of the rejected bid
	BidId string `json:"bidId"`

	// Provider is the address of the provider
	Provider string `json:"provider"`

	// Price is the bid price in uAKT
	Price int64 `json:"price"`

	// RejectionReason is why this bid was rejected
	RejectionReason string `json:"rejectionReason"`

	// RejectedAt is when this bid was rejected
	RejectedAt *metav1.Time `json:"rejectedAt"`
}

// A BidPolicySpec defines the desired state of a BidPolicy.
type BidPolicySpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       BidPolicyParameters `json:"forProvider"`
}

// A BidPolicyStatus represents the observed state of a BidPolicy.
type BidPolicyStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          BidPolicyObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A BidPolicy defines bid selection policies for Akash deployments.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.atProvider.state"
// +kubebuilder:printcolumn:name="DEPLOYMENT",type="string",JSONPath=".spec.forProvider.deploymentRef.name"
// +kubebuilder:printcolumn:name="AUTO-ACCEPT",type="boolean",JSONPath=".spec.forProvider.autoAccept"
// +kubebuilder:printcolumn:name="SELECTED-BID",type="string",JSONPath=".status.atProvider.selectedBid.bidId"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,akash}
type BidPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BidPolicySpec   `json:"spec"`
	Status BidPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BidPolicyList contains a list of BidPolicy
type BidPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BidPolicy `json:"items"`
}

// BidPolicy type metadata.
var (
	BidPolicyKind             = reflect.TypeOf(BidPolicy{}).Name()
	BidPolicyGroupKind        = schema.GroupKind{Group: Group, Kind: BidPolicyKind}.String()
	BidPolicyKindAPIVersion   = BidPolicyKind + "." + SchemeGroupVersion.String()
	BidPolicyGroupVersionKind = SchemeGroupVersion.WithKind(BidPolicyKind)
)

func init() {
	SchemeBuilder.Register(&BidPolicy{}, &BidPolicyList{})
}

// GetCondition of this BidPolicy.
func (mg *BidPolicy) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return mg.Status.GetCondition(ct)
}

// GetDeletionPolicy of this BidPolicy.
func (mg *BidPolicy) GetDeletionPolicy() xpv1.DeletionPolicy {
	return mg.Spec.DeletionPolicy
}

// GetManagementPolicies of this BidPolicy.
func (mg *BidPolicy) GetManagementPolicies() xpv1.ManagementPolicies {
	return mg.Spec.ManagementPolicies
}

// GetProviderConfigReference of this BidPolicy.
func (mg *BidPolicy) GetProviderConfigReference() *xpv1.Reference {
	return mg.Spec.ProviderConfigReference
}

// GetPublishConnectionDetailsTo of this BidPolicy.
func (mg *BidPolicy) GetPublishConnectionDetailsTo() *xpv1.PublishConnectionDetailsTo {
	return mg.Spec.PublishConnectionDetailsTo
}

// GetWriteConnectionSecretToReference of this BidPolicy.
func (mg *BidPolicy) GetWriteConnectionSecretToReference() *xpv1.SecretReference {
	return mg.Spec.WriteConnectionSecretToReference
}

// SetConditions of this BidPolicy.
func (mg *BidPolicy) SetConditions(c ...xpv1.Condition) {
	mg.Status.SetConditions(c...)
}

// SetDeletionPolicy of this BidPolicy.
func (mg *BidPolicy) SetDeletionPolicy(r xpv1.DeletionPolicy) {
	mg.Spec.DeletionPolicy = r
}

// SetManagementPolicies of this BidPolicy.
func (mg *BidPolicy) SetManagementPolicies(r xpv1.ManagementPolicies) {
	mg.Spec.ManagementPolicies = r
}

// SetProviderConfigReference of this BidPolicy.
func (mg *BidPolicy) SetProviderConfigReference(r *xpv1.Reference) {
	mg.Spec.ProviderConfigReference = r
}

// SetPublishConnectionDetailsTo of this BidPolicy.
func (mg *BidPolicy) SetPublishConnectionDetailsTo(r *xpv1.PublishConnectionDetailsTo) {
	mg.Spec.PublishConnectionDetailsTo = r
}

// SetWriteConnectionSecretToReference of this BidPolicy.
func (mg *BidPolicy) SetWriteConnectionSecretToReference(r *xpv1.SecretReference) {
	mg.Spec.WriteConnectionSecretToReference = r
}