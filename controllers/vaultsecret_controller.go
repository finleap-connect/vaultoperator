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

package controllers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ref "k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	vaultv1alpha1 "github.com/finleap-connect/vaultoperator/api/v1alpha1"
	"github.com/finleap-connect/vaultoperator/util"
	"github.com/finleap-connect/vaultoperator/vault"

	b64 "encoding/base64"
)

const (
	finalizerName = "vault.finleap.cloud"
)

// VaultSecretReconciler reconciles a VaultSecret object
type VaultSecretReconciler struct {
	client.Client
	Log      logr.Logger
	Recorder record.EventRecorder
	Vault    *vault.Client
	Scheme   *runtime.Scheme
}

// +kubebuilder:rbac:groups=vault.finleap.cloud,resources=vaultsecrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vault.finleap.cloud,resources=vaultsecrets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
func (r *VaultSecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("vaultsecret", req.NamespacedName)
	log.Info("Started reconciliation...")

	// Fetch the vault secret object
	vaultSecret := &vaultv1alpha1.VaultSecret{}
	if err := r.Get(ctx, req.NamespacedName, vaultSecret); err != nil {
		log.Error(err, "unable to fetch vaultSecret")
		// We'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, ignoreNotFound(err)
	}

	// Check whether object is being deleted
	if deleted, err := r.handleDeletion(ctx, log, vaultSecret); deleted || err != nil {
		return ctrl.Result{}, err
	}

	// Validate VaultSecret
	if err := r.handleValidation(ctx, log, vaultSecret); err != nil {
		return ctrl.Result{}, err
	}

	// Use same request if not overridden
	secretReq := req.NamespacedName
	if vaultSecret.Spec.SecretName != "" {
		secretReq = types.NamespacedName{
			Namespace: req.Namespace,
			Name:      vaultSecret.Spec.SecretName,
		}
	}

	// VaultSecret was either created or updated, create or update secret accordingly
	if err := r.handleCreateOrUpdate(ctx, log, vaultSecret, secretReq); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *VaultSecretReconciler) handleCreateOrUpdate(ctx context.Context, log logr.Logger, vaultSecret *vaultv1alpha1.VaultSecret, n types.NamespacedName) error {
	secret := corev1.Secret{}
	status := vaultSecret.Status

	// VaultSecret has a secret ref, get the secret
	if status.SecretObject != nil {
		if err := r.Get(ctx, n, &secret); err != nil {
			if ignoreNotFound(err) != nil {
				r.Recorder.Event(vaultSecret, corev1.EventTypeWarning, "Problem", fmt.Sprintf("Checking owned secret failed with: %v", err))
				return err
			} else if err != nil {
				// Not found so let's reset the reference and let's re-create it
				status.SecretObject = nil
			}
		}
	}

	if status.SecretObject == nil { // Checking here as above control flow can reset secret
		secret.ObjectMeta.Name = n.Name
		secret.ObjectMeta.Namespace = n.Namespace
	}

	err := controllerutil.SetControllerReference(vaultSecret, &secret, r.Scheme)
	if err != nil {
		return err
	}

	r.Recorder.Event(vaultSecret, corev1.EventTypeNormal, "Info", "Building required state of secret")
	// TODO: we should check spec against status to check if update is necessary
	if err := r.updateSecret(&secret, vaultSecret); err != nil {
		log.Error(err, "failed to update secret")
		r.Recorder.Event(vaultSecret, corev1.EventTypeWarning, "Problem", fmt.Sprintf("Failed to update secret: %v", err))
		return err // TODO: maybe we should wrap returned errors
	}

	// Update or create the secret with the up-to-date data
	if status.SecretObject != nil {
		r.Recorder.Event(vaultSecret, corev1.EventTypeNormal, "Info", "Updating secret")
		if err := r.Update(ctx, &secret); err != nil {
			log.Error(err, "failed to create or update secret")
			r.Recorder.Event(vaultSecret, corev1.EventTypeWarning, "Problem", fmt.Sprintf("updating secret failed with: %v", err))
			return err
		}
	} else {
		r.Recorder.Event(vaultSecret, corev1.EventTypeNormal, "Info", "Creating secret")
		if err := r.Create(ctx, &secret); err != nil {
			log.Error(err, "failed to create secret")
			r.Recorder.Event(vaultSecret, corev1.EventTypeWarning, "Problem", fmt.Sprintf("creating secret failed with: %v", err))
			return err
		}
	}

	r.Recorder.Event(vaultSecret, corev1.EventTypeNormal, "Info", "Updating vaultSecret")
	// Save the reference to make sure secret is cleaned up later as well
	secretRef, err := ref.GetReference(r.Scheme, &secret)
	if err != nil {
		log.Error(err, "unable to get reference")
		r.Recorder.Event(vaultSecret, corev1.EventTypeWarning, "Problem", "Failed fetching reference to related secret")
		return err
	}
	vaultSecret.Status.SecretObject = secretRef
	if err := r.Update(ctx, vaultSecret); err != nil {
		log.Error(err, "status update failed")
		r.Recorder.Event(vaultSecret, corev1.EventTypeWarning, "Problem", "Failed to update vaultSecret")
		return err
	}
	return nil
}

