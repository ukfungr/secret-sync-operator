package provider

import "context"

// SecretProvider defines the interface for retrieving secrets
// from an external secret management provider.
type SecretProvider interface {
	GetSecret(ctx context.Context, key string) (map[string][]byte, error)
}
