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

// HeadReconciler reconciles a Head object
type HeadReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=dummy.example.com,resources=heads,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dummy.example.com,resources=heads/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dummy.example.com,resources=heads/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Head object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *HeadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	var head dummyv1alpha1.Head
	err := r.Get(context.TODO(), types.NamespacedName{
		Namespace: req.Namespace,
		Name:      req.Name,
	}, &head)

	if errors.IsNotFound(err) {
		log.Log.Info("Head is not found.")
		return ctrl.Result{}, nil
	}

	if err != nil {
		log.Log.Error(err, "Error in fetching head")
		return ctrl.Result{}, err
	}

	if head.DeletionTimestamp != nil {
		log.Log.Info("Head is deleted.")
		return ctrl.Result{}, nil
	}

	expected, err := generateChildFromHead(&head, r.Scheme)
	if err != nil {
		return ctrl.Result{}, err
	}

	var existing dummyv1alpha1.ChildA
	err = r.Get(context.TODO(), types.NamespacedName{
		Namespace: expected.Namespace,
		Name:      expected.Name,
	}, &existing)

	if err == nil {
		log.Log.Info("Child is found.", "isReady", existing.Status.Ready)
		if existing.Status.Ready != head.Status.Ready {
			log.Log.Info("Status of head will be updated")
			head.Status.Ready = existing.Status.Ready

			span, err := startSpanForHead(&head, fmt.Sprintf("head-ready-%v", head.Status.Ready))
			if err != nil {
				return ctrl.Result{}, err
			}

			err = r.Status().Update(context.TODO(), &head)
			if err != nil {
				log.Log.Error(err, "Error while updating head status")
				return ctrl.Result{}, err
			}
			span.Finish()

		}
	} else if errors.IsNotFound(err) {
		log.Log.Info("ChildA doesn't exist. Creating...")

		span, err := startSpanForHead(&head, "create-child")
		if err != nil {
			return ctrl.Result{}, err
		}

		err = r.Create(context.TODO(), expected)
		if err != nil {
			log.Log.Error(err, "Error in creation of ChildA")
			return ctrl.Result{}, err
		}
		span.Finish()
	} else {
		log.Log.Error(err, "Error in fetching ChildA")
		return ctrl.Result{}, err
	}

	log.Log.Info("Head finished")
	return ctrl.Result{}, nil

}

func startSpanForHead(head *dummyv1alpha1.Head, operation string) (opentracing.Span, error) {
	log.Log.Info("Trace id in Head:", "id", head.Annotations["uber-trace-id"])
	tracer := opentracing.GlobalTracer()
	textCarrier := opentracing.TextMapCarrier{"uber-trace-id": head.Annotations["uber-trace-id"]}

	spanCtx, err := tracer.Extract(opentracing.TextMap, textCarrier)
	if err != nil {
		log.Log.Info("Error in context injecting", "error", err)
		return nil, err
	}

	span := tracer.StartSpan(operation, opentracing.ChildOf(spanCtx))
	return span, nil
}

func generateChildFromHead(head *dummyv1alpha1.Head, scheme *runtime.Scheme) (*dummyv1alpha1.ChildA, error) {
	child := &dummyv1alpha1.ChildA{
		ObjectMeta: metav1.ObjectMeta{
			Name:        head.Name,
			Namespace:   head.Namespace,
			Annotations: head.Annotations,
		},
		Spec: dummyv1alpha1.ChildASpec{
			Image: head.Spec.ChildA,
		},
	}

	err := controllerutil.SetControllerReference(head, child, scheme)
	if err != nil {
		log.Log.Error(err, "Error setting owner reference")
	}

	return child, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *HeadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dummyv1alpha1.Head{}).
		Owns(&dummyv1alpha1.ChildA{}).
		Complete(r)
}