func (r *VaultSecretReconciler) handleValidation(ctx context.Context, log logr.Logger, vaultSecret *vaultv1alpha1.VaultSecret) error {
	if err := vaultSecret.ValidateCreate(); err != nil {
		log.Error(err, "validation failed")
		r.Recorder.Event(vaultSecret, corev1.EventTypeWarning, "Invalid", fmt.Sprintf("Validation failed with error: %v", err))
		return nil // Do not retry, new event will be cause by update
	}
	r.Recorder.Event(vaultSecret, corev1.EventTypeNormal, "Info", "Validation successful")
	return nil
}

// handleDeletion checks if vaultSecret has the finalizer, if it has been deleted and if so cleans up related resources.
// Errors are returned and if the deletion was successful.
func (r *VaultSecretReconciler) handleDeletion(ctx context.Context, log logr.Logger, vaultSecret *vaultv1alpha1.VaultSecret) (bool, error) {
	if vaultSecret.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !containsString(vaultSecret.ObjectMeta.Finalizers, finalizerName) {
			vaultSecret.ObjectMeta.Finalizers = append(vaultSecret.ObjectMeta.Finalizers, finalizerName)
			if err := r.Update(context.Background(), vaultSecret); err != nil {
				return false, err
			}
			log.Info("finalizer added")
			r.Recorder.Event(vaultSecret, corev1.EventTypeNormal, "Updated", "Added finalizer to vaultSecret")
		}
	} else { // The object is being deleted
		log.Info("deletion in progress")
		r.Recorder.Event(vaultSecret, corev1.EventTypeNormal, "Info", "Deletion in progress")
		if containsString(vaultSecret.ObjectMeta.Finalizers, finalizerName) {
			// Our finalizer is present, so lets handle any external dependency
			if err := r.deleteExternalResources(ctx, log, vaultSecret); err != nil {
				log.Error(err, "external resources failed to clean up")
				r.Recorder.Event(vaultSecret, corev1.EventTypeWarning, "Problem", "Failed cleanup up external resources")
				// Failed, but continue exection for namespace deletion for example
			}

			// Remove our finalizer from the list and update it.
			vaultSecret.ObjectMeta.Finalizers = removeString(vaultSecret.ObjectMeta.Finalizers, finalizerName)
			if err := r.Update(context.Background(), vaultSecret); err != nil {
				log.Error(err, "removing finalizer failed")
				r.Recorder.Event(vaultSecret, corev1.EventTypeWarning, "Problem", "Removing finalizer failed")
				return false, err
			}

			log.Info("finalizer removed")

			return true, nil
		}
	}
	return false, nil
}

func (r *VaultSecretReconciler) deleteExternalResources(ctx context.Context, log logr.Logger, vaultSecret *vaultv1alpha1.VaultSecret) error {
	status := vaultSecret.Status
	if status.SecretObject != nil {
		if err := r.Delete(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: status.SecretObject.Namespace,
				Name:      status.SecretObject.Name,
			},
		}); client.IgnoreNotFound(err) != nil {
			log.Error(err, "failed to remove owned secret")
			r.Recorder.Event(vaultSecret, corev1.EventTypeWarning, "Problem", "Failed to remove owned secret")
			return err
		} else {
			status.SecretObject = nil
		}
	}
	return nil
}

