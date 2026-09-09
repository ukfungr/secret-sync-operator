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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ukfungr/secret-sync-operator/test/utils"
)

var (
	managerImage             = "example.com/secret-sync-operator:v0.0.1"
	shouldCleanupCertManager = false
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting secret-sync-operator e2e test suite\n")
	RunSpecs(t, "e2e suite")
}

var _ = BeforeSuite(func() {
	By("selecting the Kind cluster context")

	kindCluster := os.Getenv("KIND_CLUSTER")
	if kindCluster == "" {
		kindCluster = "secret-sync-operator-test-e2e"
	}

	kubeContext := fmt.Sprintf("kind-%s", kindCluster)

	_, err := utils.Run(
		exec.Command(
			"kubectl",
			"config",
			"use-context",
			kubeContext,
		),
	)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(),
		"Failed to select Kind cluster context %q", kubeContext)

	By("building the manager image")
	cmd := exec.Command(
		"make",
		"docker-build",
		fmt.Sprintf("IMG=%s", managerImage),
	)
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build the manager image")

	By("loading the manager image on Kind")
	err = utils.LoadImageToKindClusterWithName(managerImage)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load the manager image into Kind")

	By("deploying the manager")
	cmd = exec.Command(
		"make",
		"deploy",
		fmt.Sprintf("IMG=%s", managerImage),
	)
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to deploy the manager")

	By("setting up Vault")
	setupVault()

	By("configuring kubectl kuberc")
	configureKubectlKubeRC()

	setupCertManager()
})

var _ = AfterSuite(func() {
	teardownVault()
	teardownCertManager()
})

// Disable kubectl kuberc by default for test isolation.
func configureKubectlKubeRC() {
	if os.Getenv("KUBECTL_KUBERC") != "true" {
		By("disabling kubectl kuberc for test isolation")
		err := os.Setenv("KUBECTL_KUBERC", "false")
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to disable kubectl kuberc")
		_, _ = fmt.Fprintf(
			GinkgoWriter,
			"kubectl kuberc disabled for consistent test behavior (override with KUBECTL_KUBERC=true)\n",
		)
	} else {
		_, _ = fmt.Fprintf(
			GinkgoWriter,
			"kubectl kuberc enabled (KUBECTL_KUBERC=true)\n",
		)
	}
}

func setupCertManager() {
	if os.Getenv("CERT_MANAGER_INSTALL_SKIP") == "true" {
		_, _ = fmt.Fprintf(
			GinkgoWriter,
			"Skipping CertManager installation (CERT_MANAGER_INSTALL_SKIP=true)\n",
		)
		return
	}

	By("checking if CertManager is already installed")
	if utils.IsCertManagerCRDsInstalled() {
		_, _ = fmt.Fprintf(
			GinkgoWriter,
			"CertManager is already installed. Skipping installation.\n",
		)
		return
	}

	shouldCleanupCertManager = true

	By("installing CertManager")
	Expect(utils.InstallCertManager()).To(
		Succeed(),
		"Failed to install CertManager",
	)
}

func teardownCertManager() {
	if !shouldCleanupCertManager {
		_, _ = fmt.Fprintf(
			GinkgoWriter,
			"Skipping CertManager cleanup (not installed by this suite)\n",
		)
		return
	}

	By("uninstalling CertManager")
	utils.UninstallCertManager()
}
