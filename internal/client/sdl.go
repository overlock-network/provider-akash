package client

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/overlock-network/provider-akash/internal/client/types"
)

// ValidateSDL validates an SDL structure according to Akash requirements
func (ak *AkashClient) ValidateSDL(sdl *types.SDL) []string {
	var errors []string

	// Validate version
	if sdl.Version != "2.0" {
		errors = append(errors, "version must be '2.0'")
	}

	// Validate services
	if len(sdl.Services) == 0 {
		errors = append(errors, "at least one service must be defined")
	}

	for serviceName, service := range sdl.Services {
		errors = append(errors, ak.validateService(serviceName, service)...)
	}

	// Validate profiles
	errors = append(errors, ak.validateProfiles(sdl.Profiles)...)

	// Validate deployment
	if len(sdl.Deployment) == 0 {
		errors = append(errors, "at least one deployment group must be defined")
	}

	errors = append(errors, ak.validateDeployment(sdl.Deployment, sdl.Profiles, sdl.Services)...)

	return errors
}

// GenerateSDLHash creates a deterministic hash of the SDL
func (ak *AkashClient) GenerateSDLHash(sdl *types.SDL) (string, error) {
	data, err := json.Marshal(sdl)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash), nil
}

func (ak *AkashClient) validateService(name string, service types.SDLService) []string {
	var errors []string

	// Validate service name
	if !isValidServiceName(name) {
		errors = append(errors, fmt.Sprintf("service name '%s' is invalid (must match ^[a-z]([-a-z0-9]*[a-z0-9])?$)", name))
	}

	// Validate image
	if service.Image == "" {
		errors = append(errors, fmt.Sprintf("service '%s': image is required", name))
	} else if strings.HasSuffix(service.Image, ":latest") {
		errors = append(errors, fmt.Sprintf("service '%s': avoid using ':latest' tag for image", name))
	}

	// Validate expose specs
	for i, expose := range service.Expose {
		errors = append(errors, ak.validateExposeSpec(name, i, expose)...)
	}

	return errors
}

func (ak *AkashClient) validateExposeSpec(serviceName string, index int, expose types.SDLExposeSpec) []string {
	var errors []string

	// Validate port range
	if expose.Port < 1 || expose.Port > 65535 {
		errors = append(errors, fmt.Sprintf("service '%s' expose[%d]: port %d is out of valid range (1-65535)", serviceName, index, expose.Port))
	}

	// Validate external port if specified
	if expose.As != 0 && (expose.As < 1 || expose.As > 65535) {
		errors = append(errors, fmt.Sprintf("service '%s' expose[%d]: external port %d is out of valid range (1-65535)", serviceName, index, expose.As))
	}

	// Validate protocol
	if expose.Proto != "" && expose.Proto != "tcp" && expose.Proto != "udp" {
		errors = append(errors, fmt.Sprintf("service '%s' expose[%d]: protocol must be 'tcp' or 'udp', got '%s'", serviceName, index, expose.Proto))
	}

	return errors
}

func (ak *AkashClient) validateProfiles(profiles types.SDLProfiles) []string {
	var errors []string

	// Validate compute profiles
	if len(profiles.Compute) == 0 {
		errors = append(errors, "at least one compute profile must be defined")
	}

	for profileName, compute := range profiles.Compute {
		errors = append(errors, ak.validateComputeProfile(profileName, compute)...)
	}

	// Validate placement profiles
	if len(profiles.Placement) == 0 {
		errors = append(errors, "at least one placement profile must be defined")
	}

	for profileName, placement := range profiles.Placement {
		errors = append(errors, ak.validatePlacementProfile(profileName, placement)...)
	}

	return errors
}

func (ak *AkashClient) validateComputeProfile(name string, compute types.SDLComputeProfile) []string {
	var errors []string

	// Validate CPU
	if compute.Resources.CPU.Units == "" {
		errors = append(errors, fmt.Sprintf("compute profile '%s': CPU units are required", name))
	} else if !isValidCPUUnits(compute.Resources.CPU.Units) {
		errors = append(errors, fmt.Sprintf("compute profile '%s': invalid CPU units '%s'", name, compute.Resources.CPU.Units))
	}

	// Validate Memory
	if compute.Resources.Memory.Size == "" {
		errors = append(errors, fmt.Sprintf("compute profile '%s': memory size is required", name))
	} else if !isValidMemorySize(compute.Resources.Memory.Size) {
		errors = append(errors, fmt.Sprintf("compute profile '%s': invalid memory size '%s'", name, compute.Resources.Memory.Size))
	}

	// Validate Storage
	if compute.Resources.Storage.Size == "" {
		errors = append(errors, fmt.Sprintf("compute profile '%s': storage size is required", name))
	} else if !isValidStorageSize(compute.Resources.Storage.Size) {
		errors = append(errors, fmt.Sprintf("compute profile '%s': invalid storage size '%s'", name, compute.Resources.Storage.Size))
	}

	return errors
}

func (ak *AkashClient) validatePlacementProfile(name string, placement types.SDLPlacementProfile) []string {
	var errors []string

	// Validate pricing
	if len(placement.Pricing) == 0 {
		errors = append(errors, fmt.Sprintf("placement profile '%s': at least one pricing entry is required", name))
	}

	for serviceName, pricing := range placement.Pricing {
		if pricing.Amount <= 0 {
			errors = append(errors, fmt.Sprintf("placement profile '%s' service '%s': amount must be positive", name, serviceName))
		}
	}

	return errors
}

func (ak *AkashClient) validateDeployment(deployment map[string]types.SDLDeploymentGroup, profiles types.SDLProfiles, services map[string]types.SDLService) []string {
	var errors []string

	for serviceName, group := range deployment {
		// Check if service exists
		if _, exists := services[serviceName]; !exists {
			errors = append(errors, fmt.Sprintf("deployment group '%s': references non-existent service", serviceName))
		}

		// Check if placement profile exists
		if _, exists := profiles.Placement[group.Profile]; !exists {
			errors = append(errors, fmt.Sprintf("deployment group '%s': references non-existent placement profile '%s'", serviceName, group.Profile))
		}

		// Validate count
		if group.Count <= 0 {
			errors = append(errors, fmt.Sprintf("deployment group '%s': count must be positive", serviceName))
		} else if group.Count > 100 {
			errors = append(errors, fmt.Sprintf("deployment group '%s': count cannot exceed 100", serviceName))
		}
	}

	return errors
}

// Helper validation functions

func isValidServiceName(name string) bool {
	// Service name must match Kubernetes DNS-1123 label requirements
	matched, _ := regexp.MatchString(`^[a-z]([-a-z0-9]*[a-z0-9])?$`, name)
	return matched && len(name) <= 63
}

func isValidCPUUnits(units string) bool {
	// CPU can be fractional (e.g., "0.5") or with milli suffix (e.g., "100m")
	if matched, _ := regexp.MatchString(`^[0-9]*\.?[0-9]+m?$`, units); matched {
		return true
	}
	// Also allow integer values
	if _, err := strconv.Atoi(units); err == nil {
		return true
	}
	return false
}

func isValidMemorySize(size string) bool {
	// Memory size must be in format like "512Mi", "2Gi", etc.
	matched, _ := regexp.MatchString(`^[0-9]+[KMGT]i?$`, size)
	return matched
}

func isValidStorageSize(size string) bool {
	// Storage size must be in format like "1Gi", "10Gi", etc.
	matched, _ := regexp.MatchString(`^[0-9]+[KMGT]i?$`, size)
	return matched
}

