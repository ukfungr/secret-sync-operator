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
	"fmt"
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ukfungr/secret-sync-operator/test/utils"
)

const (
	vaultNamespace     = "secret-sync-operator-vault"
	vaultDeploymentName = "vault"
	vaultServiceName    = "vault"
	vaultContainerImage = "hashicorp/vault:1.21"

	vaultRootToken = "dev-only-token"
	vaultMount     = "secret"
	vaultSecret    = "test-secret"
)

// setupVault creates a Vault instance inside the Kind cluster.
//
// Vault runs in development mode because this is an isolated E2E
// environment. Vault is deployed in a separate namespace because the
// operator namespace uses the restricted Pod Security standard, while
// Vault requires CAP_SETFCAP during startup.
func setupVault() {
	By("cleaning up any existing Vault namespace")

	cmd := exec.Command(
		"kubectl",
		"delete",
		"namespace",
		vaultNamespace,
		"--ignore-not-found",
	)
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to clean up existing Vault namespace")

	By("waiting for the existing Vault namespace to be deleted")

	Eventually(func(g Gomega) {
		cmd := exec.Command(
			"kubectl",
			"get",
			"namespace",
			vaultNamespace,
			"--ignore-not-found",
			"-o",
			"name",
		)

		output, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(output).To(BeEmpty(), "Vault namespace still exists")
	}, 2*time.Minute, time.Second).Should(Succeed())

	By("creating the Vault namespace")

	cmd = exec.Command(
		"kubectl",
		"create",
		"namespace",
		vaultNamespace,
	)
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to create Vault namespace")

	By("creating the Vault deployment and service")

	manifest := fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
        app: %s
    spec:
      containers:
        - name: vault
          image: %s
          command:
            - /bin/vault
          args:
            - server
            - -dev
          env:
            - name: VAULT_DEV_ROOT_TOKEN_ID
              value: %s
            - name: VAULT_DEV_LISTEN_ADDRESS
              value: "0.0.0.0:8200"
          ports:
            - name: http
              containerPort: 8200
          securityContext:
            runAsNonRoot: true
            runAsUser: 100
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
              add:
                - SETFCAP
            seccompProfile:
              type: RuntimeDefault
---
apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: %s
spec:
  selector:
    app: %s
  ports:
    - name: http
      port: 8200
      targetPort: 8200
`,
		vaultDeploymentName,
		vaultNamespace,
		vaultDeploymentName,
		vaultDeploymentName,
		vaultContainerImage,
		vaultRootToken,
		vaultServiceName,
		vaultNamespace,
		vaultDeploymentName,
	)

	manifestFile := "/tmp/secret-sync-vault-e2e.yaml"

	err = os.WriteFile(
		manifestFile,
		[]byte(manifest),
		0o644,
	)
	Expect(err).NotTo(HaveOccurred(), "Failed to create Vault manifest")

	cmd = exec.Command(
		"kubectl",
		"apply",
		"-f",
		manifestFile,
	)
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to deploy Vault")

	By("waiting for Vault to become available")

	cmd = exec.Command(
		"kubectl",
		"wait",
		"deployment/"+vaultDeploymentName,
		"--for=condition=Available",
		"--timeout=5m",
		"-n",
		vaultNamespace,
	)
	_, err = utils.Run(cmd)

	if err != nil {
		printVaultDiagnostics()
	}

	Expect(err).NotTo(HaveOccurred(), "Vault deployment did not become available")

	By("waiting for the Vault service endpoint")

	Eventually(func(g Gomega) {
		cmd := exec.Command(
			"kubectl",
			"get",
			"endpoints",
			vaultServiceName,
			"-n",
			vaultNamespace,
			"-o",
			"jsonpath={.subsets[0].addresses[0].ip}",
		)

		output, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(output).NotTo(BeEmpty())
	}, 2*time.Minute, time.Second).Should(Succeed())

	By("seeding the Vault test secret")

	seedVaultSecret()
}

// printVaultDiagnostics prints Kubernetes diagnostics when Vault fails
// to become available. This must run before the E2E cleanup removes
// the Vault namespace.
func printVaultDiagnostics() {
	By("collecting Vault deployment diagnostics")

	cmd := exec.Command(
		"kubectl",
		"describe",
		"deployment",
		vaultDeploymentName,
		"-n",
		vaultNamespace,
	)
	if output, err := utils.Run(cmd); err == nil {
		_, _ = fmt.Fprintf(
			GinkgoWriter,
			"\nVault Deployment:\n%s\n",
			output,
		)
	} else {
		_, _ = fmt.Fprintf(
			GinkgoWriter,
			"\nFailed to describe Vault Deployment: %s\n",
			err,
		)
	}

	By("collecting Vault ReplicaSet diagnostics")

	cmd = exec.Command(
		"kubectl",
		"get",
		"replicasets",
		"-n",
		vaultNamespace,
		"-l",
		"app="+vaultDeploymentName,
		"-o",
		"name",
	)
	replicaSetOutput, err := utils.Run(cmd)
	if err == nil && replicaSetOutput != "" {
		cmd = exec.Command(
			"kubectl",
			"describe",
			"-n",
			vaultNamespace,
			replicaSetOutput,
		)

		if output, describeErr := utils.Run(cmd); describeErr == nil {
			_, _ = fmt.Fprintf(
				GinkgoWriter,
				"\nVault ReplicaSet:\n%s\n",
				output,
			)
		} else {
			_, _ = fmt.Fprintf(
				GinkgoWriter,
				"\nFailed to describe Vault ReplicaSet: %s\n",
				describeErr,
			)
		}
	} else {
		_, _ = fmt.Fprintf(
			GinkgoWriter,
			"\nFailed to get Vault ReplicaSet: %s\n",
			err,
		)
	}

	By("collecting Vault Pod diagnostics")

	cmd = exec.Command(
		"kubectl",
		"get",
		"pods",
		"-n",
		vaultNamespace,
		"-l",
		"app="+vaultDeploymentName,
		"-o",
		"name",
	)
	podOutput, err := utils.Run(cmd)

	if err == nil && podOutput != "" {
		_, _ = fmt.Fprintf(
			GinkgoWriter,
			"\nVault Pods:\n%s\n",
			podOutput,
		)

		cmd = exec.Command(
			"kubectl",
			"describe",
			"-n",
			vaultNamespace,
			podOutput,
		)

		if output, describeErr := utils.Run(cmd); describeErr == nil {
			_, _ = fmt.Fprintf(
				GinkgoWriter,
				"\nVault Pod:\n%s\n",
				output,
			)
		} else {
			_, _ = fmt.Fprintf(
				GinkgoWriter,
				"\nFailed to describe Vault Pod: %s\n",
				describeErr,
			)
		}

		cmd = exec.Command(
			"kubectl",
			"logs",
			"-n",
			vaultNamespace,
			podOutput,
			"-c",
			vaultDeploymentName,
			"--previous",
		)

		if output, logsErr := utils.Run(cmd); logsErr == nil {
			_, _ = fmt.Fprintf(
				GinkgoWriter,
				"\nPrevious Vault Container Logs:\n%s\n",
				output,
			)
		} else {
			_, _ = fmt.Fprintf(
				GinkgoWriter,
				"\nFailed to get previous Vault container logs: %s\n",
				logsErr,
			)
		}
	} else {
		_, _ = fmt.Fprintf(
			GinkgoWriter,
			"\nNo Vault Pod found: %s\n",
			err,
		)
	}

	By("collecting Vault namespace events")

	cmd = exec.Command(
		"kubectl",
		"get",
		"events",
		"-n",
		vaultNamespace,
		"--sort-by=.lastTimestamp",
	)
	if output, eventsErr := utils.Run(cmd); eventsErr == nil {
		_, _ = fmt.Fprintf(
			GinkgoWriter,
			"\nVault Namespace Events:\n%s\n",
			output,
		)
	} else {
		_, _ = fmt.Fprintf(
			GinkgoWriter,
			"\nFailed to get Vault namespace events: %s\n",
			eventsErr,
		)
	}
}

// seedVaultSecret writes the test data to Vault using the Vault CLI
// from inside the Vault container.
//
// Using the Vault container itself avoids creating an additional pod
// in the restricted namespace.
func seedVaultSecret() {
	By("getting the Vault pod name")

	var vaultPodName string

	Eventually(func(g Gomega) {
		cmd := exec.Command(
			"kubectl",
			"get",
			"pods",
			"-n",
			vaultNamespace,
			"-l",
			"app="+vaultDeploymentName,
			"-o",
			"jsonpath={.items[0].metadata.name}",
		)

		output, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(output).NotTo(BeEmpty())

		vaultPodName = output
	}, 2*time.Minute, time.Second).Should(Succeed())

	By("seeding the Vault test secret")

	Eventually(func(g Gomega) {
		cmd := exec.Command(
			"kubectl",
			"exec",
			vaultPodName,
			"-n",
			vaultNamespace,
			"-c",
			vaultDeploymentName,
			"--",
			"sh",
			"-c",
			fmt.Sprintf(
				"VAULT_ADDR=http://127.0.0.1:8200 "+
					"VAULT_TOKEN=%s "+
					"vault kv put %s/%s username=admin password=new-password",
				vaultRootToken,
				vaultMount,
				vaultSecret,
			),
		)

		_, err := utils.Run(cmd)
		if err != nil {
			_, _ = fmt.Fprintf(
				GinkgoWriter,
				"\nVault exec failed: %s\n",
				err,
			)

			logCmd := exec.Command(
				"kubectl",
				"logs",
				vaultPodName,
				"-n",
				vaultNamespace,
				"-c",
				vaultDeploymentName,
				"--previous",
			)

			if output, logErr := utils.Run(logCmd); logErr == nil {
				_, _ = fmt.Fprintf(
					GinkgoWriter,
					"\nVault previous container logs:\n%s\n",
					output,
				)
			} else {
				_, _ = fmt.Fprintf(
					GinkgoWriter,
					"\nFailed to get Vault previous container logs: %s\n",
					logErr,
				)
			}
		}

		g.Expect(err).NotTo(HaveOccurred())
	}, 2*time.Minute, time.Second).Should(Succeed())
}

// teardownVault removes the Vault namespace and everything inside it.
func teardownVault() {
	By("removing the Vault namespace")

	cmd := exec.Command(
		"kubectl",
		"delete",
		"namespace",
		vaultNamespace,
		"--ignore-not-found",
	)
	_, _ = utils.Run(cmd)
}