/*

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

package controllers

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	vaultv1alpha1 "github.com/finleap-connect/vaultoperator/api/v1alpha1"
	"github.com/finleap-connect/vaultoperator/vault"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const (
	testNamespace        = "test-namespace"
	testDockerConfigJSON = "{\"auths\":{\"https://index.docker.io/v1/\":{\"auth\":\"\"}}}"
)

var (
	testConfig      *rest.Config
	testClient      client.Client
	testEnv         *envtest.Environment
	testVaultServer *vault.DevServer
	testVaultClient *vault.Client
	testNameCounter = 0 // Used for predictable test names
	// Instances of reconcilers to test against
	testVSR            *VaultSecretReconciler
	testWithEnterprise bool = false
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	var err error
	testConfig, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(testConfig).ToNot(BeNil())

	err = vaultv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	testClient, err = client.New(testConfig, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(testClient).ToNot(BeNil())

	testVaultServer, err = vault.NewDevServer() // via `vault server -dev`
	Expect(err).ToNot(HaveOccurred())
	testVaultClient, err = testVaultServer.GetClient("")
	Expect(err).ToNot(HaveOccurred())

	health, err := testVaultClient.Sys().Health()
	Expect(err).ToNot(HaveOccurred())
	testWithEnterprise = strings.Contains(health.Version, "+prem")
	namespace := "" // if no enterprise test Vault server is present just test on root namespace
	if testWithEnterprise {
		namespace = "testnamespace"
		// First enable a secret engine in the root namespace and create a secret
		// which the VaultOperator will not be able to access using its namespaced client
		Expect(testVaultServer.ExecCommand("secrets", "enable", "-version=2", "-path=app", "kv")).To(Succeed())
		Expect(testVaultServer.ExecCommand("kv", "put", "app/test/only", "for=root")).To(Succeed())

		// Create test namespace and namespaced client
		Expect(testVaultServer.ExecCommand("namespace", "create", namespace)).To(Succeed())

		testVaultClient, err = testVaultServer.GetClient(namespace)
		Expect(err).ToNot(HaveOccurred())
	}
	// Create test Vaults in either the root or the dedicated test namespace
	Expect(testVaultServer.ExecCommand("secrets", "enable", "-namespace", namespace, "-version=2", "-path=app", "kv")).To(Succeed())
	Expect(testVaultServer.ExecCommand("secrets", "enable", "-namespace", namespace, "-version=2", "-path=cert", "kv")).To(Succeed())
	Expect(testVaultServer.ExecCommand("kv", "put", "-namespace", namespace, "app/test/bar", "baz=buzzfizz")).To(Succeed())
	Expect(testVaultServer.ExecCommand("kv", "put", "-namespace", namespace, "app/test/bar", "baz=fizzbuzz", "bax=fixxbaxx")).To(Succeed())
	Expect(testVaultServer.ExecCommand("kv", "put", "-namespace", namespace, "app/test/foo", "foo=bar", "baz=foo")).To(Succeed())
	Expect(testVaultServer.ExecCommand("kv", "put", "-namespace", namespace, "app/test/binbar", "baz=Zml6emJ1enpi", ".baz_isBinary=1")).To(Succeed())
	Expect(testVaultServer.ExecCommand("kv", "put", "-namespace", namespace, "app/test/docker", "baz="+testDockerConfigJSON)).To(Succeed())
	Expect(testVaultServer.ExecCommand("kv", "get", "-namespace", namespace, "-version=1", "app/test/bar")).To(Succeed())

	testVSR = &VaultSecretReconciler{
		Client:   testClient,
		Log:      logf.Log.WithName("controllers").WithName("VaultSecret"),
		Recorder: &record.FakeRecorder{}, // dummy recorder
		Vault:    testVaultClient,
		Scheme:   scheme.Scheme,
	}

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	if testEnv != nil {
		err := testEnv.Stop()
		Expect(err).ToNot(HaveOccurred())
	}
	if testVaultServer != nil {
		err := testVaultServer.Stop()
		Expect(err).ToNot(HaveOccurred())
	}
	if testVaultClient != nil {
		testVaultClient.Close()
	}
})

// Helper functions

func newTestName() string {
	testNameCounter += 1
	return fmt.Sprintf("test%d", testNameCounter)
}
