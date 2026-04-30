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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// SDLParameters are the configurable fields of an SDL.
type SDLParameters struct {
	// Version is the SDL version (currently only "2.0" is supported)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum="2.0"
	Version string `json:"version"`

	// Services is the map of service definitions
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinProperties=1
	Services map[string]SDLService `json:"services"`

	// Profiles contains compute and placement profiles
	// +kubebuilder:validation:Required
	Profiles SDLProfiles `json:"profiles"`

	// Deployment contains deployment group configurations
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinProperties=1
	Deployment map[string]SDLDeploymentGroup `json:"deployment"`
}

// SDLService defines a service within the SDL
type SDLService struct {
	// Image is the Docker image for the container
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// Command is the custom container execution command
	// +optional
	Command []string `json:"command,omitempty"`

	// Args are the arguments for the custom command
	// +optional
	Args []string `json:"args,omitempty"`

	// Env is the list of environment variables
	// +optional
	Env []string `json:"env,omitempty"`

	// Expose defines the ports and services to expose
	// +optional
	Expose []SDLServiceExpose `json:"expose,omitempty"`

	// Params defines service parameters like storage
	// +optional
	Params *SDLServiceParams `json:"params,omitempty"`

	// DependsOn lists services this service depends on (future use)
	// +optional
	DependsOn []string `json:"depends-on,omitempty"`
}

// SDLServiceExpose defines how a service port should be exposed
type SDLServiceExpose struct {
	// Port is the container port to expose
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// As is the external port number
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	As *int32 `json:"as,omitempty"`

	// Proto is the protocol (TCP/UDP)
	// +optional
	// +kubebuilder:validation:Enum=tcp;udp
	// +kubebuilder:default="tcp"
	Proto string `json:"proto,omitempty"`

	// To defines the exposure scope
	// +optional
	To []SDLServiceExposeTo `json:"to,omitempty"`

	// Accept defines ingress specifications
	// +optional
	Accept *SDLServiceExposeAccept `json:"accept,omitempty"`

	// HTTPOptions defines HTTP specific options
	// +optional
	HTTPOptions *SDLServiceExposeHTTPOptions `json:"http_options,omitempty"`
}

// SDLServiceExposeTo defines the exposure scope for a port
type SDLServiceExposeTo struct {
	// Service defines internal service exposure
	// +optional
	Service string `json:"service,omitempty"`

	// Global indicates if the port should be globally accessible
	// +optional
	Global bool `json:"global,omitempty"`
}

// SDLServiceExposeAccept defines accepted ingress
type SDLServiceExposeAccept struct {
	// Items is the list of accepted items
	// +optional
	Items []string `json:"items,omitempty"`
}

// SDLServiceExposeHTTPOptions defines HTTP options
type SDLServiceExposeHTTPOptions struct {
	// MaxBodySize is the maximum body size
	// +optional
	MaxBodySize int32 `json:"max_body_size,omitempty"`

	// ReadTimeout is the read timeout
	// +optional
	ReadTimeout int32 `json:"read_timeout,omitempty"`

	// SendTimeout is the send timeout
	// +optional
	SendTimeout int32 `json:"send_timeout,omitempty"`

	// NextTries is the number of next tries
	// +optional
	NextTries int32 `json:"next_tries,omitempty"`

	// NextTimeout is the next timeout
	// +optional
	NextTimeout int32 `json:"next_timeout,omitempty"`

	// NextCases is the list of next cases
	// +optional
	NextCases []string `json:"next_cases,omitempty"`
}

// SDLServiceParams defines service parameters
type SDLServiceParams struct {
	// Storage defines storage parameters
	// +optional
	Storage map[string]SDLStorageParams `json:"storage,omitempty"`
}

// SDLStorageParams defines storage parameters
type SDLStorageParams struct {
	// Mount is the mount path
	// +kubebuilder:validation:Required
	Mount string `json:"mount"`

	// ReadOnly indicates if the storage is read-only
	// +optional
	ReadOnly bool `json:"readOnly,omitempty"`
}

// SDLProfiles contains compute and placement profiles
type SDLProfiles struct {
	// Compute defines the compute resource profiles
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinProperties=1
	Compute map[string]SDLComputeProfile `json:"compute"`

	// Placement defines the placement profiles
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinProperties=1
	Placement map[string]SDLPlacementProfile `json:"placement"`
}

