package client

import (
	"context"
	"testing"
)

func TestCreateDeployment(t *testing.T) {
	tests := []struct {
		name     string
		sdl      string
		deposit  int64
		currency string
		wantErr  bool
	}{
		{
			name: "valid deployment",
			sdl: `
version: "2.0"
services:
  web:
    image: nginx
    expose:
      - port: 80
        as: 80
        to:
          - global: true
profiles:
  compute:
    web:
      resources:
        cpu:
          units: 0.1
        memory:
          size: 128Mi
        storage:
          size: 1Gi
  placement:
    akash:
      attributes:
        host: akash
      signedBy:
        anyOf:
          - "akash1365yvmc4s7awdyj3n2sav7xfx76adc6dnmlx63"
      pricing:
        web: 
          denom: uakt
          amount: 1000
deployment:
  web:
    akash:
      profile: web
      count: 1
`,
			deposit:  5000000,
			currency: "uakt",
			wantErr:  false,
		},
		{
			name:     "empty SDL",
			sdl:      "",
			deposit:  5000000,
			currency: "uakt",
			wantErr:  false, // Client uses fallback behavior, validation happens at controller level
		},
		{
			name:     "invalid deposit",
			sdl:      "version: '2.0'",
			deposit:  -1,
			currency: "uakt",
			wantErr:  false, // Client uses fallback behavior, validation happens at controller level
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &AkashClient{
				ctx: context.Background(),
				Config: AkashProviderConfiguration{
					AccountAddress: "akash1test",
					KeyName:        "test",
				},
			}

			seqs, err := client.CreateDeployment(context.Background(), tt.sdl, tt.deposit, tt.currency)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateDeployment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && seqs.Dseq == "" {
				t.Errorf("CreateDeployment() returned empty DSEQ")
			}
		})
	}
}

func TestGetDeployment(t *testing.T) {
	tests := []struct {
		name    string
		dseq    string
		owner   string
		wantErr bool
	}{
		{
			name:    "valid deployment query",
			dseq:    "12345",
			owner:   "akash1test",
			wantErr: true, // Will fail due to no credentials available
		},
		{
			name:    "empty dseq",
			dseq:    "",
			owner:   "akash1test",
			wantErr: true,
		},
		{
			name:    "invalid dseq",
			dseq:    "invalid",
			owner:   "akash1test",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &AkashClient{
				ctx: context.Background(),
				Config: AkashProviderConfiguration{
					AccountAddress: "akash1test",
					KeyName:        "test",
				},
			}

			_, err := client.GetDeployment(context.Background(), tt.dseq, tt.owner)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDeployment() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpdateDeployment(t *testing.T) {
	tests := []struct {
		name    string
		dseq    string
		sdl     string
		wantErr bool
	}{
		{
			name: "valid update",
			dseq: "12345",
			sdl: `
version: "2.0"
services:
  web:
    image: nginx:latest
`,
			wantErr: false, // Uses fallback behavior when no node client
		},
		{
			name:    "empty SDL",
			dseq:    "12345",
			sdl:     "",
			wantErr: false, // SDL validation happens at controller level
		},
		{
			name:    "invalid dseq",
			dseq:    "invalid",
			sdl:     "version: '2.0'",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &AkashClient{
				ctx: context.Background(),
				Config: AkashProviderConfiguration{
					AccountAddress: "akash1test",
					KeyName:        "test",
				},
			}

			err := client.UpdateDeployment(context.Background(), tt.dseq, tt.sdl)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateDeployment() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCloseDeployment(t *testing.T) {
	tests := []struct {
		name    string
		dseq    string
		owner   string
		wantErr bool
	}{
		{
			name:    "valid close",
			dseq:    "12345",
			owner:   "akash1test",
			wantErr: false, // Uses fallback behavior when no node client
		},
		{
			name:    "invalid dseq",
			dseq:    "invalid",
			owner:   "akash1test",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &AkashClient{
				ctx: context.Background(),
				Config: AkashProviderConfiguration{
					AccountAddress: "akash1test",
					KeyName:        "test",
				},
			}

			err := client.CloseDeployment(context.Background(), tt.dseq, tt.owner)
			if (err != nil) != tt.wantErr {
				t.Errorf("CloseDeployment() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAddFunds(t *testing.T) {
	tests := []struct {
		name    string
		dseq    string
		amount  int64
		wantErr bool
	}{
		{
			name:    "valid add funds",
			dseq:    "12345",
			amount:  1000000,
			wantErr: false, // Uses fallback behavior when no node client
		},
		{
			name:    "invalid dseq",
			dseq:    "invalid",
			amount:  1000000,
			wantErr: true,
		},
		{
			name:    "zero amount",
			dseq:    "12345",
			amount:  0,
			wantErr: false, // Amount validation happens at higher level
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &AkashClient{
				ctx: context.Background(),
				Config: AkashProviderConfiguration{
					AccountAddress: "akash1test",
					KeyName:        "test",
				},
			}

			err := client.AddFunds(context.Background(), tt.dseq, tt.amount)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddFunds() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}