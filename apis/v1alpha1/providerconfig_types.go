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

// A ProviderConfigSpec defines the desired state of a ProviderConfig.
type ProviderConfigSpec struct {
	// Credentials required to authenticate to this provider.
	Credentials ProviderCredentials `json:"credentials"`
	
	// Configuration contains Akash-specific configuration settings.
	// +optional
	Configuration *AkashConfiguration `json:"configuration,omitempty"`
}

// ProviderCredentials required to authenticate.
type ProviderCredentials struct {
	// Source of the provider credentials.
	// +kubebuilder:validation:Enum=None;Secret;InjectedIdentity;Environment;Filesystem
	Source xpv1.CredentialsSource `json:"source"`

	xpv1.CommonCredentialSelectors `json:",inline"`
}

// AkashConfiguration contains Akash-specific configuration settings.
type AkashConfiguration struct {
	// KeyName is the name of the key to use for signing transactions.
	// +optional
	// +kubebuilder:default="default"
	KeyName *string `json:"keyName,omitempty"`
	
	// KeyringBackend specifies the keyring backend to use.
	// +optional
	// +kubebuilder:validation:Enum=os;file;test;memory
	// +kubebuilder:default="test"
	KeyringBackend *string `json:"keyringBackend,omitempty"`
	
	// AccountAddress is the Akash account address to use.
	// +optional
	AccountAddress *string `json:"accountAddress,omitempty"`
	
	// Net specifies the Akash network to connect to.
	// +optional
	// +kubebuilder:validation:Enum=mainnet;testnet;sandbox
	// +kubebuilder:default="mainnet"
	Net *string `json:"net,omitempty"`
	
	// Version specifies the Akash version to use.
	// +optional
	// +kubebuilder:default="0.18.0"
	Version *string `json:"version,omitempty"`
	
	// ChainId is the chain ID of the Akash network.
	// +optional
	// +kubebuilder:default="akashnet-2"
	ChainId *string `json:"chainId,omitempty"`
	
	// Node is the RPC endpoint of the Akash node.
	// +optional
	// +kubebuilder:default="https://rpc.akashnet.io:443"
	Node *string `json:"node,omitempty"`
	
	// Home is the home directory for Akash configuration.
	// +optional
	// +kubebuilder:default="/tmp/.akash"
	Home *string `json:"home,omitempty"`
	
	// Path is the path to the Akash binary.
	// +optional
	// +kubebuilder:default="/usr/local/bin/akash"
	Path *string `json:"path,omitempty"`
	
	// ProvidersApi is the URL of the Akash providers API.
	// +optional
	// +kubebuilder:default="https://akash-api.polkachu.com"
	ProvidersApi *string `json:"providersApi,omitempty"`
}

// A ProviderConfigStatus reflects the observed state of a ProviderConfig.
type ProviderConfigStatus struct {
	xpv1.ProviderConfigStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// A ProviderConfig configures a Akash provider.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="SECRET-NAME",type="string",JSONPath=".spec.credentials.secretRef.name",priority=1
// +kubebuilder:resource:scope=Cluster
type ProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProviderConfigSpec   `json:"spec"`
	Status ProviderConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProviderConfigList contains a list of ProviderConfig.
type ProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderConfig `json:"items"`
}

// ProviderConfig type metadata.
var (
	ProviderConfigKind             = reflect.TypeOf(ProviderConfig{}).Name()
	ProviderConfigGroupKind        = schema.GroupKind{Group: Group, Kind: ProviderConfigKind}.String()
	ProviderConfigKindAPIVersion   = ProviderConfigKind + "." + SchemeGroupVersion.String()
	ProviderConfigGroupVersionKind = SchemeGroupVersion.WithKind(ProviderConfigKind)
)

func init() {
	SchemeBuilder.Register(&ProviderConfig{}, &ProviderConfigList{})
}
