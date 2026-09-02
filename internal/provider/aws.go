package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// AWSProvider retrieves secrets from AWS Secrets Manager.
type AWSProvider struct {
	client secretsManagerClient
}

// secretsManagerClient defines the AWS Secrets Manager operations
// required by AWSProvider.
type secretsManagerClient interface {
	GetSecretValue(
		ctx context.Context,
		params *secretsmanager.GetSecretValueInput,
		optFns ...func(*secretsmanager.Options),
	) (*secretsmanager.GetSecretValueOutput, error)
}

// NewAWSProvider creates an AWS Secrets Manager provider using the
// AWS configuration available in the environment.
func NewAWSProvider() (*AWSProvider, error) {

	// Load the AWS configuration using the standard AWS SDK credential
	// and region resolution mechanisms.
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf(
			"failed to load AWS configuration: %w",
			err,
		)
	}

	// Optionally overrides the AWS endpoint when using an alternative service,
	// such as LocalStack, for local development or testing.
	//
	// If an alternative endpoint is not needed, this option can be removed.
	options := func(o *secretsmanager.Options) {
		if endpoint := os.Getenv("AWS_ENDPOINT_URL"); endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}

	}

	return &AWSProvider{
		client: secretsmanager.NewFromConfig(cfg, options),
	}, nil
}

// GetSecret retrieves a secret from AWS Secrets Manager and converts
// its JSON fields into the format expected by the SecretProvider interface.
func (p *AWSProvider) GetSecret(
	ctx context.Context,
	key string,
) (map[string][]byte, error) {

	// 1. Request the secret from AWS Secrets Manager.
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(key),
	}

	output, err := p.client.GetSecretValue(ctx, input)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get AWS secret %q: %w",
			key,
			err,
		)
	}

	// 2. Ensure the secret is stored as a SecretString.
	if output.SecretString == nil {
		return nil, fmt.Errorf(
			"AWS secret %q does not contain a SecretString",
			key,
		)
	}

	// 3. Parse the SecretString as a JSON object.
	var secretData map[string]string

	err = json.Unmarshal(
		[]byte(*output.SecretString),
		&secretData,
	)

	if err != nil {
		return nil, fmt.Errorf(
			"failed to parse AWS secret %q as JSON: %w",
			key,
			err,
		)
	}

	// 4. Convert the JSON values to []byte so they can be stored
	// in a Kubernetes Secret.
	data := make(map[string][]byte, len(secretData))

	for key, value := range secretData {
		data[key] = []byte(value)
	}

	return data, nil
}
