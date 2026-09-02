// LocalStack integration test.
//
// This test is skipped automatically when AWS_ENDPOINT_URL is not set.
// Set AWS_ENDPOINT_URL to run the test against a LocalStack instance.

package provider

import (
	"context"
	"os"
	"testing"

	. "github.com/onsi/gomega"
)

func TestAWSProviderWithLocalStack(t *testing.T) {
	if os.Getenv("AWS_ENDPOINT_URL") == "" {
		t.Skip("AWS_ENDPOINT_URL is not set")
	}

	g := NewWithT(t)

	ctx := context.Background()

	awsProvider, err := NewAWSProvider()
	g.Expect(err).NotTo(HaveOccurred())

	data, err := awsProvider.GetSecret(ctx, "test-secret")
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(data).To(Equal(map[string][]byte{
		"username": []byte("admin"),
		"password": []byte("new-password"),
	}))
}

