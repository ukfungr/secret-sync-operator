package provider

import (
	"testing"
)

func TestNewVaultAuthenticator(t *testing.T) {
	tests := []struct {
		name    string
		config  vaultAuthConfig
		wantErr bool
	}{
		{
			name: "token authentication",
			config: vaultAuthConfig{
				Type: vaultAuthTypeToken,
				TokenSecretRef: &struct {
					Name string `json:"name"`
				}{
					Name: "vault-token",
				},
			},
		},
		{
			name: "token authentication without secret reference",
			config: vaultAuthConfig{
				Type: vaultAuthTypeToken,
			},
			wantErr: true,
		},
		{
			name: "token authentication with empty secret name",
			config: vaultAuthConfig{
				Type: vaultAuthTypeToken,
				TokenSecretRef: &struct {
					Name string `json:"name"`
				}{},
			},
			wantErr: true,
		},
		{
			name: "unsupported authentication type",
			config: vaultAuthConfig{
				Type: "unsupported",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authenticator, err := newVaultAuthenticator(
				tt.config,
				nil,
				"default",
			)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if authenticator == nil {
				t.Fatal("expected authenticator, got nil")
			}

			if _, ok := authenticator.(*tokenAuthenticator); !ok {
				t.Fatalf(
					"expected *tokenAuthenticator, got %T",
					authenticator,
				)
			}
		})
	}
}
