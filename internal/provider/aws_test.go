package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type fakeSecretsManagerClient struct {
	output *secretsmanager.GetSecretValueOutput
	err    error
}

func (f *fakeSecretsManagerClient) GetSecretValue(
	ctx context.Context,
	params *secretsmanager.GetSecretValueInput,
	optFns ...func(*secretsmanager.Options),
) (*secretsmanager.GetSecretValueOutput, error) {
	return f.output, f.err
}

func TestAWSProviderGetSecret(t *testing.T) {
	fakeClient := &fakeSecretsManagerClient{
		output: &secretsmanager.GetSecretValueOutput{
			SecretString: aws.String(`{
				"username": "admin",
				"password": "new-password"
			}`),
		},
	}

	provider := &AWSProvider{
		client: fakeClient,
	}

	data, err := provider.GetSecret(
		context.Background(),
		"test-secret",
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := map[string][]byte{
		"username": []byte("admin"),
		"password": []byte("new-password"),
	}

	if string(data["username"]) != string(expected["username"]) {
		t.Errorf("unexpected username: %s", data["username"])
	}

	if string(data["password"]) != string(expected["password"]) {
		t.Errorf("unexpected password: %s", data["password"])
	}
}

func TestAWSProviderGetSecretError(t *testing.T) {
	expectedError := errors.New("AWS error")

	fakeClient := &fakeSecretsManagerClient{
		err: expectedError,
	}

	provider := &AWSProvider{
		client: fakeClient,
	}

	_, err := provider.GetSecret(
		context.Background(),
		"test-secret",
	)

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}
