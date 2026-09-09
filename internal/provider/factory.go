package provider

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Factory creates a SecretProvider based on the requested provider type.
type Factory interface {
	GetProvider(
		providerType string,
		config runtime.RawExtension,
		namespace string,
	) (SecretProvider, error)
}

// ProviderFactory creates SecretProvider implementations.
type ProviderFactory struct {
	Client client.Client
}

// GetProvider returns a SecretProvider for the specified provider type.
// It returns an error if the provider type is not supported.
func (f *ProviderFactory) GetProvider(
	providerType string,
	config runtime.RawExtension,
	namespace string,
) (SecretProvider, error) {

	// Select the provider implementation based on the configured provider type.
	switch providerType {
	case "aws":
		// Create the AWS Secrets Manager provider.
		return NewAWSProvider()
	case "vault":
		return NewVaultProvider(
			f.Client,
			namespace,
			config,
		)
	default:
		// Return an error when the requested provider is not supported.
		return nil, fmt.Errorf(
			"unsupported provider type: %s",
			providerType,
		)
	}
}
