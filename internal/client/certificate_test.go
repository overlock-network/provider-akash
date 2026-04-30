/*
Copyright 2024 The Akash Provider Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package client

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/pem"
	"strings"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	certv1 "pkg.akt.dev/go/node/cert/v1"
)

// mockAddress builds an sdk.AccAddress from a bech32 string for tests.
// Falls back to an empty address if parsing fails so the test fails the
// downstream assertion rather than the helper.
func mockAddress(bech32 string) sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(bech32)
	return addr
}

func newTestContext() context.Context { return context.Background() }

// TestBuildAkashCertificate exercises the in-memory certificate generation
// (no chain interaction). The produced PEM blocks must round-trip through
// chain-sdk's ParseAndValidateCertificate, which is what the on-chain
// MsgCreateCertificate handler calls.
func TestBuildAkashCertificate(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	owner := "akash1hppmegxevqjvnz5z76vkrldgr323n8en5wa0jv"
	notBefore := time.Now().UTC()
	notAfter := notBefore.Add(time.Hour)

	certPEM, pubPEM, keyPEM, parsed, err := buildAkashCertificate(priv, owner, []string{"example.com"}, notBefore, notAfter)
	if err != nil {
		t.Fatalf("buildAkashCertificate: %v", err)
	}

	if parsed.Subject.CommonName != owner {
		t.Errorf("Subject.CN = %q, want %q", parsed.Subject.CommonName, owner)
	}
	if parsed.Issuer.CommonName != owner {
		t.Errorf("Issuer.CN = %q, want %q (self-signed)", parsed.Issuer.CommonName, owner)
	}

	for name, data := range map[string][]byte{
		"cert":    certPEM,
		"pubkey":  pubPEM,
		"privkey": keyPEM,
	} {
		blk, _ := pem.Decode(data)
		if blk == nil {
			t.Fatalf("%s PEM did not decode", name)
		}
	}

	// Final gate: the chain-sdk validator must accept what we produced —
	// otherwise MsgCreateCertificate will fail at the keeper.
	if _, err := certv1.ParseAndValidateCertificate(mockAddress(owner), certPEM, pubPEM); err != nil {
		t.Fatalf("chain-sdk validator rejected our certificate: %v", err)
	}
}

func TestValidateCertificate(t *testing.T) {
	ak := &AkashClient{}

	cases := []struct {
		name      string
		info      *CertificateInfo
		autoRenew bool
		want      bool
		wantErr   bool
	}{
		{name: "nil info", info: nil, wantErr: true},
		{name: "revoked is terminal", info: &CertificateInfo{State: CertificateStateRevoked}, wantErr: true},
		{
			name:      "auto-renew off short-circuits",
			info:      &CertificateInfo{State: CertificateStateValid, ExpiresAt: time.Now().Add(time.Hour).Unix()},
			autoRenew: false,
			want:      false,
		},
		{
			name:      "renew when within 30d window",
			info:      &CertificateInfo{State: CertificateStateValid, ExpiresAt: time.Now().Add(24 * time.Hour).Unix()},
			autoRenew: true,
			want:      true,
		},
		{
			name:      "no renew when far from expiry",
			info:      &CertificateInfo{State: CertificateStateValid, ExpiresAt: time.Now().Add(60 * 24 * time.Hour).Unix()},
			autoRenew: true,
			want:      false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ak.ValidateCertificate(tc.info, tc.autoRenew, 365)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Errorf("renew = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMapChainCertState(t *testing.T) {
	cases := map[certv1.State]string{
		certv1.CertificateValid:   CertificateStateValid,
		certv1.CertificateRevoked: CertificateStateRevoked,
		certv1.CertificateStateInvalid: "",
	}
	for in, want := range cases {
		if got := mapChainCertState(in); got != want {
			t.Errorf("mapChainCertState(%v) = %q, want %q", in, got, want)
		}
	}
}

// TestInputValidation covers the early returns in the chain-touching methods
// — they don't reach the network when inputs are missing.
func TestInputValidation(t *testing.T) {
	ak := &AkashClient{}
	ctx := newTestContext()

	if _, err := ak.CreateCertificate(ctx, nil, ""); err == nil || !strings.Contains(err.Error(), "owner") {
		t.Errorf("CreateCertificate with empty owner: want owner error, got %v", err)
	}
	if _, err := ak.GetCertificate(ctx, "", ""); err == nil {
		t.Errorf("GetCertificate with empty inputs: expected error")
	}
	if err := ak.RevokeCertificate(ctx, "", ""); err == nil {
		t.Errorf("RevokeCertificate with empty inputs: expected error")
	}
	if _, err := ak.GetCertificates(ctx, ""); err == nil {
		t.Errorf("GetCertificates with empty owner: expected error")
	}
}
