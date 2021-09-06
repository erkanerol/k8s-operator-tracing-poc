/*
Copyright 2021.

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
	"github.com/opentracing/opentracing-go"
	apiv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dummyv1alpha1 "github.com/erkanerol/k8s-operator-tracing/api/v1alpha1"
)

// ChildAReconciler reconciles a ChildA object
type ChildAReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *ChildAReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dummyv1alpha1.ChildA{}).
		Owns(&apiv1.Deployment{}).
		Complete(r)
}

//+kubebuilder:rbac:groups=dummy.example.com,resources=childas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dummy.example.com,resources=childas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dummy.example.com,resources=childas/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ChildA object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *ChildAReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	log.Log.Info("Detected change on child A begin")

	var child dummyv1alpha1.ChildA
	var err error

	err = r.Get(context.TODO(), types.NamespacedName{
		Namespace: req.Namespace,
		Name:      req.Name,
	}, &child)

	if errors.IsNotFound(err) {
		log.Log.Info("Child is not found.")
		return ctrl.Result{}, nil
	}

	if err != nil {
		log.Log.Error(err, "Error in fetching child")
		return ctrl.Result{}, err
	}

	if child.DeletionTimestamp != nil {
		log.Log.Info("Child is deleted.")
		return ctrl.Result{}, nil
	}

	expected, err := generateDeploymentFromChild(&child, r.Scheme)
	if err != nil {
		return ctrl.Result{}, err
	}

	var existing apiv1.Deployment
	err = r.Get(context.TODO(), types.NamespacedName{
		Namespace: expected.Namespace,
		Name:      expected.Name,
	}, &existing)

	if err == nil {
		log.Log.Info("Deployment is found. Status check")
		isReady := false
		if existing.Status.Replicas > 0 && existing.Status.ReadyReplicas == existing.Status.Replicas {
			isReady = true
		}
		log.Log.Info("status", "isReady", isReady,
			"readyReplicas", existing.Status.ReadyReplicas,
			"replicas", existing.Status.Replicas)

		if isReady != child.Status.Ready {
			log.Log.Info("Status of child will be updated")
			child.Status.Ready = isReady

			span, err := startSpanForChild(&child, fmt.Sprintf("child-ready-%v", isReady))
			if err != nil {
				return ctrl.Result{}, err
			}

			err = r.Status().Update(context.TODO(), &child)
			if err != nil {
				log.Log.Error(err, "Error while updating child status")
				return ctrl.Result{}, err
			}
			span.Finish()
		}
	} else if errors.IsNotFound(err) {
		log.Log.Info("Deployment not found. Will be created", "child name", child.Name)
		span, err := startSpanForChild(&child, "create-deploy")
		if err != nil {
			return ctrl.Result{}, err
		}

		err = r.Create(context.TODO(), &expected)
		if err != nil {
			log.Log.Error(err, "Error in creation of deployment")
			return ctrl.Result{}, err
		}

		span.Finish()
	} else {
		log.Log.Error(err, "Error in fetching deployment")
		return ctrl.Result{}, err
	}

	log.Log.Info("ChildA finished")
	return ctrl.Result{}, nil
}

func startSpanForChild(child *dummyv1alpha1.ChildA, operation string) (opentracing.Span, error) {
	log.Log.Info("Trace id in ChildA:", "id", child.Annotations["uber-trace-id"])
	tracer := opentracing.GlobalTracer()
	textCarrier := opentracing.TextMapCarrier{"uber-trace-id": child.Annotations["uber-trace-id"]}

	spanCtx, err := tracer.Extract(opentracing.TextMap, textCarrier)
	if err != nil {
		log.Log.Info("Error in context injecting", "error", err)
		return nil, err
	}

	span := tracer.StartSpan(operation, opentracing.ChildOf(spanCtx))
	return span, nil
}

func generateDeploymentFromChild(child *dummyv1alpha1.ChildA, scheme *runtime.Scheme) (apiv1.Deployment, error) {
	deployment := apiv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        child.Name,
			Namespace:   child.Namespace,
			Annotations: child.Annotations,
		},
		Spec: apiv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":    "demo",
					"parent": child.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":    "demo",
						"parent": child.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "main",
							Image: child.Spec.Image,
						},
					},
				},
			},
		},
	}
	err := controllerutil.SetControllerReference(child, &deployment, scheme)
	if err != nil {
		log.Log.Error(err, "Error setting owner reference")
	}
	return deployment, err
}
