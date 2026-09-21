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
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	mlv1alpha1 "github.com/samjove/model-controller/api/v1alpha1"
)

// ModelDeploymentReconciler reconciles a ModelDeployment object
type ModelDeploymentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ml.model-controller.com,resources=modeldeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ml.model-controller.com,resources=modeldeployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ml.model-controller.com,resources=modeldeployments/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ModelDeployment object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.25.0/pkg/reconcile
func (r *ModelDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	// 1. Fetch desired  ModelDeployment

	var model mlv1alpha1.ModelDeployment

	if err := r.Get(
		ctx,
		req.NamespacedName,
		&model,
	); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound((err))
	}

	// 2. Fetch observed Deployment

	var deployment appsv1.Deployment

	err := r.Get(
		ctx,
		types.NamespacedName{
			Name:      model.Name,
			Namespace: model.Namespace,
		},
		&deployment,
	)

	// 3. Deployment doesn't exist

	if apierrors.IsNotFound(err) {
		replicas := model.Spec.Replicas

		deployment = appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      model.Name,
				Namespace: model.Namespace,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": model.Name,
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": model.Name,
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "model-server",
								Image: model.Spec.Image,
							},
						},
					},
				},
			},
		}

		if err := ctrl.SetControllerReference(
			&model,
			&deployment,
			r.Scheme,
		); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.Create(ctx, &deployment); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	if err != nil {
		return ctrl.Result{}, err
	}

	// 4. Deployment exists: detect drift

	changed := false

	if deployment.Spec.Replicas == nil ||
		*deployment.Spec.Replicas != model.Spec.Replicas {
		replicas := model.Spec.Replicas
		deployment.Spec.Replicas = &replicas
		changed = true
	}

	if len(deployment.Spec.Template.Spec.Containers) > 0 &&
		deployment.Spec.Template.Spec.Containers[0].Image != model.Spec.Image {

		deployment.Spec.Template.Spec.Containers[0].Image = model.Spec.Image

		changed = true
	}

	// 5. Correct drift

	if changed {
		if err := r.Update(ctx, &deployment); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ModelDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mlv1alpha1.ModelDeployment{}).
		Named("modeldeployment").
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
