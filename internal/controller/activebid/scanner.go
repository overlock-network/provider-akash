/*
Copyright 2024 The Akash Provider Authors.

Licensed under the Apache License, Version 2.0 (the "License");
*/

package activebid

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	kubeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/controller"

	akashv1alpha1 "github.com/overlock-network/provider-akash/apis/akash/v1alpha1"
	"github.com/overlock-network/provider-akash/apis/resource/v1alpha1"
	apisv1alpha1 "github.com/overlock-network/provider-akash/apis/v1alpha1"
	client "github.com/overlock-network/provider-akash/internal/client"
)

// bidScanner is a secondary reconciler living inside the activebid module.
// It watches Deployment CRs and, for each on-chain Deployment, queries the
// chain for bids and creates one ActiveBid CR per chain bid. The existing
// per-CR ActiveBid Observe (managed reconciler) then refreshes each
// ActiveBid's status from chain on every poll.
//
// Putting the producer here (rather than inside BidPolicy or the Deployment
// controller) keeps "one entity → one controller": every aspect of the
// ActiveBid lifecycle — creation, observation, and status — lives in this
// package.
type bidScanner struct {
	kubeClient kubeclient.Client
}

// SetupScanner wires the bidScanner reconciler into the manager. Called from
// activebid.Setup alongside the standard managed-resource controller.
func SetupScanner(mgr ctrl.Manager, _ controller.Options) error {
	r := &bidScanner{kubeClient: mgr.GetClient()}

	return ctrl.NewControllerManagedBy(mgr).
		Named("activebid-scanner").
		For(&v1alpha1.Deployment{}).
		Complete(r)
}

// Reconcile fires for each Deployment event. If the Deployment is on chain
// (DSEQ + owner populated) we query bids and ensure an ActiveBid CR exists
// for each one. Errors from the chain query are non-fatal: the reconcile
// returns nil so controller-runtime requeues on its own schedule.
func (r *bidScanner) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	dep := &v1alpha1.Deployment{}
	if err := r.kubeClient.Get(ctx, req.NamespacedName, dep); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	dseq := dep.Status.AtProvider.DeploymentId
	owner := dep.Status.AtProvider.Owner
	if dseq == "" || owner == "" {
		// Not on chain yet; the next Deployment event will retrigger.
		return reconcile.Result{}, nil
	}

	akash, err := r.buildAkashClient(ctx, dep)
	if err != nil {
		fmt.Printf("activebid-scanner: cannot build Akash client for %s: %v\n", dep.Name, err)
		return reconcile.Result{}, nil
	}

	bids, err := akash.GetBids(ctx, dseq, owner)
	if err != nil {
		fmt.Printf("activebid-scanner: GetBids %s/%s: %v\n", owner, dseq, err)
		return reconcile.Result{}, nil
	}

	for _, b := range bids {
		bidId := fmt.Sprintf("%s-%s-%s-%s-%s", b.Id.Owner, b.Id.Dseq, b.Id.Gseq, b.Id.Oseq, b.Id.Provider)
		if err := r.ensureActiveBid(ctx, dep, bidId); err != nil {
			fmt.Printf("activebid-scanner: ensureActiveBid %s: %v\n", bidId, err)
		}
	}
	return reconcile.Result{}, nil
}

// buildAkashClient constructs a chain-talking AkashClient using the
// Deployment's ProviderConfig. It mirrors the resolution path used by every
// other managed reconciler in this provider.
func (r *bidScanner) buildAkashClient(ctx context.Context, dep *v1alpha1.Deployment) (*client.AkashClient, error) {
	ref := dep.GetProviderConfigReference()
	if ref == nil {
		return nil, errors.New("deployment has no providerConfigRef")
	}
	pc := &apisv1alpha1.ProviderConfig{}
	if err := r.kubeClient.Get(ctx, types.NamespacedName{Name: ref.Name}, pc); err != nil {
		return nil, errors.Wrap(err, "cannot get ProviderConfig")
	}
	pcInfo := client.ProviderConfigInfo{
		Source:              pc.Spec.Credentials.Source,
		CredentialSelectors: pc.Spec.Credentials.CommonCredentialSelectors,
		Configuration:       pc.Spec.Configuration,
	}
	if pc.Spec.Passphrase != nil {
		pcInfo.PassphraseSource = &pc.Spec.Passphrase.Source
		pcInfo.PassphraseSelectors = &pc.Spec.Passphrase.CommonCredentialSelectors
	}
	return client.NewFromManagedResource(ctx, r.kubeClient, nil, dep, pcInfo)
}

// ensureActiveBid creates an ActiveBid CR for the given chain bidId if one
// doesn't exist. Idempotent — safe to call on every Deployment reconcile.
func (r *bidScanner) ensureActiveBid(ctx context.Context, dep *v1alpha1.Deployment, bidId string) error {
	name := scannerActiveBidName(dep.Name, bidId)

	existing := &akashv1alpha1.ActiveBid{}
	err := r.kubeClient.Get(ctx, kubeclient.ObjectKey{Name: name}, existing)
	switch {
	case err == nil:
		return nil
	case !apierrors.IsNotFound(err):
		return err
	}

	depNs := dep.Namespace
	truthy := true
	ab := &akashv1alpha1.ActiveBid{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"akash.overlock.network/deployment": dep.Name,
				"akash.overlock.network/dseq":       dep.Status.AtProvider.DeploymentId,
			},
			// Tying ActiveBids to the Deployment via ownerRef means K8s
			// garbage-collects them when the Deployment CR is deleted —
			// no stale ActiveBids referencing a previous DSEQ if the
			// user re-creates the Deployment under the same name.
			OwnerReferences: []metav1.OwnerReference{{
				// APIVersion/Kind aren't populated on typed Get() — fill
				// them from the resource type so the GC controller can
				// resolve the owner correctly.
				APIVersion:         v1alpha1.SchemeGroupVersion.String(),
				Kind:               v1alpha1.DeploymentKind,
				Name:               dep.Name,
				UID:                dep.UID,
				Controller:         &truthy,
				BlockOwnerDeletion: &truthy,
			}},
		},
		Spec: akashv1alpha1.ActiveBidSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: dep.GetProviderConfigReference(),
			},
			ForProvider: akashv1alpha1.ActiveBidParameters{
				DeploymentRef: akashv1alpha1.DeploymentReference{
					Name:      dep.Name,
					Namespace: &depNs,
				},
				BidId: bidId,
			},
		},
	}
	return r.kubeClient.Create(ctx, ab)
}

// scannerActiveBidName produces a stable DNS-1123 name for an ActiveBid CR.
// Hash-based to keep the result short and globally unique even when bidIds
// contain long bech32 addresses.
func scannerActiveBidName(deploymentName, bidId string) string {
	sum := sha256.Sum256([]byte(bidId))
	return fmt.Sprintf("%s-bid-%x", deploymentName, sum[:6])
}

