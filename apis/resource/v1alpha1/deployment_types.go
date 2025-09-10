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

// DeploymentParameters are the configurable fields of a Deployment.
type DeploymentParameters struct {
	// SDL contains the Akash Stack Definition Language (SDL) deployment manifest as YAML string
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=10
	// +kubebuilder:validation:MaxLength=1048576
	SDL string `json:"sdl"`

	// Deposit is the deployment deposit amount in uakt (minimum 500000, default 5000000)
	// +kubebuilder:validation:Minimum=500000
	// +kubebuilder:validation:Maximum=1000000000000
	// +kubebuilder:default=5000000
	Deposit *int64 `json:"deposit,omitempty"`

	// Currency is the token denomination (default "uakt")
	// +kubebuilder:validation:Enum=uakt;akt
	// +kubebuilder:default="uakt"
	Currency *string `json:"currency,omitempty"`
}


// DeploymentObservation are the observable fields of a Deployment.
type DeploymentObservation struct {
	// DeploymentId is the deployment sequence number assigned by Akash
	DeploymentId string `json:"deploymentId,omitempty"`

	// Owner is the deployment owner address from ProviderConfig
	Owner string `json:"owner,omitempty"`

	// State is the current deployment state from Akash network
	State string `json:"state,omitempty"`

	// CreatedHeight is the block height when deployment was created
	CreatedHeight int64 `json:"createdHeight,omitempty"`

	// EscrowBalance is the current escrow account balance
	EscrowBalance *BalanceStatus `json:"escrowBalance,omitempty"`

	// Version is the deployment version
	Version string `json:"version,omitempty"`
}


// BalanceStatus represents balance information
type BalanceStatus struct {
	Denom  string `json:"denom,omitempty"`
	Amount string `json:"amount,omitempty"`
}


// A DeploymentSpec defines the desired state of a Deployment.
type DeploymentSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       DeploymentParameters `json:"forProvider"`
}

// A DeploymentStatus represents the observed state of a Deployment.
type DeploymentStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          DeploymentObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Deployment represents an Akash Network deployment resource.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="DSEQ",type="string",JSONPath=".status.atProvider.deploymentId"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.atProvider.state"
// +kubebuilder:printcolumn:name="OWNER",type="string",JSONPath=".status.atProvider.owner"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,akash}
type Deployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeploymentSpec   `json:"spec"`
	Status DeploymentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DeploymentList contains a list of Deployment
type DeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Deployment `json:"items"`
}

// Deployment type metadata.
var (
	DeploymentKind             = reflect.TypeOf(Deployment{}).Name()
	DeploymentGroupKind        = schema.GroupKind{Group: Group, Kind: DeploymentKind}.String()
	DeploymentKindAPIVersion   = DeploymentKind + "." + SchemeGroupVersion.String()
	DeploymentGroupVersionKind = SchemeGroupVersion.WithKind(DeploymentKind)
)

func init() {
	SchemeBuilder.Register(&Deployment{}, &DeploymentList{})
}
