package client

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	manifesttypes "pkg.akt.dev/go/manifest/v2beta3"
	providercli "pkg.akt.dev/go/node/provider/v1beta4"

	clienttypes "github.com/overlock-network/provider-akash/internal/client/types"
)

var ErrManifestNotFound = errors.New("manifest not found on provider")

// LeaseInfo contains the lease information needed for manifest operations
type LeaseInfo struct {
	Owner    string
	Dseq     string
	Gseq     string
	Oseq     string
	Provider string
}

// ManifestStatus represents the status of a manifest
type ManifestStatus struct {
	State            string                `json:"state"`
	Version          string                `json:"version"`
	DeployedAt       int64                 `json:"deployedAt"`
	Services         []ManifestServiceInfo `json:"services"`
	ValidationErrors []ManifestError       `json:"validationErrors"`
	ProviderResponse string                `json:"providerResponse"`
}

// ManifestServiceInfo represents information about a service in the manifest
type ManifestServiceInfo struct {
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	Available bool              `json:"available"`
	Endpoints []ServiceEndpoint `json:"endpoints"`
}

// ServiceEndpoint represents an endpoint for a service
type ServiceEndpoint struct {
	URI  string `json:"uri"`
	Port int32  `json:"port"`
	Host string `json:"host"`
}

// ManifestError represents a validation error from the provider
type ManifestError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// SDLDocument represents the structure of an Akash SDL document
type SDLDocument struct {
	Version  string             `yaml:"version"`
	Services map[string]Service `yaml:"services"`
	Profiles *Profiles          `yaml:"profiles,omitempty"`
}

