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

package main

import (
	"errors"
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	vaultv1alpha1 "github.com/finleap-connect/vaultoperator/api/v1alpha1"
	"github.com/finleap-connect/vaultoperator/controllers"
	"github.com/finleap-connect/vaultoperator/vault"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = vaultv1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var (
		vaultAddr            string
		vaultRoleID          string
		vaultSecretID        string
		vaultToken           string
		metricsAddr          string
		enableLeaderElection bool
		vaultNamespace       string
	)
	// TODO: which vaults to manage should be configured via CRD, e.g. similar to "storageclass"
	flag.StringVar(&vaultAddr, "vault-addr", "", "The address the vault client will connect to.")
	flag.StringVar(&vaultRoleID, "vault-role-id", "", "AppRole RoleID used to connect to vault.")
	flag.StringVar(&vaultSecretID, "vault-secret-id", "", "AppRole SecretID used to connect to vault.")
	flag.StringVar(&vaultToken, "vault-token", "", "If no AppRole should be used, a token can be provided.")
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&vaultNamespace, "vault-namespace", "", "The Vault namespace the operator works with.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	// Check if vault credentials were provided via env variables and make sure
	// mandatory variables are provided
	if vaultAddr == "" {
		vaultAddr = os.Getenv("VAULT_ADDR")
	}
	if vaultRoleID == "" {
		vaultRoleID = os.Getenv("VAULT_ROLE_ID")
	}
	if vaultSecretID == "" {
		vaultSecretID = os.Getenv("VAULT_SECRET_ID")
	}
	if vaultToken == "" {
		vaultToken = os.Getenv("VAULT_TOKEN")
	}
	if vaultNamespace == "" {
		vaultNamespace = os.Getenv("VAULT_NAMESPACE")
	}
	if vaultAddr == "" {
		setupLog.Error(errors.New("vault configuration incomplete"), "vault addr missing")
		os.Exit(1)
	}
	var authMethod vault.AuthMethod
	if vaultToken != "" {
		authMethod = &vault.TokenAuth{
			Token: vaultToken,
		}
	} else if vaultRoleID != "" && vaultSecretID != "" {
		authMethod = &vault.AppRoleAuth{
			RoleID:   vaultRoleID,
			SecretID: vaultSecretID,
		}
	} else {
		setupLog.Error(errors.New("no valid configuration for authentication provided"), "token or approle missing")
		os.Exit(2)
	}
	vc, err := vault.NewClient(vaultAddr, vaultNamespace, authMethod)
	if err != nil {
		setupLog.Error(err, "unable to create vault client")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "43aa3c97.finleap.cloud",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.VaultSecretReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("VaultSecret"),
		Scheme: mgr.GetScheme(),
		Vault:  vc,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "VaultSecret")
		os.Exit(1)
	}
	if err = (&vaultv1alpha1.VaultSecret{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "VaultSecret")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
