package client

import (
	"fmt"

	"gopkg.in/yaml.v3"

	resourcev1alpha1 "github.com/overlock-network/provider-akash/apis/resource/v1alpha1"
)

// RenderSDLToYAML produces the canonical YAML SDL string from a SDL CR.
func RenderSDLToYAML(sdl *resourcev1alpha1.SDL) (string, error) {
	doc := map[string]interface{}{
		"version":    sdl.Spec.ForProvider.Version,
		"services":   renderSDLServices(sdl.Spec.ForProvider.Services),
		"profiles":   renderSDLProfiles(sdl.Spec.ForProvider.Profiles),
		"deployment": sdl.Spec.ForProvider.Deployment,
	}
	out, err := yaml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshal SDL doc to YAML: %w", err)
	}
	return string(out), nil
}

func renderSDLServices(services map[string]resourcev1alpha1.SDLService) map[string]interface{} {
	result := make(map[string]interface{})
	for name, service := range services {
		svc := map[string]interface{}{"image": service.Image}
		if len(service.Command) > 0 {
			svc["command"] = service.Command
		}
		if len(service.Args) > 0 {
			svc["args"] = service.Args
		}
		if len(service.Env) > 0 {
			svc["env"] = service.Env
		}
		if len(service.Expose) > 0 {
			exposes := make([]map[string]interface{}, len(service.Expose))
			for i, e := range service.Expose {
				m := map[string]interface{}{"port": e.Port}
				if e.As != nil {
					m["as"] = *e.As
				}
				if e.Proto != "" {
					m["proto"] = e.Proto
				}
				if len(e.To) > 0 {
					to := make([]map[string]interface{}, len(e.To))
					for j, t := range e.To {
						tm := make(map[string]interface{})
						if t.Service != "" {
							tm["service"] = t.Service
						}
						if t.Global {
							tm["global"] = t.Global
						}
						to[j] = tm
					}
					m["to"] = to
				}
				if e.Accept != nil && len(e.Accept.Items) > 0 {
					m["accept"] = e.Accept.Items
				}
				exposes[i] = m
			}
			svc["expose"] = exposes
		}
		result[name] = svc
	}
	return result
}

func renderSDLProfiles(profiles resourcev1alpha1.SDLProfiles) map[string]interface{} {
	return map[string]interface{}{
		"compute":   renderSDLComputeProfiles(profiles.Compute),
		"placement": renderSDLPlacementProfiles(profiles.Placement),
	}
}

func renderSDLComputeProfiles(compute map[string]resourcev1alpha1.SDLComputeProfile) map[string]interface{} {
	result := make(map[string]interface{})
	for name, profile := range compute {
		resources := map[string]interface{}{
			"cpu":    profile.Resources.CPU,
			"memory": profile.Resources.Memory,
		}
		if len(profile.Resources.Storage) > 0 {
			storage := make([]map[string]interface{}, len(profile.Resources.Storage))
			for i, s := range profile.Resources.Storage {
				m := map[string]interface{}{"size": s.Size}
				if s.Name != "" {
					m["name"] = s.Name
				}
				if s.Class != "" {
					m["class"] = s.Class
				}
				storage[i] = m
			}
			resources["storage"] = storage
		}
		if profile.Resources.GPU != nil {
			gpu := map[string]interface{}{"units": profile.Resources.GPU.Units}
			if len(profile.Resources.GPU.Attributes) > 0 {
				gpu["attributes"] = profile.Resources.GPU.Attributes
			}
			resources["gpu"] = gpu
		}
		result[name] = map[string]interface{}{"resources": resources}
	}
	return result
}

func renderSDLPlacementProfiles(placement map[string]resourcev1alpha1.SDLPlacementProfile) map[string]interface{} {
	result := make(map[string]interface{})
	for name, profile := range placement {
		m := make(map[string]interface{})
		if len(profile.Attributes) > 0 {
			m["attributes"] = profile.Attributes
		}
		if profile.SignedBy != nil && (len(profile.SignedBy.AnyOf) > 0 || len(profile.SignedBy.AllOf) > 0) {
			signedBy := make(map[string]interface{})
			if len(profile.SignedBy.AnyOf) > 0 {
				signedBy["anyOf"] = profile.SignedBy.AnyOf
			}
			if len(profile.SignedBy.AllOf) > 0 {
				signedBy["allOf"] = profile.SignedBy.AllOf
			}
			m["signedBy"] = signedBy
		}
		if len(profile.Pricing) > 0 {
			pricing := make(map[string]interface{})
			for svcName, price := range profile.Pricing {
				pricing[svcName] = map[string]interface{}{
					"denom":  resourcev1alpha1.DepositDenom,
					"amount": price.Amount,
				}
			}
			m["pricing"] = pricing
		}
		result[name] = m
	}
	return result
}