func (r *VaultSecretReconciler) updateSecret(secret *corev1.Secret, vaultSecret *vaultv1alpha1.VaultSecret) error {
	switch {
	case vaultSecret.Spec.SecretType != "":
		secret.Type = vaultSecret.Spec.SecretType
	// Check if it is a pull secret, if so set type
	case len(vaultSecret.Spec.Data) == 1 && vaultSecret.Spec.Data[0].Name == corev1.DockerConfigJsonKey:
		secret.Type = corev1.SecretTypeDockerConfigJson
	}

	// Update secret data
	if vaultSecret.Spec.Data != nil && len(vaultSecret.Spec.Data) > 0 {
		for _, data := range vaultSecret.Spec.Data {
			var value string
			if data.Location != nil { // Location was provided
				var err error
				value, err = r.getVaultSecretData(vaultSecret, &data)
				if err != nil {
					return fmt.Errorf("get vault secret data from %s/%s failed with: %w", data.Location.Path, data.Location.Field, err)
				}
			} else if data.Template != "" {
				// Gather all variable values
				variables := map[string]string{}
				for _, variable := range data.Variables {
					variableValue, err := r.getVaultSecretData(vaultSecret, &variable)
					if err != nil {
						return fmt.Errorf("get vault secret data from %s/%s failed with: %w", variable.Location.Path, variable.Location.Field, err)
					}
					variables[variable.Name] = variableValue
				}
				// Run templating
				tmpl, err := template.New("template").Funcs(sprig.TxtFuncMap()).Parse(data.Template)
				if err != nil {
					return fmt.Errorf("template parsing failed with: %w", err)
				}
				var output bytes.Buffer
				if err := tmpl.Execute(&output, variables); err != nil {
					return fmt.Errorf("template execute failed with: %w", err)
				}
				value = output.String()
			} else {
				return errors.New("vaultsecret malformed either location or template+variables required")
			}
			if secret.Data == nil {
				secret.Data = map[string][]byte{}
			}
			secret.Data[data.Name] = []byte(value)
		}
	}

	if vaultSecret.Spec.DataFrom != nil && len(vaultSecret.Spec.DataFrom) > 0 {
		otherVaultData := make(map[string]bool)

		for _, data := range vaultSecret.Spec.DataFrom {
			if pairs, err := r.getVaultSecretDataFrom(vaultSecret, &data); err != nil {
				return fmt.Errorf("get vault secret data from %s failed with: %w", data.Path, err)
			} else {
				if secret.Data == nil {
					secret.Data = map[string][]byte{}
				}
				for k, v := range pairs {
					_, keyExists := otherVaultData[k]

					if keyExists {
						r.Recorder.Event(vaultSecret, corev1.EventTypeWarning, "Collision", fmt.Sprintf("Vault secret collision detected, strategy set to '%v'; colliding key is '%v'", data.GetCollisionStrategy(), k))
						if data.GetCollisionStrategy() == vaultv1alpha1.ErrorOnCollision {
							return fmt.Errorf("vaultSecret collision detected, strategy set to '%v'", data.GetCollisionStrategy())
						}
						if data.GetCollisionStrategy() == vaultv1alpha1.IgnoreCollision {
							continue
						}
					}
					secret.Data[k] = []byte(v)
					otherVaultData[k] = true
				}
			}
		}
	}

	return nil
}

func (r *VaultSecretReconciler) getVaultSecretDataFrom(vaultSecret *vaultv1alpha1.VaultSecret, data vaultv1alpha1.AnyVaultSecretData) (map[string]string, error) {
	if data.GetLocation() == nil {
		return nil, errors.New("location missing")
	}
	location := data.GetLocation()
	path := strings.Trim(location.Path, "/")
	if err := r.checkPermission(vaultSecret, path); err != nil {
		return nil, err
	}

	if fields, err := r.Vault.GetAll(path, location.Version); err == vault.ErrNotFound && data.GetGenerator() != nil {
		return nil, fmt.Errorf("generation of secret value failed with: %w", err)
	} else {
		resultingSecrets := make(map[string]string)
		for key, value := range fields {
			if fields[vault.GetIsBinaryKey(key)] == "1" {
				byteVal, err := b64.StdEncoding.DecodeString(value)
				if err != nil {
					return nil, err
				}
				resultingSecrets[key] = string(byteVal)
			} else if !strings.HasPrefix(key, ".") {
				resultingSecrets[key] = value
			}
		}

		return fields, err
	}
}

