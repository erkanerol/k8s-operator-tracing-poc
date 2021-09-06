module github.com/erkanerol/k8s-operator-tracing

go 1.16

require (
	github.com/HdrHistogram/hdrhistogram-go v1.1.2 // indirect
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/opentracing/opentracing-go v1.2.0
	github.com/uber/jaeger-client-go v2.29.1+incompatible
	github.com/uber/jaeger-lib v2.4.1+incompatible
	k8s.io/api v0.21.2 // indirect
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v0.21.2
	sigs.k8s.io/controller-runtime v0.9.2
)
