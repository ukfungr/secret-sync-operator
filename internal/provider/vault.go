package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vault "github.com/hashicorp/vault/api"
)

// VaultProvider retrieves secrets from HashiCorp Vault.
type VaultProvider struct {
	client        *vault.Client
	mount         string
	authenticator vaultAuthenticator
}

type VaultConfig struct {
	Address string           `json:"address"`
	Mount   string           `json:"mount"`
	Auth    *vaultAuthConfig `json:"auth"`
}

// vaultAuthConfig contains the configuration used to select
// and configure the Vault authentication method.
type vaultAuthConfig struct {
	Type string `json:"type"`

	TokenSecretRef *struct {
		Name string `json:"name"`
	} `json:"tokenSecretRef,omitempty"`
}

// NewVaultProvider creates a Vault provider using the configured
// Vault address, authentication token, and KV mount.
func NewVaultProvider(
	kubeClient client.Client,
	namespace string,
	rawConfig runtime.RawExtension,
) (*VaultProvider, error) {

	var config VaultConfig

	// Decode config
	if err := json.Unmarshal(rawConfig.Raw, &config); err != nil {
		return nil, fmt.Errorf(
			"failed to parse Vault provider config: %w",
			err,
		)
	}

	// Validate required values
	if config.Address == "" {
		return nil, fmt.Errorf("vault address is required")
	}

	if config.Mount == "" {
		return nil, fmt.Errorf("vault mount is required")
	}

	// Create Vault client using decoded address
	vaultClientConfig := vault.DefaultConfig()
	vaultClientConfig.Address = config.Address

	vaultClient, err := vault.NewClient(vaultClientConfig)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create Vault client: %w",
			err,
		)
	}

	// Create the authenticator based on the configured
	// authentication method.
	authenticator, err := newVaultAuthenticator(
		*config.Auth,
		kubeClient,
		namespace,
	)
	if err != nil {
		return nil, err
	}

	return &VaultProvider{
		client:        vaultClient,
		mount:         config.Mount,
		authenticator: authenticator,
	}, nil
}

// GetSecret retrieves a secret from Vault using the provided path.
func (p *VaultProvider) GetSecret(
	ctx context.Context,
	key string,
) (map[string][]byte, error) {

	// Authenticate the Vault client using the configured
	// authentication method.
	err := p.authenticator.Authenticate(ctx, p.client)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to authenticate to Vault: %w",
			err,
		)
	}

	// Retrieve the secret from Vault.
	secret, err := p.client.KVv2(p.mount).Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get Vault secret %q: %w",
			key,
			err,
		)
	}

	if secret == nil {
		return nil, fmt.Errorf(
			"vault secret %q was not found",
			key,
		)
	}

	// Convert Vault secret values to the format expected by SecretProvider.
	data := make(map[string][]byte, len(secret.Data))

	for dataKey, value := range secret.Data {
		switch v := value.(type) {
		case string:
			data[dataKey] = []byte(v)
		default:
			data[dataKey] = fmt.Append(nil, v)
		}
	}

	return data, nil
}
