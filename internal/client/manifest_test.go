package client

import (
	"testing"
	"time"
)

func TestValidateManifestSDL(t *testing.T) {
	client := &AkashClient{}

	tests := []struct {
		name          string
		sdlContent    string
		expectedErrors int
	}{
		{
			name: "valid SDL",
			sdlContent: `---
version: "2.0"
services:
  web:
    image: nginx:latest
    expose:
      - port: 80
        as: 80
        global: true`,
			expectedErrors: 0,
		},
		{
			name:           "missing version",
			sdlContent:     "services:\n  web:\n    image: nginx:latest",
			expectedErrors: 1,
		},
		{
			name:           "missing services",
			sdlContent:     `version: "2.0"`,
			expectedErrors: 1, // missing services
		},
		{
			name:           "empty SDL",
			sdlContent:     "",
			expectedErrors: 2, // missing version, services
		},
		{
			name: "invalid port number",
			sdlContent: `---
version: "2.0"
services:
  web:
    image: nginx:latest
    expose:
      - port: 99999`,
			expectedErrors: 1, // invalid port
		},
		{
			name: "malformed YAML",
			sdlContent: `version: "2.0"
services:
  web:
    image: nginx:latest
    [invalid yaml`,
			expectedErrors: 1, // YAML error only
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := client.ValidateManifestSDL(tt.sdlContent)
			if len(errors) != tt.expectedErrors {
				t.Errorf("ValidateManifestSDL() got %d errors, want %d", len(errors), tt.expectedErrors)
				for _, err := range errors {
					t.Logf("Error: %s - %s", err.Field, err.Message)
				}
			}
		})
	}
}

func TestCalculateManifestVersion(t *testing.T) {
	client := &AkashClient{}

	sdl1 := `version: "2.0"
services:
  web:
    image: nginx:latest`

	sdl2 := `version: "2.0"
services:
  web:
    image: nginx:1.20`

	version1 := client.calculateManifestVersion(sdl1)
	version2 := client.calculateManifestVersion(sdl2)

	// Versions should be different for different content
	if version1 == version2 {
		t.Error("Expected different versions for different SDL content")
	}

	// Version should be consistent for same content
	version1Again := client.calculateManifestVersion(sdl1)
	if version1 != version1Again {
		t.Error("Expected same version for same SDL content")
	}

	// Version should be 12 characters (first 12 chars of SHA256)
	if len(version1) != 12 {
		t.Errorf("Expected version length 12, got %d", len(version1))
	}
}

func TestParseServicesFromSDL(t *testing.T) {
	client := &AkashClient{}

	sdl := `---
version: "2.0"
services:
  web:
    image: nginx:latest
    expose:
      - port: 80
        as: 80
        global: true
  api:
    image: myapp:latest
    expose:
      - port: 8080`

	services := client.parseServicesFromSDL(sdl)

	// Should find 2 services
	expectedServices := 2
	if len(services) != expectedServices {
		t.Errorf("Expected %d services, got %d", expectedServices, len(services))
	}

	// Check if web service is found with correct image
	webFound := false
	apiFound := false
	for _, svc := range services {
		if svc.Name == "web" && svc.Image == "nginx:latest" {
			webFound = true
			// Check if endpoints are parsed correctly
			if len(svc.Endpoints) != 1 {
				t.Errorf("Expected 1 endpoint for web service, got %d", len(svc.Endpoints))
			} else if svc.Endpoints[0].Port != 80 {
				t.Errorf("Expected port 80 for web service, got %d", svc.Endpoints[0].Port)
			}
		}
		if svc.Name == "api" && svc.Image == "myapp:latest" {
			apiFound = true
			// Check if endpoints are parsed correctly
			if len(svc.Endpoints) != 1 {
				t.Errorf("Expected 1 endpoint for api service, got %d", len(svc.Endpoints))
			} else if svc.Endpoints[0].Port != 8080 {
				t.Errorf("Expected port 8080 for api service, got %d", svc.Endpoints[0].Port)
			}
		}
	}
	if !webFound {
		t.Error("Expected to find web service with nginx:latest image")
	}
	if !apiFound {
		t.Error("Expected to find api service with myapp:latest image")
	}
}

func TestLeaseInfo(t *testing.T) {
	leaseInfo := LeaseInfo{
		Owner:    "akash1owner123",
		Dseq:     "12345",
		Gseq:     "1",
		Oseq:     "1",
		Provider: "akash1provider456",
	}

	// Test that all fields are properly set
	if leaseInfo.Owner == "" {
		t.Error("Owner should not be empty")
	}
	if leaseInfo.Dseq == "" {
		t.Error("Dseq should not be empty")
	}
	if leaseInfo.Provider == "" {
		t.Error("Provider should not be empty")
	}
}

func TestManifestStatus(t *testing.T) {
	status := &ManifestStatus{
		State:      "deployed",
		Version:    "abc123def456",
		DeployedAt: time.Now().Unix(),
		Services: []ManifestServiceInfo{
			{
				Name:      "web",
				Image:     "nginx:latest",
				Available: true,
			},
		},
		ValidationErrors: []ManifestError{},
	}

	// Test status fields
	if status.State != "deployed" {
		t.Errorf("Expected state 'deployed', got '%s'", status.State)
	}

	if len(status.Services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(status.Services))
	}

	if status.Services[0].Name != "web" {
		t.Errorf("Expected service name 'web', got '%s'", status.Services[0].Name)
	}

	if len(status.ValidationErrors) != 0 {
		t.Error("Expected no validation errors")
	}
}