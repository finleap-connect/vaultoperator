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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	vaultv1alpha1 "github.com/finleap-connect/vaultoperator/api/v1alpha1"
	"github.com/finleap-connect/vaultoperator/vault"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

// +kubebuilder:docs-gen:collapse=Imports

const (
	testNamespace        = "test-namespace"
	testDockerConfigJSON = "{\"auths\":{\"https://index.docker.io/v1/\":{\"auth\":\"\"}}}"
)

var (
	k8sClient client.Client // You'll be using this client in your tests.
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc

	testVaultServer *vault.DevServer
	testVaultClient *vault.Client
	testNameCounter = 0 // Used for predictable test names
	// Instances of reconcilers to test against
	testVSR            *VaultSecretReconciler
	testWithEnterprise bool = false
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	done := make(chan interface{})
	defer close(done)

	logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))

	ctx, cancel = context.WithCancel(context.TODO())

	/*
		First, the envtest cluster is configured to read CRDs from the CRD directory Kubebuilder scaffolds for you.
	*/
	By("bootstrapping test environment")
	Expect(os.Setenv("KUBEBUILDER_ASSETS", "../tools")).To(Succeed())

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	/*
		Then, we start the envtest cluster.
	*/
	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = vaultv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	err = k8sClient.Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace,
		},
	})
	Expect(err).ToNot(HaveOccurred())

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

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&VaultSecretReconciler{
		Client:   k8sClient,
		Log:      logf.Log.WithName("controllers").WithName("VaultSecret"),
		Recorder: &record.FakeRecorder{}, // dummy recorder
		Vault:    testVaultClient,
		Scheme:   scheme.Scheme,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		defer close(done)

		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

	Eventually(done, 60).Should(BeClosed())
})

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
