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

package bid

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	akashv1alpha1 "github.com/overlock-network/provider-akash/apis/akash/v1alpha1"
	clienttypes "github.com/overlock-network/provider-akash/internal/client/types"
)

func TestBidTypeMetadata(t *testing.T) {
	// Test that our Bid type has the expected metadata
	expectedKind := "Bid"
	if akashv1alpha1.BidKind != expectedKind {
		t.Errorf("Expected BidKind to be %s, got %s", expectedKind, akashv1alpha1.BidKind)
	}

	expectedGroup := "akash.overlock.network"
	if akashv1alpha1.Group != expectedGroup {
		t.Errorf("Expected Group to be %s, got %s", expectedGroup, akashv1alpha1.Group)
	}
}

func TestBidCreation(t *testing.T) {
	// Test that we can create a Bid instance
	autoAccept := true
	maxPrice := int64(1000000)

	bid := &akashv1alpha1.Bid{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bid",
			Namespace: "default",
		},
		Spec: akashv1alpha1.BidSpec{
			ForProvider: akashv1alpha1.BidParameters{
				DeploymentRef: akashv1alpha1.DeploymentReference{
					Name:      "test-deployment",
					Namespace: nil, // Same namespace
				},
				AutoAccept: &autoAccept,
				MaxPrice:   &maxPrice,
			},
		},
	}

	// Basic validation
	if bid.Name != "test-bid" {
		t.Errorf("Expected name to be test-bid, got %s", bid.Name)
	}

	if bid.Spec.ForProvider.DeploymentRef.Name != "test-deployment" {
		t.Errorf("Expected deployment ref name to be test-deployment, got %s", bid.Spec.ForProvider.DeploymentRef.Name)
	}

	if bid.Spec.ForProvider.AutoAccept == nil || *bid.Spec.ForProvider.AutoAccept != true {
		t.Error("Expected AutoAccept to be true")
	}

	if bid.Spec.ForProvider.MaxPrice == nil || *bid.Spec.ForProvider.MaxPrice != 1000000 {
		t.Error("Expected MaxPrice to be 1000000")
	}
}

func TestBidFiltering(t *testing.T) {
	// Test the GetLowestPriceBid method from types
	bids := clienttypes.Bids{
		{
			Id: clienttypes.BidId{
				Provider: "provider1",
				Dseq:     "123",
				Gseq:     "1",
				Oseq:     "1",
				Owner:    "owner1",
			},
			Price: clienttypes.BidPrice{
				Amount: 2.0,
				Denom:  "akt",
			},
			State: "open",
		},
		{
			Id: clienttypes.BidId{
				Provider: "provider2",
				Dseq:     "123",
				Gseq:     "1",
				Oseq:     "1",
				Owner:    "owner1",
			},
			Price: clienttypes.BidPrice{
				Amount: 1.5,
				Denom:  "akt",
			},
			State: "open",
		},
	}

	// Test lowest price bid selection
	lowestBid := bids.GetLowestPriceBid()
	if lowestBid == nil {
		t.Fatal("Expected to find lowest price bid")
	}
	if lowestBid.Price.Amount != 1.5 {
		t.Errorf("Expected lowest price to be 1.5, got %f", lowestBid.Price.Amount)
	}
	if lowestBid.Id.Provider != "provider2" {
		t.Errorf("Expected lowest price provider to be provider2, got %s", lowestBid.Id.Provider)
	}

	// Test empty bids slice
	empty := clienttypes.Bids{}
	if len(empty) != 0 {
		t.Error("Expected empty bids slice to have length 0")
	}
	if empty.GetLowestPriceBid() != nil {
		t.Error("Expected nil for empty bids slice")
	}
}
