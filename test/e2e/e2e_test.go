//go:build e2e
// +build e2e

/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ukfungr/secret-sync-operator/test/utils"
)

var _ = Describe("SecretSync", Ordered, func() {
	AfterEach(func() {
		cleanupSecretSyncResources()
	})

	It("should synchronize a Vault secret into a new Kubernetes Secret", func() {
		By("creating the Vault token Secret")

		tokenSecretManifest := `
apiVersion: v1
kind: Secret
metadata:
  name: vault-token
  namespace: secret-sync-operator-system
type: Opaque
stringData:
  token: dev-only-token
`

		manifestPath := filepath.Join(GinkgoT().TempDir(), "vault-token.yaml")
		err := os.WriteFile(
			manifestPath,
			[]byte(tokenSecretManifest),
			0o644,
		)
		Expect(err).NotTo(HaveOccurred())

		_, err = utils.Run(
			exec.Command(
				"kubectl",
				"apply",
				"-f",
				manifestPath,
			),
		)
		Expect(err).NotTo(HaveOccurred())

		By("creating the SecretSync resource")

		secretSyncManifest := `
apiVersion: ops.example.com/v1alpha1
kind: SecretSync
metadata:
  name: vault-secret-sync
  namespace: secret-sync-operator-system
spec:
  provider:
    type: vault
    config:
      address: http://vault.secret-sync-operator-vault.svc.cluster.local:8200
      mount: secret
      auth:
        type: token
        tokenSecretRef:
          name: vault-token
  remote:
    name: test-secret
  target:
    name: vault-synced-secret
`

		manifestPath = filepath.Join(GinkgoT().TempDir(), "vault-secret-sync.yaml")
		err = os.WriteFile(
			manifestPath,
			[]byte(secretSyncManifest),
			0o644,
		)
		Expect(err).NotTo(HaveOccurred())

		_, err = utils.Run(
			exec.Command(
				"kubectl",
				"apply",
				"-f",
				manifestPath,
			),
		)
		Expect(err).NotTo(HaveOccurred())

		By("waiting for the target Secret to be created")

		Eventually(func() error {
			_, err := utils.Run(
				exec.Command(
					"kubectl",
					"get",
					"secret",
					"vault-synced-secret",
					"-n",
					"secret-sync-operator-system",
				),
			)
			return err
		}, 2*time.Minute, 2*time.Second).Should(Succeed())

		By("checking the synchronized Secret data")

		output, err := utils.Run(
			exec.Command(
				"kubectl",
				"get",
				"secret",
				"vault-synced-secret",
				"-n",
				"secret-sync-operator-system",
				"-o",
				"json",
			),
		)
		Expect(err).NotTo(HaveOccurred())

		var secret struct {
			Data map[string]string `json:"data"`
		}

		Expect(json.Unmarshal([]byte(output), &secret)).To(Succeed())

		username, err := base64.StdEncoding.DecodeString(secret.Data["username"])
		Expect(err).NotTo(HaveOccurred())
		Expect(string(username)).To(Equal("admin"))

		password, err := base64.StdEncoding.DecodeString(secret.Data["password"])
		Expect(err).NotTo(HaveOccurred())
		Expect(string(password)).To(Equal("new-password"))

		By("waiting for SecretSync to report Ready")

		Eventually(func() bool {
			output, err := utils.Run(
				exec.Command(
					"kubectl",
					"get",
					"secretsync",
					"vault-secret-sync",
					"-n",
					"secret-sync-operator-system",
					"-o",
					"json",
				),
			)
			if err != nil {
				return false
			}

			var secretSync struct {
				Status struct {
					Conditions []struct {
						Type    string `json:"type"`
						Status  string `json:"status"`
						Reason  string `json:"reason"`
						Message string `json:"message"`
					} `json:"conditions"`
				} `json:"status"`
			}

			if err := json.Unmarshal([]byte(output), &secretSync); err != nil {
				return false
			}

			for _, condition := range secretSync.Status.Conditions {
				if condition.Type == "Ready" &&
					condition.Status == "True" &&
					condition.Reason == "SecretSynced" {
					return true
				}
			}

			return false
		}, 2*time.Minute, 2*time.Second).Should(BeTrue())
	})

	It("should update an existing Kubernetes Secret when the Vault secret changes", func() {
		By("creating the Vault token Secret")

		tokenSecretManifest := `
apiVersion: v1
kind: Secret
metadata:
  name: vault-token-update
  namespace: secret-sync-operator-system
type: Opaque
stringData:
  token: dev-only-token
`

		manifestPath := filepath.Join(GinkgoT().TempDir(), "vault-token-update.yaml")
		err := os.WriteFile(
			manifestPath,
			[]byte(tokenSecretManifest),
			0o644,
		)
		Expect(err).NotTo(HaveOccurred())

		_, err = utils.Run(
			exec.Command(
				"kubectl",
				"apply",
				"-f",
				manifestPath,
			),
		)
		Expect(err).NotTo(HaveOccurred())

		By("creating the existing target Secret with old data")

		targetSecretManifest := `
apiVersion: v1
kind: Secret
metadata:
  name: vault-update-target
  namespace: secret-sync-operator-system
type: Opaque
stringData:
  username: old-user
  password: old-password
`

		manifestPath = filepath.Join(GinkgoT().TempDir(), "vault-update-target.yaml")
		err = os.WriteFile(
			manifestPath,
			[]byte(targetSecretManifest),
			0o644,
		)
		Expect(err).NotTo(HaveOccurred())

		_, err = utils.Run(
			exec.Command(
				"kubectl",
				"apply",
				"-f",
				manifestPath,
			),
		)
		Expect(err).NotTo(HaveOccurred())

		By("creating the SecretSync resource")

		secretSyncManifest := `
apiVersion: ops.example.com/v1alpha1
kind: SecretSync
metadata:
  name: vault-update-secret-sync
  namespace: secret-sync-operator-system
spec:
  provider:
    type: vault
    config:
      address: http://vault.secret-sync-operator-vault.svc.cluster.local:8200
      mount: secret
      auth:
        type: token
        tokenSecretRef:
          name: vault-token-update
  remote:
    name: test-secret
  target:
    name: vault-update-target
`

		manifestPath = filepath.Join(GinkgoT().TempDir(), "vault-update-secret-sync.yaml")
		err = os.WriteFile(
			manifestPath,
			[]byte(secretSyncManifest),
			0o644,
		)
		Expect(err).NotTo(HaveOccurred())

		_, err = utils.Run(
			exec.Command(
				"kubectl",
				"apply",
				"-f",
				manifestPath,
			),
		)
		Expect(err).NotTo(HaveOccurred())

		By("waiting for the target Secret to contain the Vault data")

		Eventually(func() bool {
			output, err := utils.Run(
				exec.Command(
					"kubectl",
					"get",
					"secret",
					"vault-update-target",
					"-n",
					"secret-sync-operator-system",
					"-o",
					"json",
				),
			)
			if err != nil {
				return false
			}

			var secret struct {
				Data map[string]string `json:"data"`
			}

			if err := json.Unmarshal([]byte(output), &secret); err != nil {
				return false
			}

			username, err := base64.StdEncoding.DecodeString(secret.Data["username"])
			if err != nil || string(username) != "admin" {
				return false
			}

			password, err := base64.StdEncoding.DecodeString(secret.Data["password"])
			if err != nil || string(password) != "new-password" {
				return false
			}

			return true
		}, 2*time.Minute, 2*time.Second).Should(BeTrue())

		By("waiting for SecretSync to report Ready")

		Eventually(func() bool {
			output, err := utils.Run(
				exec.Command(
					"kubectl",
					"get",
					"secretsync",
					"vault-update-secret-sync",
					"-n",
					"secret-sync-operator-system",
					"-o",
					"json",
				),
			)
			if err != nil {
				return false
			}

			var secretSync struct {
				Status struct {
					Conditions []struct {
						Type   string `json:"type"`
						Status string `json:"status"`
						Reason string `json:"reason"`
					} `json:"conditions"`
				} `json:"status"`
			}

			if err := json.Unmarshal([]byte(output), &secretSync); err != nil {
				return false
			}

			for _, condition := range secretSync.Status.Conditions {
				if condition.Type == "Ready" &&
					condition.Status == "True" &&
					condition.Reason == "SecretSynced" {
					return true
				}
			}

			return false
		}, 2*time.Minute, 2*time.Second).Should(BeTrue())
	})

	It("should not update an existing Kubernetes Secret when it is already synchronized", func() {
		By("creating the Vault token Secret")

		tokenSecretManifest := `
apiVersion: v1
kind: Secret
metadata:
  name: vault-token-noop
  namespace: secret-sync-operator-system
type: Opaque
stringData:
  token: dev-only-token
`

		manifestPath := filepath.Join(GinkgoT().TempDir(), "vault-token-noop.yaml")
		err := os.WriteFile(
			manifestPath,
			[]byte(tokenSecretManifest),
			0o644,
		)
		Expect(err).NotTo(HaveOccurred())

		_, err = utils.Run(
			exec.Command(
				"kubectl",
				"apply",
				"-f",
				manifestPath,
			),
		)
		Expect(err).NotTo(HaveOccurred())

		By("creating the existing target Secret with the correct data")

		targetSecretManifest := `
apiVersion: v1
kind: Secret
metadata:
  name: vault-noop-target
  namespace: secret-sync-operator-system
type: Opaque
stringData:
  username: admin
  password: new-password
`

		manifestPath = filepath.Join(GinkgoT().TempDir(), "vault-noop-target.yaml")
		err = os.WriteFile(
			manifestPath,
			[]byte(targetSecretManifest),
			0o644,
		)
		Expect(err).NotTo(HaveOccurred())

		_, err = utils.Run(
			exec.Command(
				"kubectl",
				"apply",
				"-f",
				manifestPath,
			),
		)
		Expect(err).NotTo(HaveOccurred())

		By("recording the target Secret resourceVersion")

		output, err := utils.Run(
			exec.Command(
				"kubectl",
				"get",
				"secret",
				"vault-noop-target",
				"-n",
				"secret-sync-operator-system",
				"-o",
				"json",
			),
		)
		Expect(err).NotTo(HaveOccurred())

		var targetSecretBefore struct {
			Metadata struct {
				ResourceVersion string `json:"resourceVersion"`
			} `json:"metadata"`
			Data map[string]string `json:"data"`
		}

		Expect(json.Unmarshal([]byte(output), &targetSecretBefore)).To(Succeed())

		initialResourceVersion := targetSecretBefore.Metadata.ResourceVersion

		By("creating the SecretSync resource")

		secretSyncManifest := `
apiVersion: ops.example.com/v1alpha1
kind: SecretSync
metadata:
  name: vault-noop-secret-sync
  namespace: secret-sync-operator-system
spec:
  provider:
    type: vault
    config:
      address: http://vault.secret-sync-operator-vault.svc.cluster.local:8200
      mount: secret
      auth:
        type: token
        tokenSecretRef:
          name: vault-token-noop
  remote:
    name: test-secret
  target:
    name: vault-noop-target
`

		manifestPath = filepath.Join(GinkgoT().TempDir(), "vault-noop-secret-sync.yaml")
		err = os.WriteFile(
			manifestPath,
			[]byte(secretSyncManifest),
			0o644,
		)
		Expect(err).NotTo(HaveOccurred())

		_, err = utils.Run(
			exec.Command(
				"kubectl",
				"apply",
				"-f",
				manifestPath,
			),
		)
		Expect(err).NotTo(HaveOccurred())

		By("waiting for SecretSync to report that the Secret is already synchronized")

		Eventually(func() bool {
			output, err := utils.Run(
				exec.Command(
					"kubectl",
					"get",
					"secretsync",
					"vault-noop-secret-sync",
					"-n",
					"secret-sync-operator-system",
					"-o",
					"json",
				),
			)
			if err != nil {
				return false
			}

			var secretSync struct {
				Status struct {
					Conditions []struct {
						Type    string `json:"type"`
						Status  string `json:"status"`
						Reason  string `json:"reason"`
						Message string `json:"message"`
					} `json:"conditions"`
				} `json:"status"`
			}

			if err := json.Unmarshal([]byte(output), &secretSync); err != nil {
				return false
			}

			for _, condition := range secretSync.Status.Conditions {
				if condition.Type == "Ready" &&
					condition.Status == "True" &&
					condition.Reason == "SecretSynced" &&
					condition.Message == "Secret is already synchronized" {
					return true
				}
			}

			return false
		}, 2*time.Minute, 2*time.Second).Should(BeTrue())

		By("verifying that the target Secret was not updated")

		output, err = utils.Run(
			exec.Command(
				"kubectl",
				"get",
				"secret",
				"vault-noop-target",
				"-n",
				"secret-sync-operator-system",
				"-o",
				"json",
			),
		)
		Expect(err).NotTo(HaveOccurred())

		var targetSecretAfter struct {
			Metadata struct {
				ResourceVersion string `json:"resourceVersion"`
			} `json:"metadata"`
			Data map[string]string `json:"data"`
		}

		Expect(json.Unmarshal([]byte(output), &targetSecretAfter)).To(Succeed())

		Expect(targetSecretAfter.Metadata.ResourceVersion).
			To(Equal(initialResourceVersion))

		Expect(targetSecretAfter.Data).
			To(Equal(targetSecretBefore.Data))
	})
})

