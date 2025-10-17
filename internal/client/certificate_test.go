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

package client

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCreateCertificate(t *testing.T) {
	tests := []struct {
		name          string
		domains       []string
		owner         string
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid certificate creation",
			domains:     []string{"example.com", "www.example.com"},
			owner:       "akash1test",
			expectError: false,
		},
		{
			name:          "empty domains",
			domains:       []string{},
			owner:         "akash1test",
			expectError:   true,
			errorContains: "at least one domain is required",
		},
		{
			name:          "empty owner",
			domains:       []string{"example.com"},
			owner:         "",
			expectError:   true,
			errorContains: "owner address is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &AkashClient{}
			certInfo, err := client.CreateCertificate(context.Background(), tt.domains, tt.owner)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, err)
				}
				if certInfo != nil {
					t.Error("Expected nil certInfo on error")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				if certInfo == nil {
					t.Error("Expected certInfo but got nil")
					return
				}
				if certInfo.Owner != tt.owner {
					t.Errorf("Expected owner %s, got %s", tt.owner, certInfo.Owner)
				}
				if certInfo.State != CertificateStateValid {
					t.Errorf("Expected state %s, got %s", CertificateStateValid, certInfo.State)
				}
				if certInfo.Serial == "" {
					t.Error("Expected non-empty serial")
				}
				if certInfo.PEM == "" {
					t.Error("Expected non-empty PEM")
				}
			}
		})
	}
}

func TestGetCertificate(t *testing.T) {
	tests := []struct {
		name          string
		serial        string
		owner         string
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid certificate retrieval",
			serial:      "1234567890",
			owner:       "akash1test",
			expectError: false,
		},
		{
			name:          "empty serial",
			serial:        "",
			owner:         "akash1test",
			expectError:   true,
			errorContains: "certificate serial is required",
		},
		{
			name:          "empty owner",
			serial:        "1234567890",
			owner:         "",
			expectError:   true,
			errorContains: "owner address is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &AkashClient{}
			certInfo, err := client.GetCertificate(context.Background(), tt.serial, tt.owner)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, err)
				}
				if certInfo != nil {
					t.Error("Expected nil certInfo on error")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				if certInfo == nil {
					t.Error("Expected certInfo but got nil")
					return
				}
				if certInfo.Serial != tt.serial {
					t.Errorf("Expected serial %s, got %s", tt.serial, certInfo.Serial)
				}
				if certInfo.Owner != tt.owner {
					t.Errorf("Expected owner %s, got %s", tt.owner, certInfo.Owner)
				}
				if certInfo.State != CertificateStateValid {
					t.Errorf("Expected state %s, got %s", CertificateStateValid, certInfo.State)
				}
			}
		})
	}
}

func TestRevokeCertificate(t *testing.T) {
	tests := []struct {
		name          string
		serial        string
		owner         string
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid certificate revocation",
			serial:      "1234567890",
			owner:       "akash1test",
			expectError: false,
		},
		{
			name:          "empty serial",
			serial:        "",
			owner:         "akash1test",
			expectError:   true,
			errorContains: "certificate serial is required",
		},
		{
			name:          "empty owner",
			serial:        "1234567890",
			owner:         "",
			expectError:   true,
			errorContains: "owner address is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &AkashClient{}
			err := client.RevokeCertificate(context.Background(), tt.serial, tt.owner)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestGetCertificates(t *testing.T) {
	tests := []struct {
		name          string
		owner         string
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid certificates listing",
			owner:       "akash1test",
			expectError: false,
		},
		{
			name:          "empty owner",
			owner:         "",
			expectError:   true,
			errorContains: "owner address is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &AkashClient{}
			certs, err := client.GetCertificates(context.Background(), tt.owner)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, err)
				}
				if certs != nil {
					t.Error("Expected nil certs on error")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				if certs == nil {
					t.Error("Expected certs but got nil")
					return
				}
				if len(certs) != 1 {
					t.Errorf("Expected 1 certificate, got %d", len(certs))
					return
				}
				if certs[0].Owner != tt.owner {
					t.Errorf("Expected owner %s, got %s", tt.owner, certs[0].Owner)
				}
			}
		})
	}
}

func TestValidateCertificate(t *testing.T) {
	tests := []struct {
		name          string
		certInfo      *CertificateInfo
		autoRenew     bool
		validityDays  int32
		expectRenewal bool
		expectError   bool
		errorContains string
	}{
		{
			name: "valid certificate - no renewal needed",
			certInfo: &CertificateInfo{
				State:     CertificateStateValid,
				ExpiresAt: time.Now().Add(60 * 24 * time.Hour).Unix(), // 60 days from now
			},
			autoRenew:    true,
			expectRenewal: false,
			expectError:   false,
		},
		{
			name: "valid certificate - renewal needed",
			certInfo: &CertificateInfo{
				State:     CertificateStateValid,
				ExpiresAt: time.Now().Add(15 * 24 * time.Hour).Unix(), // 15 days from now
			},
			autoRenew:    true,
			expectRenewal: true,
			expectError:   false,
		},
		{
			name: "expired certificate",
			certInfo: &CertificateInfo{
				State:     CertificateStateExpired,
				ExpiresAt: time.Now().Add(-24 * time.Hour).Unix(), // 1 day ago
			},
			autoRenew:      true,
			expectError:    true,
			errorContains:  "certificate is expired",
		},
		{
			name: "revoked certificate",
			certInfo: &CertificateInfo{
				State:     CertificateStateRevoked,
				ExpiresAt: time.Now().Add(30 * 24 * time.Hour).Unix(),
			},
			autoRenew:     true,
			expectError:   true,
			errorContains: "certificate is revoked",
		},
		{
			name:          "nil certificate info",
			certInfo:      nil,
			autoRenew:     true,
			expectError:   true,
			errorContains: "certificate info is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &AkashClient{}
			needsRenewal, err := client.ValidateCertificate(tt.certInfo, tt.autoRenew, tt.validityDays)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				if needsRenewal != tt.expectRenewal {
					t.Errorf("Expected renewal %t, got %t", tt.expectRenewal, needsRenewal)
				}
			}
		})
	}
}

func TestGenerateCertificate(t *testing.T) {
	tests := []struct {
		name        string
		domains     []string
		expectError bool
	}{
		{
			name:        "valid domains",
			domains:     []string{"example.com", "www.example.com"},
			expectError: false,
		},
		{
			name:        "single domain",
			domains:     []string{"test.com"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &AkashClient{}
			certPEM, cert, err := client.generateCertificate(tt.domains)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				if certPEM == "" {
					t.Error("Expected non-empty PEM")
				}
				if cert == nil {
					t.Error("Expected cert but got nil")
					return
				}
				if cert.Subject.CommonName != tt.domains[0] {
					t.Errorf("Expected CommonName %s, got %s", tt.domains[0], cert.Subject.CommonName)
				}
				if len(cert.DNSNames) != len(tt.domains) {
					t.Errorf("Expected %d DNS names, got %d", len(tt.domains), len(cert.DNSNames))
				}
				if cert.SerialNumber == nil {
					t.Error("Expected non-nil serial number")
				}
			}
		})
	}
}

func TestCertificateConstants(t *testing.T) {
	expectedStates := map[string]string{
		"valid":   CertificateStateValid,
		"expired": CertificateStateExpired,
		"revoked": CertificateStateRevoked,
	}

	for expected, actual := range expectedStates {
		if actual != expected {
			t.Errorf("Expected certificate state %s, got %s", expected, actual)
		}
	}
}