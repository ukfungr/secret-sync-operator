package provider

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vault "github.com/hashicorp/vault/api"
)

const vaultAuthTypeToken = "token"

// vaultAuthenticator authenticates a Vault client.
type vaultAuthenticator interface {
	Authenticate(ctx context.Context, vaultClient *vault.Client) error
}

// tokenAuthenticator authenticates to Vault using a token
// stored in a Kubernetes Secret.
type tokenAuthenticator struct {
	kubeClient      client.Client
	namespace       string
	tokenSecretName string
}

// Authenticate retrieves the Vault token from a Kubernetes Secret
// and configures the Vault client with it.
func (a *tokenAuthenticator) Authenticate(
	ctx context.Context,
	vaultClient *vault.Client,
) error {

	var tokenSecret corev1.Secret

	err := a.kubeClient.Get(
		ctx,
		types.NamespacedName{
			Name:      a.tokenSecretName,
			Namespace: a.namespace,
		},
		&tokenSecret,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to get Vault token Secret %q: %w",
			a.tokenSecretName,
			err,
		)
	}

	token, ok := tokenSecret.Data["token"]
	if !ok {
		return fmt.Errorf(
			"vault token Secret %q does not contain key %q",
			a.tokenSecretName,
			"token",
		)
	}

	vaultClient.SetToken(string(token))

	return nil
}

// newVaultAuthenticator creates the configured Vault authentication
// implementation.
func newVaultAuthenticator(
	config vaultAuthConfig,
	kubeClient client.Client,
	namespace string,
) (vaultAuthenticator, error) {

	switch config.Type {
	case vaultAuthTypeToken:
		if config.TokenSecretRef == nil {
			return nil, fmt.Errorf(
				"tokenSecretRef is required for token authentication",
			)
		}

		if config.TokenSecretRef.Name == "" {
			return nil, fmt.Errorf(
				"tokenSecretRef.name is required for token authentication",
			)
		}

		return &tokenAuthenticator{
			kubeClient:      kubeClient,
			namespace:       namespace,
			tokenSecretName: config.TokenSecretRef.Name,
		}, nil

	default:
		return nil, fmt.Errorf(
			"unsupported Vault authentication type: %s",
			config.Type,
		)
	}
}
