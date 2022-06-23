// Copyright 2022 VaultOperator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var vaultsecretlog = logf.Log.WithName("webhook.vaultsecret")

func (r *VaultSecret) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/validate-vault-finleap-cloud-v1alpha1-vaultsecret,mutating=false,failurePolicy=fail,groups=vault.finleap.cloud,resources=vaultsecrets,verbs=create;update,versions=v1alpha1,name=vvaultsecret.kb.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Validator = &VaultSecret{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *VaultSecret) ValidateCreate() error {
	vaultsecretlog.Info("validating create of vaultSecret", "name", r.Name, "namespace", r.Namespace)

	if (r.Spec.Data == nil || len(r.Spec.Data) == 0) && (r.Spec.DataFrom == nil || len(r.Spec.DataFrom) == 0) {
		return errors.New("One of spec.data or spec.dataFrom is mandatory")
	}

	if r.Spec.Data != nil || len(r.Spec.Data) > 0 {
		for _, data := range r.Spec.Data {
			if data.Name == "" {
				return errors.New("spec.data[].name can not be empty")
			}
			if data.Location != nil {
				if data.Variables != nil {
					return errors.New("spec.data[].location conflicting with spec.data[].variables")
				}
				if data.Template != "" {
					return errors.New("spec.data[].location conflicting with spec.data[].template")
				}
				if data.Location.Path == "" || data.Location.Field == "" {
					return errors.New("spec.data[].location.path and spec.data[].location.field are required")
				}
				if data.Generator != nil {
					if data.Generator.Name == "" {
						return errors.New("spec.data[].generator.name is required if generator is used")
					}
					if data.Location.Version > 0 {
						return errors.New("spec.data[].location.version is not allowed when specifying spec.data[].generator")
					}
				}
			} else {
				if data.Template == "" {
					return errors.New("spec.data[].template is required if spec.data[].location not provided")
				}
				for _, variable := range data.Variables {
					if variable.Name == "" || variable.Location == nil {
						return errors.New("spec.data[].variables[] requires both name and location")
					}
					if variable.Location.Path == "" || variable.Location.Field == "" {
						return errors.New("spec.data[].variable[].location.path and spec.data[].variable[].location.field are required")
					}
					if variable.Generator != nil {
						if variable.Generator.Name == "" {
							return errors.New("spec.data[].variable[].generator.name is required if generator is used")
						}
						if variable.Location.Version > 0 {
							return errors.New("spec.data[].variable[].location.version is not allowed when specifying spec.data[].variable[].generator")
						}
					}
				}
			}
		}
	}

	if r.Spec.DataFrom == nil || len(r.Spec.DataFrom) == 0 {
		for _, data := range r.Spec.DataFrom {
			if data.Path == "" {
				return errors.New("spec.dataFrom[].path is required")
			}
		}
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *VaultSecret) ValidateUpdate(old runtime.Object) error {
	vaultsecretlog.Info("validating update of vaultSecret", "name", r.Name, "namespace", r.Namespace)
	return r.ValidateCreate()
}

func (r *VaultSecret) ValidateDelete() error {
	return nil
}
