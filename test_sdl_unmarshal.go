package main

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"github.com/overlock-network/provider-akash/internal/client/types"
)

func main() {
	// Test YAML that matches the error format
	testYAML := `
version: "2.0"
services:
  web:
    image: nginx
profiles:
  compute:
    web:
      resources:
        cpu: "0.5"
        memory: "512Mi"
        storage: ["1Gi"]
deployment:
  web:
    web:
      profile: web
      count: 1
`

	var sdl types.SDL
	err := yaml.Unmarshal([]byte(testYAML), &sdl)
	if err != nil {
		fmt.Printf("❌ Error unmarshaling SDL: %v\n", err)
		return
	}

	fmt.Printf("✅ SDL unmarshaling successful!\n")
	fmt.Printf("CPU: %s\n", sdl.Profiles.Compute["web"].Resources.CPU.Units)
	fmt.Printf("Memory: %s\n", sdl.Profiles.Compute["web"].Resources.Memory.Size)
	fmt.Printf("Storage: %s\n", sdl.Profiles.Compute["web"].Resources.Storage.Size)
}