func cleanupSecretSyncResources() {
	const namespace = "secret-sync-operator-system"

	secretSyncs := []string{
		"vault-secret-sync",
		"vault-update-secret-sync",
		"vault-noop-secret-sync",
	}

	targetSecrets := []string{
		"vault-synced-secret",
		"vault-update-target",
		"vault-noop-target",
	}

	tokenSecrets := []string{
		"vault-token",
		"vault-token-update",
		"vault-token-noop",
	}

	By("cleaning up SecretSync resources")

	for _, name := range secretSyncs {
		_, err := utils.Run(
			exec.Command(
				"kubectl",
				"delete",
				"secretsync",
				name,
				"-n",
				namespace,
				"--ignore-not-found",
			),
		)

		if err != nil {
			_, _ = GinkgoWriter.Write(
				[]byte("warning: failed to delete SecretSync " + name + ": " + err.Error() + "\n"),
			)
		}
	}

	By("cleaning up target Secrets")

	for _, name := range targetSecrets {
		_, err := utils.Run(
			exec.Command(
				"kubectl",
				"delete",
				"secret",
				name,
				"-n",
				namespace,
				"--ignore-not-found",
			),
		)

		if err != nil {
			_, _ = GinkgoWriter.Write(
				[]byte("warning: failed to delete target Secret " + name + ": " + err.Error() + "\n"),
			)
		}
	}

	By("cleaning up Vault token Secrets")

	for _, name := range tokenSecrets {
		_, err := utils.Run(
			exec.Command(
				"kubectl",
				"delete",
				"secret",
				name,
				"-n",
				namespace,
				"--ignore-not-found",
			),
		)

		if err != nil {
			_, _ = GinkgoWriter.Write(
				[]byte("warning: failed to delete Vault token Secret " + name + ": " + err.Error() + "\n"),
			)
		}
	}
}

var _ = Describe("Controller Manager", func() {
	It("should be running", func() {
		By("checking the controller manager deployment")

		Eventually(func() error {
			cmd := exec.Command(
				"kubectl",
				"get",
				"deployment",
				"secret-sync-operator-controller-manager",
				"-n",
				"secret-sync-operator-system",
			)

			_, err := utils.Run(cmd)
			return err
		}, 2*time.Minute, 2*time.Second).Should(Succeed())
	})
})

func serviceAccountToken(namespace string) string {
	output, err := utils.Run(
		exec.Command(
			"kubectl",
			"create",
			"token",
			"default",
			"-n",
			namespace,
		),
	)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(output)
}