type Service struct {
	Image   string            `yaml:"image"`
	Command []string          `yaml:"command,omitempty"`
	Args    []string          `yaml:"args,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
	Expose  []Expose          `yaml:"expose,omitempty"`
}

type Expose struct {
	Port    int                      `yaml:"port"`
	As      int                      `yaml:"as,omitempty"`
	Proto   string                   `yaml:"proto,omitempty"`
	Service string                   `yaml:"service,omitempty"`
	Global  bool                     `yaml:"global,omitempty"`
	To      []map[string]interface{} `yaml:"to,omitempty"`
	Hosts   []string                 `yaml:"hosts,omitempty"`
}

type Profiles struct {
	Compute   map[string]interface{} `yaml:"compute,omitempty"`
	Placement map[string]interface{} `yaml:"placement,omitempty"`
}

// ProviderHostURI returns the on-chain HostURI for a provider by bech32 address.
func (ak *AkashClient) ProviderHostURI(ctx context.Context, providerAddr string) (string, error) {
	nodeClient, err := ak.getNodeClient()
	if err != nil {
		return "", fmt.Errorf("node client: %w", err)
	}
	resp, err := nodeClient.Query().Provider().Provider(ctx, &providercli.QueryProviderRequest{Owner: providerAddr})
	if err != nil {
		return "", fmt.Errorf("query provider %s: %w", providerAddr, err)
	}
	if resp.Provider.HostURI == "" {
		return "", fmt.Errorf("provider %s registered no HostURI on chain", providerAddr)
	}
	return strings.TrimRight(resp.Provider.HostURI, "/"), nil
}

// SendManifestToProvider PUTs the manifest to the provider over mTLS.
func (ak *AkashClient) SendManifestToProvider(
	ctx context.Context,
	lease LeaseInfo,
	sdl string,
	certPEM, keyPEM []byte,
) (*ManifestStatus, error) {
	hostURI, err := ak.ProviderHostURI(ctx, lease.Provider)
	if err != nil {
		return nil, err
	}

	httpClient, err := newMTLSClient(certPEM, keyPEM, hostURI)
	if err != nil {
		return nil, fmt.Errorf("build mTLS client: %w", err)
	}

	mani, err := convertSDLToManifest(sdl)
	if err != nil {
		return nil, fmt.Errorf("render manifest from SDL: %w", err)
	}
	body, err := json.Marshal(mani)
	if err != nil {
		return nil, fmt.Errorf("marshal manifest JSON: %w", err)
	}

	endpoint := fmt.Sprintf("%s/deployment/%s/manifest", hostURI, lease.Dseq)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build manifest request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("PUT %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	status := &ManifestStatus{
		Version:          ak.calculateManifestVersion(sdl),
		DeployedAt:       time.Now().Unix(),
		Services:         ak.parseServicesFromSDL(sdl),
		ValidationErrors: []ManifestError{},
		ProviderResponse: string(respBody),
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		status.State = "deployed"
		return status, nil
	}
	status.State = "failed"
	status.ValidationErrors = append(status.ValidationErrors, ManifestError{
		Field:   "provider",
		Message: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)),
		Code:    fmt.Sprintf("HTTP_%d", resp.StatusCode),
	})
	return status, fmt.Errorf("provider rejected manifest: HTTP %d: %s", resp.StatusCode, string(respBody))
}

// GetManifestStatus queries the provider's lease status endpoint over mTLS.
func (ak *AkashClient) GetManifestStatus(
	ctx context.Context,
	lease LeaseInfo,
	certPEM, keyPEM []byte,
) (*ManifestStatus, error) {
	hostURI, err := ak.ProviderHostURI(ctx, lease.Provider)
	if err != nil {
		return nil, err
	}
	httpClient, err := newMTLSClient(certPEM, keyPEM, hostURI)
	if err != nil {
		return nil, fmt.Errorf("build mTLS client: %w", err)
	}

	endpoint := fmt.Sprintf("%s/lease/%s/%s/%s/status", hostURI, lease.Dseq, lease.Gseq, lease.Oseq)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusNotFound ||
		resp.StatusCode == http.StatusUnauthorized ||
		resp.StatusCode == http.StatusForbidden {
		return nil, ErrManifestNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("provider returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Services map[string]struct {
			Available int    `json:"available"`
			Total     int    `json:"total"`
			URIs      []string `json:"uris,omitempty"`
		} `json:"services"`
	}
	_ = json.Unmarshal(respBody, &parsed)

	services := make([]ManifestServiceInfo, 0, len(parsed.Services))
	for name, s := range parsed.Services {
		eps := make([]ServiceEndpoint, 0, len(s.URIs))
		for _, u := range s.URIs {
			eps = append(eps, ServiceEndpoint{URI: u})
		}
		services = append(services, ManifestServiceInfo{
			Name:      name,
			Available: s.Available > 0,
			Endpoints: eps,
		})
	}

	return &ManifestStatus{
		State:            "active",
		Services:         services,
		ProviderResponse: string(respBody),
	}, nil
}

// UpdateManifest pushes a new SDL to the provider — same endpoint as create.
func (ak *AkashClient) UpdateManifest(
	ctx context.Context,
	lease LeaseInfo,
	sdl string,
	certPEM, keyPEM []byte,
) (*ManifestStatus, error) {
	return ak.SendManifestToProvider(ctx, lease, sdl, certPEM, keyPEM)
}

// newMTLSClient builds an http.Client for client-cert auth with SNI mtls.<host>.
func newMTLSClient(certPEM, keyPEM []byte, hostURI string) (*http.Client, error) {
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("load mTLS keypair: %w", err)
	}
	host := hostURI
	if u, err := url.Parse(hostURI); err == nil && u.Hostname() != "" {
		host = u.Hostname()
	}
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates:       []tls.Certificate{cert},
				ServerName:         "mtls." + host,
				MinVersion:         tls.VersionTLS13,
				InsecureSkipVerify: true,
				RootCAs:            x509.NewCertPool(),
			},
		},
	}, nil
}

// ValidateManifestSDL parses the SDL as YAML and checks required fields.
func (ak *AkashClient) ValidateManifestSDL(sdlContent string) []ManifestError {
	errors := []ManifestError{}
	var sdl SDLDocument
	if err := yaml.Unmarshal([]byte(sdlContent), &sdl); err != nil {
		errors = append(errors, ManifestError{Field: "sdl", Message: "invalid YAML: " + err.Error(), Code: "INVALID_YAML"})
		return errors
	}
	if sdl.Version == "" {
		errors = append(errors, ManifestError{Field: "version", Message: "version is required", Code: "MISSING_VERSION"})
	}
	if len(sdl.Services) == 0 {
		errors = append(errors, ManifestError{Field: "services", Message: "at least one service required", Code: "MISSING_SERVICES"})
	}
	for name, svc := range sdl.Services {
		if svc.Image == "" {
			errors = append(errors, ManifestError{Field: "services." + name + ".image", Message: "image required", Code: "MISSING_IMAGE"})
		}
		for i, e := range svc.Expose {
			if e.Port <= 0 || e.Port > 65535 {
				errors = append(errors, ManifestError{
					Field:   fmt.Sprintf("services.%s.expose[%d].port", name, i),
					Message: "port must be 1..65535",
					Code:    "INVALID_PORT",
				})
			}
		}
	}
	return errors
}

func (ak *AkashClient) calculateManifestVersion(sdlContent string) string {
	hash := sha256.Sum256([]byte(strings.TrimSpace(sdlContent)))
	return fmt.Sprintf("%x", hash[:])[:12]
}

func (ak *AkashClient) parseServicesFromSDL(sdlContent string) []ManifestServiceInfo {
	var sdl SDLDocument
	if err := yaml.Unmarshal([]byte(sdlContent), &sdl); err != nil {
		return nil
	}
	out := make([]ManifestServiceInfo, 0, len(sdl.Services))
	for name, svc := range sdl.Services {
		info := ManifestServiceInfo{Name: name, Image: svc.Image}
		for _, e := range svc.Expose {
			port := int32(e.Port)
			if e.As > 0 {
				port = int32(e.As)
			}
			info.Endpoints = append(info.Endpoints, ServiceEndpoint{Port: port})
		}
		out = append(out, info)
	}
	return out
}

const (
	defaultMaxBodySize = uint32(1048576)
	defaultReadTimeout = uint32(60000)
	defaultSendTimeout = uint32(60000)
	defaultNextTries   = uint32(3)
)

var defaultNextCases = []string{"error", "timeout"}

// convertSDLToManifest renders a YAML SDL into manifest.Manifest.
func convertSDLToManifest(sdl string) (manifesttypes.Manifest, error) {
	var parsed clienttypes.SDL
	if err := yaml.Unmarshal([]byte(sdl), &parsed); err != nil {
		return nil, fmt.Errorf("parse SDL YAML: %w", err)
	}
	if err := validateSDL(&parsed); err != nil {
		return nil, fmt.Errorf("invalid SDL: %w", err)
	}

	type groupB struct {
		name     string
		services manifesttypes.Services
		nextID   uint32
	}
	groups := make(map[string]*groupB)

	svcNames := make([]string, 0, len(parsed.Deployment))
	for n := range parsed.Deployment {
		svcNames = append(svcNames, n)
	}
	sort.Strings(svcNames)

	for _, svcName := range svcNames {
		deployGroup := parsed.Deployment[svcName]
		svc, ok := parsed.Services[svcName]
		if !ok {
			continue
		}
		compute, ok := parsed.Profiles.Compute[svcName]
		if !ok {
			continue
		}
		placementName := deployGroup.Profile
		if _, ok := parsed.Profiles.Placement[placementName]; !ok {
			return nil, fmt.Errorf("service %s references unknown placement profile %q", svcName, placementName)
		}

		grp := groups[placementName]
		if grp == nil {
			grp = &groupB{name: placementName, nextID: 1}
			groups[placementName] = grp
		}

		resources, err := convertSDLResourcesToAkash(compute.Resources)
		if err != nil {
			return nil, fmt.Errorf("service %s resources: %w", svcName, err)
		}
		endpoints, err := convertSDLExposeToEndpoints(svc.Expose)
		if err != nil {
			return nil, fmt.Errorf("service %s expose: %w", svcName, err)
		}
		if len(endpoints) > 0 {
			resources.Endpoints = endpoints
		}
		resources.ID = grp.nextID
		grp.nextID++

		grp.services = append(grp.services, manifesttypes.Service{
			Name:      svcName,
			Image:     svc.Image,
			Command:   svc.Command,
			Args:      svc.Args,
			Env:       svc.Env,
			Resources: *resources,
			Count:     uint32(deployGroup.Count),
			Expose:    convertSDLExposesToManifest(svc.Expose),
		})
	}

	if len(groups) == 0 {
		return nil, fmt.Errorf("no deployment groups rendered from SDL")
	}

	names := make([]string, 0, len(groups))
	for n := range groups {
		names = append(names, n)
	}
	sort.Strings(names)

	out := make(manifesttypes.Manifest, 0, len(names))
	for _, n := range names {
		g := groups[n]
		sort.Sort(g.services)
		out = append(out, manifesttypes.Group{
			Name:     g.name,
			Services: g.services,
		})
	}
	return out, nil
}

// convertSDLExposesToManifest renders SDL expose specs into manifest ServiceExposes.
func convertSDLExposesToManifest(specs []clienttypes.SDLExposeSpec) manifesttypes.ServiceExposes {
	out := make(manifesttypes.ServiceExposes, 0, len(specs))
	for _, spec := range specs {
		port := uint32(spec.Port)
		external := uint32(spec.As)
		if external == 0 {
			external = port
		}
		proto := manifesttypes.TCP
		if strings.EqualFold(spec.Proto, "udp") {
			proto = manifesttypes.UDP
		}
		global := false
		for _, to := range spec.To {
			if to.Global {
				global = true
				break
			}
		}
		out = append(out, manifesttypes.ServiceExpose{
			Port:         port,
			ExternalPort: external,
			Proto:        proto,
			Global:       global,
			Hosts:        spec.Accept,
			HTTPOptions: manifesttypes.ServiceExposeHTTPOptions{
				MaxBodySize: defaultMaxBodySize,
				ReadTimeout: defaultReadTimeout,
				SendTimeout: defaultSendTimeout,
				NextTries:   defaultNextTries,
				NextCases:   defaultNextCases,
			},
		})
	}
	return out
}
