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
	Storage SDLResourceStorage `yaml:"storage,omitempty"`
}

// SDLResourceCPU defines CPU resource requirements
type SDLResourceCPU struct {
	Units string `yaml:"units"`
}

// UnmarshalYAML implements custom YAML unmarshaling for CPU resources
func (c *SDLResourceCPU) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try to unmarshal as a string first (simple format)
	var units string
	if err := unmarshal(&units); err == nil {
		c.Units = units
		return nil
	}
	
	// Fall back to structured format
	var structured struct {
		Units string `yaml:"units"`
	}
	if err := unmarshal(&structured); err != nil {
		return err
	}
	c.Units = structured.Units
	return nil
}

// SDLResourceMemory defines memory resource requirements
type SDLResourceMemory struct {
	Size string `yaml:"size"`
}

// UnmarshalYAML implements custom YAML unmarshaling for memory resources
func (m *SDLResourceMemory) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try to unmarshal as a string first (simple format)
	var size string
	if err := unmarshal(&size); err == nil {
		m.Size = size
		return nil
	}
	
	// Fall back to structured format
	var structured struct {
		Size string `yaml:"size"`
	}
	if err := unmarshal(&structured); err != nil {
		return err
	}
	m.Size = structured.Size
	return nil
}

// SDLResourceStorage defines storage resource requirements
type SDLResourceStorage struct {
	Size  string `yaml:"size"`
	Class string `yaml:"class,omitempty"`
}

// UnmarshalYAML implements custom YAML unmarshaling for storage resources
func (s *SDLResourceStorage) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try to unmarshal as a string first (simple format)
	var size string
	if err := unmarshal(&size); err == nil {
		s.Size = size
		return nil
	}
	
	// Try to unmarshal as a sequence (array format)
	var sequence []interface{}
	if err := unmarshal(&sequence); err == nil {
		if len(sequence) > 0 {
			if sizeStr, ok := sequence[0].(string); ok {
				s.Size = sizeStr
			}
		}
		return nil
	}
	
	// Fall back to structured format
	var structured struct {
		Size  string `yaml:"size"`
		Class string `yaml:"class,omitempty"`
	}
	if err := unmarshal(&structured); err != nil {
		return err
	}
	s.Size = structured.Size
	s.Class = structured.Class
	return nil
}

// SDLStorage defines storage requirements
type SDLStorage struct {
	Name  string `yaml:"name,omitempty"`
	Size  string `yaml:"size"`
	Class string `yaml:"class,omitempty"`
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

// SDLPricing defines pricing information for a service.
// Denom is fixed to uact under node v2 BME and is not part of the SDL spec.
type SDLPricing struct {
	Amount int64 `yaml:"amount"`
}

// SDLDeploymentGroup defines the deployment configuration for a service
type SDLDeploymentGroup struct {
	Profile string `yaml:"profile"`
	Count   int    `yaml:"count"`
}
