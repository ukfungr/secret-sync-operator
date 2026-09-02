package provider

import "fmt"

// Factory creates a SecretProvider based on the requested provider type.
type Factory interface {
	GetProvider(providerType string) (SecretProvider, error)
}

// ProviderFactory creates SecretProvider implementations.
type ProviderFactory struct{}

// GetProvider returns a SecretProvider for the specified provider type. 
// It returns an error if the provider type is not supported.
func (f *ProviderFactory) GetProvider(providerType string) (SecretProvider, error) {

	// Select the provider implementation based on the configured provider type.
	switch providerType {
	case "aws":
		// Create the AWS Secrets Manager provider.
		return NewAWSProvider()
	// case "vault":
	// 	return NewVaultProvider(), nil

	default:
		// Return an error when the requested provider is not supported.
		return nil, fmt.Errorf(
			"unsupported provider type: %s",
			providerType,
		)
	}
}
