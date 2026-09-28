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

package controller

import (
	"bytes"
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	opsv1alpha1 "github.com/ukfungr/secret-sync-operator/api/v1alpha1"
	"github.com/ukfungr/secret-sync-operator/internal/provider"
)

// SecretSyncReconciler reconciles a SecretSync resource.
type SecretSyncReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	ProviderFactory provider.Factory
}

// defaultRefreshInterval sets defulat time to reconcile every 5 minutes
// if the user does not specify refreshInterval in the SecretSync.
const defaultRefreshInterval = 5 * time.Minute

// getRefreshInterval returns the refresh interval configured by the user,
// or the default 5 minutes when no valid interval is configured.
func getRefreshInterval(secretSync *opsv1alpha1.SecretSync) time.Duration {
	if secretSync.Spec.RefreshInterval.Duration <= 0 {
		return defaultRefreshInterval
	}

	return secretSync.Spec.RefreshInterval.Duration
}

// +kubebuilder:rbac:groups=ops.example.com,resources=secretsyncs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ops.example.com,resources=secretsyncs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ops.example.com,resources=secretsyncs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.24.1/pkg/reconcile
func (r *SecretSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	var secretSync opsv1alpha1.SecretSync
	var targetSecret corev1.Secret

	// 1. Get SecretSync resource
	err := r.Get(
		ctx,
		req.NamespacedName,
		&secretSync,
	)

	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// Calculate the effective refresh interval once the
	// SecretSync has been retrieved
	refreshInterval := getRefreshInterval(&secretSync)

	// 2. Get the provider configured in the SecretSync resource.
	secretProvider, err := r.ProviderFactory.GetProvider(
		secretSync.Spec.Provider.Type,
		secretSync.Spec.Provider.Config,
		secretSync.Namespace,
	)

	if err != nil {
		// Update SecretSync status in case it fails getting secret provider
		statusErr := r.setCondition(
			ctx,
			&secretSync,
			metav1.ConditionFalse,
			"SyncFailed",
			err.Error(),
		)
		if statusErr != nil {
			return ctrl.Result{}, statusErr
		}
		return ctrl.Result{}, err
	}

	// 3. Retrieve the remote secret from the provider.
	data, err := secretProvider.GetSecret(
		ctx,
		secretSync.Spec.Remote.Name,
	)

	if err != nil {
		// Update SecretSync status in case it fails getting remote secret from provider
		statusErr := r.setCondition(
			ctx,
			&secretSync,
			metav1.ConditionFalse,
			"SyncFailed",
			err.Error(),
		)
		if statusErr != nil {
			return ctrl.Result{}, statusErr
		}

		return ctrl.Result{}, err
	}

	// 4. Get the target Kubernetes Secret.
	err = r.Get(
		ctx,
		client.ObjectKey{
			Name:      secretSync.Spec.Target.Name,
			Namespace: secretSync.Namespace,
		},
		&targetSecret,
	)

	// 4.a If the target Secret does not exist, create it.
	if err != nil {
		if apierrors.IsNotFound(err) {
			targetSecret = corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretSync.Spec.Target.Name,
					Namespace: secretSync.Namespace,
				},
				Data: data,
			}

			err = r.Create(ctx, &targetSecret)
			if err != nil {
				return ctrl.Result{}, err
			}
			// Update SecretSync status in case it succeeded creating Kubernetes secret
			err := r.setCondition(
				ctx,
				&secretSync,
				metav1.ConditionTrue,
				"SecretSynced",
				"Secret successfully synchronized",
			)
			if err != nil {
				return ctrl.Result{}, err
			}

			// refresh interval after successfully creating the Secret
			return ctrl.Result{
				RequeueAfter: refreshInterval,
			}, nil
		}
		return ctrl.Result{}, err
	}

	// 4.b If the target Secret already contains the same data, no update is needed.
	if secretDataEqual(targetSecret.Data, data) {
		// Update SecretSync status since the secret is up to date
		err = r.setCondition(
			ctx,
			&secretSync,
			metav1.ConditionTrue,
			"SecretSynced",
			"Secret is already synchronized",
		)
		if err != nil {
			return ctrl.Result{}, err
		}

		// Requeue even when the Secret is already up to date.
		// Without this, the controller would stop checking the external
		// provider after this successful reconciliation.
		return ctrl.Result{
			RequeueAfter: refreshInterval,
		}, nil
	}

	// 4.c If the target Secret data differs from the remote secret, update it.
	targetSecret.Data = data

	err = r.Update(ctx, &targetSecret)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Update SecretSync status in case the secret update succeeded
	err = r.setCondition(
		ctx,
		&secretSync,
		metav1.ConditionTrue,
		"SecretSynced",
		"Secret successfully synchronized",
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Requeue after successfully updating the Secret so the
	// controller checks the external provider again after the configured
	// interval.
	return ctrl.Result{
		RequeueAfter: refreshInterval,
	}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opsv1alpha1.SecretSync{}).
		Named("secretsync").
		Complete(r)
}

// secretDataEqual compares the data of the existing Kubernetes Secret
// with the data retrieved from the external provider.
func secretDataEqual(
	existing map[string][]byte,
	remote map[string][]byte,
) bool {
	if len(existing) != len(remote) {
		return false
	}

	for key, remoteValue := range remote {
		existingValue, ok := existing[key]
		if !ok {
			return false
		}

		if !bytes.Equal(existingValue, remoteValue) {
			return false
		}
	}
	return true
}

// setCondition updates the Ready condition of the SecretSync resource.
func (r *SecretSyncReconciler) setCondition(
	ctx context.Context,
	secretSync *opsv1alpha1.SecretSync,
	status metav1.ConditionStatus,
	reason string,
	message string,
) error {

	// Create Kubernetes standard condition
	condition := metav1.Condition{
		Type:               "Ready",
		Status:             status,
		ObservedGeneration: secretSync.Generation,
		Reason:             reason,
		Message:            message,
	}

	// Adds/Replaces kubernetes standard condition
	meta.SetStatusCondition(
		&secretSync.Status.Conditions,
		condition,
	)

	// Writes condition in SecretSync.status
	return r.Status().Update(ctx, secretSync)
}
