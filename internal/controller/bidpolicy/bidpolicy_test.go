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

package bidpolicy

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	akashv1alpha1 "github.com/overlock-network/provider-akash/apis/akash/v1alpha1"
)

func TestBidPolicyTypeMetadata(t *testing.T) {
	// Test that our BidPolicy type has the expected metadata
	expectedKind := "BidPolicy"
	if akashv1alpha1.BidPolicyKind != expectedKind {
		t.Errorf("Expected BidPolicyKind to be %s, got %s", expectedKind, akashv1alpha1.BidPolicyKind)
	}

	expectedGroup := "akash.overlock.network"
	if akashv1alpha1.Group != expectedGroup {
		t.Errorf("Expected Group to be %s, got %s", expectedGroup, akashv1alpha1.Group)
	}
}

func TestBidPolicyCreation(t *testing.T) {
	// Test that we can create a BidPolicy instance
	maxPrice := int64(1000)
	minScore := int32(75)

	bidPolicy := &akashv1alpha1.BidPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bidpolicy",
			Namespace: "default",
		},
		Spec: akashv1alpha1.BidPolicySpec{
			ForProvider: akashv1alpha1.BidPolicyParameters{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "web-server",
					},
				},
				MaxPrice:          &maxPrice,
				MinProviderScore:  &minScore,
				SelectionStrategy: "lowest-price",
				AutoAccept:        true,
			},
		},
	}

	// Basic validation
	if bidPolicy.Name != "test-bidpolicy" {
		t.Errorf("Expected name to be test-bidpolicy, got %s", bidPolicy.Name)
	}

	if bidPolicy.Spec.ForProvider.Selector.MatchLabels["app"] != "web-server" {
		t.Errorf("Expected app label to be web-server, got %s", bidPolicy.Spec.ForProvider.Selector.MatchLabels["app"])
	}

	if *bidPolicy.Spec.ForProvider.MaxPrice != maxPrice {
		t.Errorf("Expected MaxPrice to be %d, got %d", maxPrice, *bidPolicy.Spec.ForProvider.MaxPrice)
	}

	if *bidPolicy.Spec.ForProvider.MinProviderScore != minScore {
		t.Errorf("Expected MinProviderScore to be %d, got %d", minScore, *bidPolicy.Spec.ForProvider.MinProviderScore)
	}
}

func TestBidPolicyValidation(t *testing.T) {
	service := &BidPolicyService{}

	testCases := []struct {
		name      string
		policy    *akashv1alpha1.BidPolicy
		wantError bool
	}{
		{
			name: "valid policy with selector",
			policy: &akashv1alpha1.BidPolicy{
				Spec: akashv1alpha1.BidPolicySpec{
					ForProvider: akashv1alpha1.BidPolicyParameters{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test"},
						},
						SelectionStrategy: "lowest-price",
					},
				},
			},
			wantError: false,
		},
		{
			name: "valid policy with deployment ref",
			policy: &akashv1alpha1.BidPolicy{
				Spec: akashv1alpha1.BidPolicySpec{
					ForProvider: akashv1alpha1.BidPolicyParameters{
						DeploymentRef: &akashv1alpha1.DeploymentReference{
							Name: "test-deployment",
						},
						SelectionStrategy: "best-score",
					},
				},
			},
			wantError: false,
		},
		{
			name: "invalid policy - no selector or deployment ref",
			policy: &akashv1alpha1.BidPolicy{
				Spec: akashv1alpha1.BidPolicySpec{
					ForProvider: akashv1alpha1.BidPolicyParameters{
						SelectionStrategy: "lowest-price",
					},
				},
			},
			wantError: true,
		},
		{
			name: "invalid policy - bad selection strategy",
			policy: &akashv1alpha1.BidPolicy{
				Spec: akashv1alpha1.BidPolicySpec{
					ForProvider: akashv1alpha1.BidPolicyParameters{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test"},
						},
						SelectionStrategy: "invalid-strategy",
					},
				},
			},
			wantError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.ValidatePolicy(context.Background(), tc.policy)
			if (err != nil) != tc.wantError {
				t.Errorf("ValidatePolicy() error = %v, wantError %v", err, tc.wantError)
			}
		})
	}
}

