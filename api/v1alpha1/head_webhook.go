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

package v1alpha1

import (
	"context"
	"github.com/opentracing/opentracing-go"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var headlog = logf.Log.WithName("head-resource")

var c client.Client

func (r *Head) SetupWebhookWithManager(mgr ctrl.Manager) error {
	c = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-dummy-example-com-v1alpha1-head,mutating=true,failurePolicy=fail,sideEffects=None,groups=dummy.example.com,resources=heads,verbs=create;update,versions=v1alpha1,name=mhead.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &Head{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Head) Default() {
	headlog.Info("default", "name", r.Name)

	var current Head
	err := c.Get(context.TODO(), types.NamespacedName{
		Namespace: r.Namespace,
		Name:      r.Name,
	}, &current)

	if err == nil {
		headlog.Info("webhook: Head is found. Trace id will be kept")
	} else if errors.IsNotFound(err) {
		headlog.Info("webhook: Head is not found. Trace id will be injected")

		tracer := opentracing.GlobalTracer()
		span := tracer.StartSpan("get-head")
		textCarrier := opentracing.TextMapCarrier{}
		err := span.Tracer().Inject(span.Context(), opentracing.TextMap, textCarrier)
		if err != nil {
			headlog.Info("webhook: Error in context injecting", "error", err)
			return
		}

		headlog.Info("Injecting trace id", "uber-trace-id", textCarrier["uber-trace-id"])
		r.Annotations = map[string]string{"uber-trace-id": textCarrier["uber-trace-id"]}
	} else {
		headlog.Info("webhook: Error while fetching current head")
	}
}
