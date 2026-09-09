//go:build integration

// Vault integration test.
//
// This test is skipped automatically when VAULT_ADDR or VAULT_TOKEN
// is not set.
// Set both variables to run the test against a Vault instance.

package provider

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestVaultProvider(t *testing.T) {
	vaultAddress := os.Getenv("VAULT_ADDR")
	if vaultAddress == "" {
		t.Skip("VAULT_ADDR is not set")
	}

	vaultToken := os.Getenv("VAULT_TOKEN")
	if vaultToken == "" {
		t.Skip("VAULT_TOKEN is not set")
	}

	g := NewWithT(t)

	ctx := context.Background()

	// Create a fake Kubernetes client containing the Vault token Secret.
	kubeScheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(kubeScheme)).To(Succeed())

	tokenSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vault-token",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"token": []byte(vaultToken),
		},
	}

	kubeClient := fake.NewClientBuilder().
		WithScheme(kubeScheme).
		WithObjects(tokenSecret).
		Build()

	// Build the same provider configuration that a SecretSync
	// resource would provide.
	vaultProviderConfig := struct {
		Address string `json:"address"`
		Mount   string `json:"mount"`
		Auth    struct {
			Type           string `json:"type"`
			TokenSecretRef struct {
				Name string `json:"name"`
			} `json:"tokenSecretRef"`
		} `json:"auth"`
	}{
		Address: vaultAddress,
		Mount:   "secret",
	}

	vaultProviderConfig.Auth.Type = "token"
	vaultProviderConfig.Auth.TokenSecretRef.Name = "vault-token"

	configData, err := json.Marshal(vaultProviderConfig)
	g.Expect(err).NotTo(HaveOccurred())

	rawConfig := runtime.RawExtension{
		Raw: configData,
	}

	vaultProvider, err := NewVaultProvider(
		kubeClient,
		"default",
		rawConfig,
	)
	g.Expect(err).NotTo(HaveOccurred())

	data, err := vaultProvider.GetSecret(ctx, "test-secret")
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(data).To(Equal(map[string][]byte{
		"username": []byte("admin"),
		"password": []byte("new-password"),
	}))
}
