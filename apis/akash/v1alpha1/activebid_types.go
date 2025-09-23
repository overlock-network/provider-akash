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

// DeploymentReference represents a reference to a Deployment resource
type DeploymentReference struct {
	// Name of the Deployment resource
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the Deployment resource (optional, defaults to same namespace)
	Namespace *string `json:"namespace,omitempty"`
}

// ActiveBidParameters are the configurable fields of an ActiveBid.
type ActiveBidParameters struct {
	// DeploymentRef is a reference to the Deployment CR this ActiveBid observes
	// +kubebuilder:validation:Required
	DeploymentRef DeploymentReference `json:"deploymentRef"`

	// BidId is the unique identifier for the bid to observe (set by BidPolicy)
	// Format: owner-dseq-gseq-oseq-provider
	// +kubebuilder:validation:Required
	BidId string `json:"bidId"`
}

// ActiveBidPriceStatus represents price information for a bid
type ActiveBidPriceStatus struct {
	// Amount is the bid price amount
	Amount string `json:"amount,omitempty"`

	// Denom is the currency denomination (typically "uakt")
	Denom string `json:"denom,omitempty"`
}

// ActiveBidObservation are the observable fields of an ActiveBid.
type ActiveBidObservation struct {
	// BidId is the unique identifier for the bid (empty when pending)
	BidId string `json:"bidId,omitempty"`

	// Dseq is the deployment sequence number resolved from deploymentRef
	Dseq string `json:"dseq,omitempty"`

	// Gseq is the group sequence number from the bid
	Gseq string `json:"gseq,omitempty"`

	// Oseq is the order sequence number from the bid
	Oseq string `json:"oseq,omitempty"`

	// Owner is the deployment owner resolved from deploymentRef
	Owner string `json:"owner,omitempty"`

	// Provider is the provider address who submitted the bid (empty when pending)
	Provider string `json:"provider,omitempty"`

	// Price contains the bid price information (empty when pending)
	Price *ActiveBidPriceStatus `json:"price,omitempty"`

	// State is the ActiveBid state (pending, received, matched, lost, closed)
	State string `json:"state,omitempty"`

	// CreatedAt is when the ActiveBid was created
	CreatedAt int64 `json:"createdAt,omitempty"`

	// ReceivedAt is when the actual bid was received from provider
	ReceivedAt int64 `json:"receivedAt,omitempty"`
}

// An ActiveBidSpec defines the desired state of an ActiveBid.
type ActiveBidSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       ActiveBidParameters `json:"forProvider"`
}

// An ActiveBidStatus represents the observed state of an ActiveBid.
type ActiveBidStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          ActiveBidObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// An ActiveBid represents an Akash Network ActiveBid for observing and managing provider bids.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="DEPLOYMENT",type="string",JSONPath=".spec.forProvider.deploymentRef.name"
// +kubebuilder:printcolumn:name="PROVIDER",type="string",JSONPath=".status.atProvider.provider"
// +kubebuilder:printcolumn:name="PRICE",type="string",JSONPath=".status.atProvider.price.amount"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.atProvider.state"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,akash}
type ActiveBid struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ActiveBidSpec   `json:"spec"`
	Status ActiveBidStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ActiveBidList contains a list of ActiveBid
type ActiveBidList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ActiveBid `json:"items"`
}

// ActiveBid type metadata.
var (
	ActiveBidKind             = reflect.TypeOf(ActiveBid{}).Name()
	ActiveBidGroupKind        = schema.GroupKind{Group: Group, Kind: ActiveBidKind}.String()
	ActiveBidKindAPIVersion   = ActiveBidKind + "." + SchemeGroupVersion.String()
	ActiveBidGroupVersionKind = SchemeGroupVersion.WithKind(ActiveBidKind)
)

func init() {
	SchemeBuilder.Register(&ActiveBid{}, &ActiveBidList{})
}