package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	akashclient "github.com/overlock-network/provider-akash/internal/client"
)

func main() {
	ctx := context.Background()

	// Get Kubernetes client
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatal(err)
	}
	
	kubeClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		log.Fatal(err)
	}

	// Example 1: Create client with direct credentials (legacy - not recommended)
	config1 := akashclient.AkashProviderConfiguration{
		Creds:          []byte("my-private-key"),
		KeyName:        akashclient.DefaultKeyName,
		KeyringBackend: akashclient.DefaultKeyringBackend,
		Net:            akashclient.DefaultNet,
		ChainId:        akashclient.DefaultChainId,
		Node:           akashclient.DefaultNode,
		ProvidersApi:   akashclient.DefaultProvidersApi,
	}
	client1 := akashclient.New(ctx, config1)
	fmt.Println("Created legacy client:", client1.GetPath())

	// Example 2: Create client with secret reference (legacy - not recommended)
	secretRef := akashclient.SecretReference{
		Name:      "akash-credentials",
		Namespace: "default",
		Key:       "private-key",
	}
	config2 := akashclient.AkashProviderConfiguration{
		KeyName:        akashclient.DefaultKeyName,
		KeyringBackend: akashclient.DefaultKeyringBackend,
		Net:            akashclient.DefaultNet,
		ChainId:        akashclient.DefaultChainId,
		Node:           akashclient.DefaultNode,
		ProvidersApi:   akashclient.DefaultProvidersApi,
	}
	client2 := akashclient.NewWithSecretRef(ctx, kubeClient, secretRef, config2)
	
	// Set custom cache TTL
	client2.SetCredentialCacheTTL(10 * time.Minute)
	
	// Get credentials (will load from secret and cache)
	creds, err := client2.GetCredentials()
	if err != nil {
		fmt.Printf("Error loading credentials: %v\n", err)
	} else {
		fmt.Printf("Loaded credentials from secret (length: %d bytes)\n", len(creds))
	}

	// Example 3: Create client from ProviderConfig-style credentials (legacy)
	credSource := xpv1.CredentialsSourceSecret
	credSelectors := xpv1.CommonCredentialSelectors{
		SecretRef: &xpv1.SecretKeySelector{
			SecretReference: xpv1.SecretReference{
				Name:      "akash-credentials",
				Namespace: "default",
			},
			Key: "private-key",
		},
	}
	
	client3, err := akashclient.NewFromProviderConfig(ctx, kubeClient, credSource, credSelectors, config2)
	if err != nil {
		fmt.Printf("Error creating client from ProviderConfig: %v\n", err)
	} else {
		fmt.Println("Created client from ProviderConfig (legacy)")
		
		// Force refresh of credentials
		if err := client3.RefreshCredentials(); err != nil {
			fmt.Printf("Error refreshing credentials: %v\n", err)
		} else {
			fmt.Println("Credentials refreshed successfully")
		}
	}

	// Example 4: Create client from managed resource (NEW APPROACH)
	// This would typically be done in a controller with an actual managed resource
	/*
	// Mock managed resource (in reality this would be your v1alpha1.Deployment)
	managedResource := &v1alpha1.Deployment{} // Your actual managed resource
	usage := resource.NewProviderConfigUsageTracker(kubeClient, &apisv1alpha1.ProviderConfigUsage{})
	
	pcInfo := akashclient.ProviderConfigInfo{
		Source: credSource,
		CredentialSelectors: credSelectors,
	}
	
	client4, err := akashclient.NewFromManagedResource(ctx, kubeClient, usage, managedResource, pcInfo, config2)
	if err != nil {
		fmt.Printf("Error creating client from managed resource: %v\n", err)
	} else {
		fmt.Println("Created client from managed resource - credentials auto-loaded!")
		
		// Credentials are automatically loaded and cached
		creds4, err := client4.GetCredentials()
		if err != nil {
			fmt.Printf("Error getting credentials: %v\n", err)
		} else {
			fmt.Printf("Auto-loaded credentials (length: %d bytes)\n", len(creds4))
		}
	}
	*/
	
	fmt.Println("\nNEW APPROACH:")
	fmt.Println("In controllers, use NewFromManagedResource() which automatically:")
	fmt.Println("1. Loads credentials from ProviderConfig referenced by managed resource")
	fmt.Println("2. Loads configuration from ProviderConfig.spec.configuration (with sensible defaults)")
	fmt.Println("3. Handles credential caching and refresh")
	fmt.Println("4. Tracks ProviderConfig usage")
	fmt.Println("5. Returns ready-to-use client with no additional setup needed")
	fmt.Println("\nProviderConfig now supports Akash-specific configuration:")
	fmt.Println("- keyName, keyringBackend, net, version, chainId")
	fmt.Println("- node, home, path, providersApi, accountAddress")
	fmt.Println("- All fields are optional with sensible defaults")
	fmt.Println("- Controllers no longer need hardcoded configuration!")
}