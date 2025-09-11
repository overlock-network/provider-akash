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

// BidParameters are the configurable fields of a Bid.
type BidParameters struct {
	// DeploymentRef is a reference to the Deployment CR this bid is for
	// +kubebuilder:validation:Required
	DeploymentRef DeploymentReference `json:"deploymentRef"`

	// AutoAccept automatically accepts the lowest bid if true
	// +kubebuilder:default=false
	AutoAccept *bool `json:"autoAccept,omitempty"`

	// MaxPrice is the maximum acceptable price filter in uakt
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000000000000
	MaxPrice *int64 `json:"maxPrice,omitempty"`
}

// BidPriceStatus represents price information for a bid
type BidPriceStatus struct {
	// Amount is the bid price amount
	Amount string `json:"amount,omitempty"`

	// Denom is the currency denomination (typically "uakt")
	Denom string `json:"denom,omitempty"`
}

// BidObservation are the observable fields of a Bid.
type BidObservation struct {
	// BidId is the unique identifier for the bid
	BidId string `json:"bidId,omitempty"`

	// Dseq is the deployment sequence number resolved from deploymentRef
	Dseq string `json:"dseq,omitempty"`

	// Gseq is the group sequence number from the bid
	Gseq string `json:"gseq,omitempty"`

	// Oseq is the order sequence number from the bid
	Oseq string `json:"oseq,omitempty"`

	// Owner is the deployment owner resolved from deploymentRef
	Owner string `json:"owner,omitempty"`

	// Provider is the provider address who submitted the bid
	Provider string `json:"provider,omitempty"`

	// Price contains the bid price information
	Price *BidPriceStatus `json:"price,omitempty"`

	// State is the current bid state (open, matched, lost, closed)
	State string `json:"state,omitempty"`

	// CreatedAt is when the bid was received (block height)
	CreatedAt int64 `json:"createdAt,omitempty"`
}

// A BidSpec defines the desired state of a Bid.
type BidSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       BidParameters `json:"forProvider"`
}

// A BidStatus represents the observed state of a Bid.
type BidStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          BidObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Bid represents an Akash Network bid resource for deployment auctions.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="DEPLOYMENT",type="string",JSONPath=".spec.forProvider.deploymentRef.name"
// +kubebuilder:printcolumn:name="PROVIDER",type="string",JSONPath=".status.atProvider.provider"
// +kubebuilder:printcolumn:name="PRICE",type="string",JSONPath=".status.atProvider.price.amount"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.atProvider.state"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,akash}
type Bid struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BidSpec   `json:"spec"`
	Status BidStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BidList contains a list of Bid
type BidList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Bid `json:"items"`
}

// Bid type metadata.
var (
	BidKind             = reflect.TypeOf(Bid{}).Name()
	BidGroupKind        = schema.GroupKind{Group: Group, Kind: BidKind}.String()
	BidKindAPIVersion   = BidKind + "." + SchemeGroupVersion.String()
	BidGroupVersionKind = SchemeGroupVersion.WithKind(BidKind)
)

func init() {
	SchemeBuilder.Register(&Bid{}, &BidList{})
}