// SDLComputeProfile defines the computational resources for a profile
type SDLComputeProfile struct {
	// Resources defines the resource requirements
	// +kubebuilder:validation:Required
	Resources SDLResourceUnits `json:"resources"`
}

// SDLResourceUnits defines the resource requirements
type SDLResourceUnits struct {
	// CPU defines CPU units (can be fractional, e.g., "0.5", "100m")
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([0-9]+m?|[0-9]*\.[0-9]+)$`
	CPU string `json:"cpu"`

	// Memory defines memory size (e.g., "512Mi", "2Gi")
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[0-9]+[KMGT]i?$`
	Memory string `json:"memory"`

	// Storage defines ephemeral storage size (e.g., "1Gi", "10Gi")
	// +optional
	Storage []SDLStorage `json:"storage,omitempty"`

	// GPU defines GPU requirements
	// +optional
	GPU *SDLGPUUnits `json:"gpu,omitempty"`
}

// SDLStorage defines storage requirements
type SDLStorage struct {
	// Name is the storage name
	// +optional
	Name string `json:"name,omitempty"`

	// Size is the storage size (e.g., "1Gi", "10Gi")
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[0-9]+[KMGT]i?$`
	Size string `json:"size"`

	// Class is the storage class
	// +optional
	// +kubebuilder:validation:Enum=default;beta1;beta2;beta3
	Class string `json:"class,omitempty"`
}

// SDLGPUUnits defines GPU requirements
type SDLGPUUnits struct {
	// Units is the number of GPU units
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	Units int32 `json:"units"`

	// Attributes defines GPU attributes
	// +optional
	Attributes map[string]runtime.RawExtension `json:"attributes,omitempty"`
}

// SDLPlacementProfile defines placement constraints and pricing
type SDLPlacementProfile struct {
	// Attributes defines provider attributes
	// +optional
	Attributes map[string]string `json:"attributes,omitempty"`

	// SignedBy defines signing requirements for placement
	// +optional
	SignedBy *SDLSignedBy `json:"signedBy,omitempty"`

	// Pricing defines pricing information for services
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinProperties=1
	Pricing map[string]SDLPricing `json:"pricing"`
}

// SDLSignedBy defines signing requirements for placement
type SDLSignedBy struct {
	// AnyOf requires signature from any of the listed addresses
	// +optional
	AnyOf []string `json:"anyOf,omitempty"`

	// AllOf requires signatures from all listed addresses
	// +optional
	AllOf []string `json:"allOf,omitempty"`
}

// SDLPricing defines pricing information for a service.
// Node v2 BME hard-codes the price denom to uact, so it is not user-configurable.
type SDLPricing struct {
	// Amount is the price amount in uact
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	Amount int64 `json:"amount"`
}

// SDLDeploymentGroup defines the deployment configuration for a service
type SDLDeploymentGroup struct {
	// Profile references a placement profile name
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Profile string `json:"profile"`

	// Count is the number of instances to deploy
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	Count int32 `json:"count"`
}

// SDLObservation are the observable fields of an SDL.
type SDLObservation struct {
	// Hash is the content hash of the SDL for change detection
	Hash string `json:"hash,omitempty"`

	// Validated indicates if the SDL has been validated
	Validated bool `json:"validated,omitempty"`

	// ValidationErrors contains any validation error messages
	ValidationErrors []string `json:"validationErrors,omitempty"`

	// LastValidated is the timestamp of the last validation
	LastValidated *metav1.Time `json:"lastValidated,omitempty"`
}

// An SDLSpec defines the desired state of an SDL.
type SDLSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       SDLParameters `json:"forProvider"`
}

// An SDLStatus represents the observed state of an SDL.
type SDLStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          SDLObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// An SDL represents an Akash Stack Definition Language resource.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="VALIDATED",type="boolean",JSONPath=".status.atProvider.validated"
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.forProvider.version"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,akash}
type SDL struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SDLSpec   `json:"spec"`
	Status SDLStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SDLList contains a list of SDL
type SDLList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SDL `json:"items"`
}

// SDL type metadata.
var (
	SDLKind             = reflect.TypeOf(SDL{}).Name()
	SDLGroupKind        = schema.GroupKind{Group: Group, Kind: SDLKind}.String()
	SDLKindAPIVersion   = SDLKind + "." + SchemeGroupVersion.String()
	SDLGroupVersionKind = SchemeGroupVersion.WithKind(SDLKind)
)

func init() {
	SchemeBuilder.Register(&SDL{}, &SDLList{})
}