func (r *VaultSecretReconciler) getVaultSecretData(vaultSecret *vaultv1alpha1.VaultSecret, data vaultv1alpha1.AnyVaultSecretData) (string, error) {
	if data.GetLocation() == nil {
		return "", errors.New("location missing")
	}
	location := data.GetLocation()
	path := strings.Trim(location.Path, "/")
	err := r.checkPermission(vaultSecret, path)
	if err != nil {
		return "", err
	}
	value, err := r.Vault.Get(path, location.Field, location.Version)
	var isBinary bool
	if err == vault.ErrNotFound && data.GetGenerator() != nil {
		value, isBinary, err = r.generateValue(data.GetGenerator())
		if err != nil {
			return "", fmt.Errorf("generation of secret value failed with: %w", err)
		}
		location.IsBinary = isBinary
		fields := map[string]interface{}{
			location.Field: value,
		}
		if location.IsBinary {
			fields[vault.GetIsBinaryKey(location.Field)] = "1"
		}
		err = r.Vault.CreateOrUpdate(path, fields)
	}
	if err != nil {
		return "", err
	}
	if location.IsBinary {
		byteVal, err := b64.StdEncoding.DecodeString(value)
		if err != nil {
			return "", err
		}
		value = string(byteVal)
	}
	return value, nil
}

func (r *VaultSecretReconciler) generateValue(gen *vaultv1alpha1.VaultSecretGenerator) (v string, isBinary bool, e error) {
	if gen.Name != "uuid" && len(gen.Args) < 1 {
		e = ErrInvalidGeneratorArgs
		return
	}

	switch gen.Name { // TODO: use constants and use enum in
	case "password":
		n, digits, symbols := int(gen.Args[0]), 0, 0
		if len(gen.Args) > 1 {
			digits = int(gen.Args[1])
		}
		if len(gen.Args) > 2 {
			symbols = int(gen.Args[2])
		}
		var err error
		v, err = util.RandPassword(n, digits, symbols)
		if err != nil {
			e = fmt.Errorf("failed to generate password: %w", err)
			return
		}
	case "string":
		v = util.RandString(int(gen.Args[0]))
	case "bytes":
		v = b64.StdEncoding.EncodeToString(util.RandBytes(int(gen.Args[0])))
		isBinary = true
	case "uuid":
		v = uuid.New().String()
	case "rsa":
		rsaBytes, err := util.GenerateRSA(gen.Args[0])
		if err != nil {
			e = fmt.Errorf("failed to generate rsa: %w", err)
			return
		}
		v = b64.StdEncoding.EncodeToString(rsaBytes)
		isBinary = true
	case "ecdsa":
		ecdsaBytes, err := util.GenerateECDSA(gen.Args[0])
		if err != nil {
			e = fmt.Errorf("failed to generate ecdsa: %w", err)
			return
		}
		v = b64.StdEncoding.EncodeToString(ecdsaBytes)
		isBinary = true
	default:
		e = ErrUnknownGenerator
	}
	return
}

func (r *VaultSecretReconciler) checkPermission(vaultSecret *vaultv1alpha1.VaultSecret, vaultPath string) error {
	// TODO: we should implement CRDs to controll permissions to vault secrets! SUPER IMPORTANT TO REMOVE THIS MADNESS!
	segments := strings.Split(vaultPath, "/")
	if len(segments) < 1 {
		return ErrInvalidVaultPath
	}

	firstSegment := segments[0]
	if firstSegment == "cert" {
		return nil
	}
	if firstSegment == "app" {
		// The Vault path should be scoped (e.g. app/<namespace>/<key-name>) and thus consist
		// of at least 3 parts.
		if len(segments) < 3 {
			return ErrInvalidVaultPath
		}

		// The scope-part should either match the namespace the VaultSecret is deployed to or
		// some pre-defined shared identifiers.
		secondSegment := segments[1]
		sharedPaths := strings.Split(os.Getenv("SHARED_PATHS"), ",")
		switch {
		case secondSegment == vaultSecret.ObjectMeta.Namespace: // namespace of the VaultSecret itself
			return nil
		case util.ContainsString(sharedPaths, secondSegment):
			return nil
		}
		r.Log.Error(ErrPermissionDenied, "second segment must be equal to VaultSecret namespace or in shared paths", "secondSegment", secondSegment, "namespace", vaultSecret.ObjectMeta.Namespace, "sharedPaths", os.Getenv("SHARED_PATHS"))
	}
	return ErrPermissionDenied
}

func (r *VaultSecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("vaultsecret-controller")
	r.Scheme = mgr.GetScheme()
	return ctrl.NewControllerManagedBy(mgr).
		For(&vaultv1alpha1.VaultSecret{}).
		Owns(&corev1.Secret{}).
		Named("vaultoperator").
		Complete(r)
}
