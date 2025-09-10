package types

// Seqs represents the sequence numbers returned from deployment operations
type Seqs struct {
	Dseq string
	Gseq string
	Oseq string
}

// SDL structure definitions for parsing Akash Stack Definition Language

// SDL represents the root structure of an Akash deployment manifest
type SDL struct {
	Version    string                        `yaml:"version"`
	Services   map[string]SDLService         `yaml:"services"`
	Profiles   SDLProfiles                   `yaml:"profiles"`
	Deployment map[string]SDLDeploymentGroup `yaml:"deployment"`
}

// SDLService defines a service within the SDL
type SDLService struct {
	Image   string          `yaml:"image"`
	Command []string        `yaml:"command,omitempty"`
	Args    []string        `yaml:"args,omitempty"`
	Env     []string        `yaml:"env,omitempty"`
	Expose  []SDLExposeSpec `yaml:"expose,omitempty"`
}

// SDLExposeSpec defines how a service port should be exposed
type SDLExposeSpec struct {
	Port   int           `yaml:"port"`
	As     int           `yaml:"as,omitempty"`
	Accept []string      `yaml:"accept,omitempty"`
	To     []SDLExposeTo `yaml:"to,omitempty"`
	Proto  string        `yaml:"proto,omitempty"`
}

// SDLExposeTo defines the exposure scope for a port
type SDLExposeTo struct {
	Global bool `yaml:"global,omitempty"`
}

// SDLProfiles contains compute and placement profiles
type SDLProfiles struct {
	Compute   map[string]SDLComputeProfile   `yaml:"compute"`
	Placement map[string]SDLPlacementProfile `yaml:"placement"`
}

// SDLComputeProfile defines the computational resources for a profile
type SDLComputeProfile struct {
	Resources SDLResources `yaml:"resources"`
}

// SDLResources defines the resource requirements
type SDLResources struct {
	CPU     SDLResourceCPU     `yaml:"cpu"`
	Memory  SDLResourceMemory  `yaml:"memory"`
	Storage SDLResourceStorage `yaml:"storage"`
}

// SDLResourceCPU defines CPU resource requirements
type SDLResourceCPU struct {
	Units string `yaml:"units"`
}

// SDLResourceMemory defines memory resource requirements
type SDLResourceMemory struct {
	Size string `yaml:"size"`
}

// SDLResourceStorage defines storage resource requirements
type SDLResourceStorage struct {
	Size string `yaml:"size"`
}

// SDLPlacementProfile defines placement constraints and pricing
type SDLPlacementProfile struct {
	Attributes map[string]interface{} `yaml:"attributes,omitempty"`
	SignedBy   SDLSignedBy            `yaml:"signedBy,omitempty"`
	Pricing    map[string]SDLPricing  `yaml:"pricing"`
}

// SDLSignedBy defines signing requirements for placement
type SDLSignedBy struct {
	AnyOf []string `yaml:"anyOf,omitempty"`
	AllOf []string `yaml:"allOf,omitempty"`
}

// SDLPricing defines pricing information for a service
type SDLPricing struct {
	Denom  string `yaml:"denom"`
	Amount int64  `yaml:"amount"`
}

// SDLDeploymentGroup defines the deployment configuration for a service
type SDLDeploymentGroup struct {
	Profile string `yaml:"profile"`
	Count   int    `yaml:"count"`
}
