/*
Copyright 2025.

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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	vault "github.com/hashicorp/vault/api"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	secretsv1alpha1 "github.com/Amogha-rao/secret-rotator-operator/api/v1alpha1"
)

// +kubebuilder:rbac:groups=secrets.github.com,resources=secretrotations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=secrets.github.com,resources=secretrotations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=secrets.github.com,resources=secretrotations/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SecretRotation object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
type SecretRotationReconciler struct {
	client.Client
	Log    logr.Logger
	Vault  *vault.Client
	Scheme *runtime.Scheme
}

func (r *SecretRotationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("secretrotation", req.NamespacedName)

	// Fetch the SecretRotation instance
	var sr secretsv1alpha1.SecretRotation
	if err := r.Get(ctx, req.NamespacedName, &sr); err != nil {
		if kerrors.IsNotFound(err) {
			// Resource deleted
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch secret data from Vault
	secret, err := r.Vault.Logical().Read(sr.Spec.VaultPath)
	if err != nil {
		log.Error(err, "failed to read from Vault", "path", sr.Spec.VaultPath)
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}
	if secret == nil || secret.Data == nil {
		log.Info("Vault secret not found or empty", "path", sr.Spec.VaultPath)
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	// Vault KV v2 stores actual data under "data" key
	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		data = secret.Data // fallback if not KV v2
	}

	// Convert vault data to k8s secret data (map[string][]byte)
	secretData := make(map[string][]byte)
	for k, v := range data {
		strVal := fmt.Sprintf("%v", v)
		secretData[k] = []byte(strVal)
	}

	// Calculate checksum of secret data
	newChecksum := r.calculateSecretChecksum(secretData)
	secretChanged := sr.Status.SecretChecksum != newChecksum

	// Prepare Kubernetes Secret object
	k8sSecret := &corev1.Secret{}
	err = r.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: sr.Spec.TargetSecret}, k8sSecret)
	if err != nil && !kerrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if kerrors.IsNotFound(err) {
		// Create Secret if not found
		k8sSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sr.Spec.TargetSecret,
				Namespace: req.Namespace,
			},
			Data: secretData,
		}
		if err := r.Create(ctx, k8sSecret); err != nil {
			log.Error(err, "failed to create Kubernetes Secret")
			return ctrl.Result{}, err
		}
		log.Info("Created Kubernetes Secret", "secret", sr.Spec.TargetSecret)
		secretChanged = true
	} else {
		// Update existing Secret if data differs
		needUpdate := false
		if len(k8sSecret.Data) != len(secretData) {
			needUpdate = true
		} else {
			for k, v := range secretData {
				if string(k8sSecret.Data[k]) != string(v) {
					needUpdate = true
					break
				}
			}
		}
		if needUpdate {
			k8sSecret.Data = secretData
			if err := r.Update(ctx, k8sSecret); err != nil {
				log.Error(err, "failed to update Kubernetes Secret")
				return ctrl.Result{}, err
			}
			log.Info("Updated Kubernetes Secret", "secret", sr.Spec.TargetSecret)
			secretChanged = true
		} else {
			log.Info("Kubernetes Secret already up-to-date", "secret", sr.Spec.TargetSecret)
		}
	}

	// Update target workloads if secret changed
	var updatedWorkloads []string
	if secretChanged && len(sr.Spec.TargetWorkloads) > 0 {
		log.Info("Secret changed, updating target workloads", "checksum", newChecksum)
		
		annotationPrefix := sr.Spec.AnnotationPrefix
		if annotationPrefix == "" {
			annotationPrefix = "secrets.github.com/"
		}
		
		for _, workload := range sr.Spec.TargetWorkloads {
			err := r.updateWorkloadAnnotation(ctx, log, workload, sr.Namespace, annotationPrefix, newChecksum)
			if err != nil {
				log.Error(err, "failed to update workload", "kind", workload.Kind, "name", workload.Name)
				continue
			}
			workloadKey := fmt.Sprintf("%s/%s", workload.Kind, workload.Name)
			if workload.Namespace != "" && workload.Namespace != sr.Namespace {
				workloadKey = fmt.Sprintf("%s/%s/%s", workload.Namespace, workload.Kind, workload.Name)
			}
			updatedWorkloads = append(updatedWorkloads, workloadKey)
			log.Info("Updated workload annotation", "kind", workload.Kind, "name", workload.Name, "checksum", newChecksum)
		}
	}

	// Update status with last rotation time and checksum
	sr.Status.LastRotation = metav1.Now()
	sr.Status.SecretChecksum = newChecksum
	if len(updatedWorkloads) > 0 {
		sr.Status.UpdatedWorkloads = updatedWorkloads
	}
	if err := r.Status().Update(ctx, &sr); err != nil {
		log.Error(err, "failed to update SecretRotation status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil // rotate every 10 minutes or adjust as needed
}

// calculateSecretChecksum calculates a SHA256 checksum of the secret data
func (r *SecretRotationReconciler) calculateSecretChecksum(data map[string][]byte) string {
	hash := sha256.New()
	
	// Sort keys to ensure consistent hashing
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	
	// Hash key-value pairs in sorted order
	for _, k := range keys {
		hash.Write([]byte(k))
		hash.Write(data[k])
	}
	
	return hex.EncodeToString(hash.Sum(nil))[:16] // Use first 16 characters for brevity
}

// updateWorkloadAnnotation updates the specified workload with a checksum annotation
func (r *SecretRotationReconciler) updateWorkloadAnnotation(ctx context.Context, log logr.Logger, workload secretsv1alpha1.WorkloadReference, defaultNamespace, annotationPrefix, checksum string) error {
	namespace := workload.Namespace
	if namespace == "" {
		namespace = defaultNamespace
	}
	
	annotationKey := annotationPrefix + "secret-checksum"
	
	switch strings.ToLower(workload.Kind) {
	case "deployment":
		return r.updateDeploymentAnnotation(ctx, namespace, workload.Name, annotationKey, checksum)
	case "statefulset":
		return r.updateStatefulSetAnnotation(ctx, namespace, workload.Name, annotationKey, checksum)
	case "daemonset":
		return r.updateDaemonSetAnnotation(ctx, namespace, workload.Name, annotationKey, checksum)
	case "replicaset":
		return r.updateReplicaSetAnnotation(ctx, namespace, workload.Name, annotationKey, checksum)
	default:
		return fmt.Errorf("unsupported workload kind: %s", workload.Kind)
	}
}

func (r *SecretRotationReconciler) updateDeploymentAnnotation(ctx context.Context, namespace, name, annotationKey, checksum string) error {
	deployment := &appsv1.Deployment{}
	key := types.NamespacedName{Namespace: namespace, Name: name}
	
	if err := r.Get(ctx, key, deployment); err != nil {
		return err
	}
	
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations[annotationKey] = checksum
	
	return r.Update(ctx, deployment)
}

func (r *SecretRotationReconciler) updateStatefulSetAnnotation(ctx context.Context, namespace, name, annotationKey, checksum string) error {
	statefulSet := &appsv1.StatefulSet{}
	key := types.NamespacedName{Namespace: namespace, Name: name}
	
	if err := r.Get(ctx, key, statefulSet); err != nil {
		return err
	}
	
	if statefulSet.Spec.Template.Annotations == nil {
		statefulSet.Spec.Template.Annotations = make(map[string]string)
	}
	statefulSet.Spec.Template.Annotations[annotationKey] = checksum
	
	return r.Update(ctx, statefulSet)
}

func (r *SecretRotationReconciler) updateDaemonSetAnnotation(ctx context.Context, namespace, name, annotationKey, checksum string) error {
	daemonSet := &appsv1.DaemonSet{}
	key := types.NamespacedName{Namespace: namespace, Name: name}
	
	if err := r.Get(ctx, key, daemonSet); err != nil {
		return err
	}
	
	if daemonSet.Spec.Template.Annotations == nil {
		daemonSet.Spec.Template.Annotations = make(map[string]string)
	}
	daemonSet.Spec.Template.Annotations[annotationKey] = checksum
	
	return r.Update(ctx, daemonSet)
}

func (r *SecretRotationReconciler) updateReplicaSetAnnotation(ctx context.Context, namespace, name, annotationKey, checksum string) error {
	replicaSet := &appsv1.ReplicaSet{}
	key := types.NamespacedName{Namespace: namespace, Name: name}
	
	if err := r.Get(ctx, key, replicaSet); err != nil {
		return err
	}
	
	if replicaSet.Spec.Template.Annotations == nil {
		replicaSet.Spec.Template.Annotations = make(map[string]string)
	}
	replicaSet.Spec.Template.Annotations[annotationKey] = checksum
	
	return r.Update(ctx, replicaSet)
}

func (r *SecretRotationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&secretsv1alpha1.SecretRotation{}).
		Complete(r)
}