func TestBidEvaluation(t *testing.T) {
	service := &BidPolicyService{}

	testCases := []struct {
		name             string
		policy           *akashv1alpha1.BidPolicy
		bids             []akashv1alpha1.ActiveBid
		expectedCount    int
		expectedProvider string
	}{
		{
			name: "filters by max price and excluded providers",
			policy: &akashv1alpha1.BidPolicy{
				Spec: akashv1alpha1.BidPolicySpec{
					ForProvider: akashv1alpha1.BidPolicyParameters{
						MaxPrice:          func() *int64 { v := int64(1000); return &v }(),
						ExcludedProviders: []string{"bad-provider"},
					},
				},
			},
			bids: []akashv1alpha1.ActiveBid{
				{
					Status: akashv1alpha1.ActiveBidStatus{
						AtProvider: akashv1alpha1.ActiveBidObservation{
							Provider: "good-provider",
							Price: &akashv1alpha1.ActiveBidPriceStatus{
								Amount: "500",
								Denom:  "uakt",
							},
						},
					},
				},
				{
					Status: akashv1alpha1.ActiveBidStatus{
						AtProvider: akashv1alpha1.ActiveBidObservation{
							Provider: "expensive-provider",
							Price: &akashv1alpha1.ActiveBidPriceStatus{
								Amount: "2000",
								Denom:  "uakt",
							},
						},
					},
				},
				{
					Status: akashv1alpha1.ActiveBidStatus{
						AtProvider: akashv1alpha1.ActiveBidObservation{
							Provider: "bad-provider",
							Price: &akashv1alpha1.ActiveBidPriceStatus{
								Amount: "300",
								Denom:  "uakt",
							},
						},
					},
				},
			},
			expectedCount:    1,
			expectedProvider: "good-provider",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			eligibleBids, err := service.EvaluateBids(context.Background(), tc.policy, tc.bids)
			if err != nil {
				t.Fatalf("EvaluateBids() error = %v", err)
			}

			if len(eligibleBids) != tc.expectedCount {
				t.Errorf("EvaluateBids() count = %v, want %v", len(eligibleBids), tc.expectedCount)
			}

			if tc.expectedCount > 0 && len(eligibleBids) > 0 {
				if eligibleBids[0].Status.AtProvider.Provider != tc.expectedProvider {
					t.Errorf("EvaluateBids() provider = %v, want %v",
						eligibleBids[0].Status.AtProvider.Provider, tc.expectedProvider)
				}
			}
		})
	}
}

func TestBidSelection(t *testing.T) {
	service := &BidPolicyService{}

	testCases := []struct {
		name             string
		policy           *akashv1alpha1.BidPolicy
		bids             []akashv1alpha1.ActiveBid
		expectedBidName  string
		expectedProvider string
		wantReason       bool
	}{
		{
			name: "lowest price selection",
			policy: &akashv1alpha1.BidPolicy{
				Spec: akashv1alpha1.BidPolicySpec{
					ForProvider: akashv1alpha1.BidPolicyParameters{
						SelectionStrategy: "lowest-price",
					},
				},
			},
			bids: []akashv1alpha1.ActiveBid{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bid1"},
					Status: akashv1alpha1.ActiveBidStatus{
						AtProvider: akashv1alpha1.ActiveBidObservation{
							Provider: "provider1",
							Price: &akashv1alpha1.ActiveBidPriceStatus{
								Amount: "1000",
								Denom:  "uakt",
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bid2"},
					Status: akashv1alpha1.ActiveBidStatus{
						AtProvider: akashv1alpha1.ActiveBidObservation{
							Provider: "provider2",
							Price: &akashv1alpha1.ActiveBidPriceStatus{
								Amount: "500",
								Denom:  "uakt",
							},
						},
					},
				},
			},
			expectedBidName:  "bid2",
			expectedProvider: "provider2",
			wantReason:       true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			selectedBid, reason, err := service.SelectBid(context.Background(), tc.policy, tc.bids)
			if err != nil {
				t.Fatalf("SelectBid() error = %v", err)
			}

			if selectedBid.Name != tc.expectedBidName {
				t.Errorf("SelectBid() bid name = %v, want %v", selectedBid.Name, tc.expectedBidName)
			}

			if selectedBid.Status.AtProvider.Provider != tc.expectedProvider {
				t.Errorf("SelectBid() provider = %v, want %v",
					selectedBid.Status.AtProvider.Provider, tc.expectedProvider)
			}

			if tc.wantReason && reason == "" {
				t.Error("SelectBid() expected selection reason to be provided")
			}
		})
	}
}

func TestPreferredProviderSelection(t *testing.T) {
	service := &BidPolicyService{}

	testCases := []struct {
		name             string
		policy           *akashv1alpha1.BidPolicy
		bids             []akashv1alpha1.ActiveBid
		expectedProvider string
		wantReason       bool
	}{
		{
			name: "preferred provider selection over lower price",
			policy: &akashv1alpha1.BidPolicy{
				Spec: akashv1alpha1.BidPolicySpec{
					ForProvider: akashv1alpha1.BidPolicyParameters{
						SelectionStrategy:  "preferred-first",
						PreferredProviders: []string{"trusted-provider"},
					},
				},
			},
			bids: []akashv1alpha1.ActiveBid{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bid1"},
					Status: akashv1alpha1.ActiveBidStatus{
						AtProvider: akashv1alpha1.ActiveBidObservation{
							Provider: "prefered-provider",
							Price: &akashv1alpha1.ActiveBidPriceStatus{
								Amount: "500",
								Denom:  "uakt",
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bid2"},
					Status: akashv1alpha1.ActiveBidStatus{
						AtProvider: akashv1alpha1.ActiveBidObservation{
							Provider: "trusted-provider",
							Price: &akashv1alpha1.ActiveBidPriceStatus{
								Amount: "1000",
								Denom:  "uakt",
							},
						},
					},
				},
			},
			expectedProvider: "trusted-provider",
			wantReason:       true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			selectedBid, reason, err := service.SelectBid(context.Background(), tc.policy, tc.bids)
			if err != nil {
				t.Fatalf("SelectBid() error = %v", err)
			}

			if selectedBid.Status.AtProvider.Provider != tc.expectedProvider {
				t.Errorf("SelectBid() provider = %v, want %v",
					selectedBid.Status.AtProvider.Provider, tc.expectedProvider)
			}

			if tc.wantReason && reason == "" {
				t.Error("SelectBid() expected selection reason to be provided")
			}
		})
	}
}
