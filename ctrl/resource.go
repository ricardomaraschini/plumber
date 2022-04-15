package ctrl

import (
	appsv1 "k8s.io/api/apps/v1"
	asclv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/kustomize/kyaml/resid"

	routev1 "github.com/openshift/api/route/v1"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// objectToResource finds the proper client.Object for the provided resource.Resource. Returns
// nil if the resource.Resource can't be managed to any client.Object.
func objectToResource(res *resource.Resource) client.Object {
	switch res.GetGvk() {
	case resid.Gvk{
		Version: "v1",
		Kind:    "Secret",
	}:
		return &corev1.Secret{}

	case resid.Gvk{
		Version: "v1",
		Kind:    "ConfigMap",
	}:
		return &corev1.ConfigMap{}

	case resid.Gvk{
		Version: "v1",
		Kind:    "Service",
	}:
		return &corev1.Service{}

	case resid.Gvk{
		Version: "v1",
		Kind:    "ServiceAccount",
	}:
		return &corev1.ServiceAccount{}

	case resid.Gvk{
		Version: "v1",
		Kind:    "PersistentVolumeClaim",
	}:
		return &corev1.PersistentVolumeClaim{}

	case resid.Gvk{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}:
		return &appsv1.Deployment{}

	case resid.Gvk{
		Group:   "autoscaling",
		Version: "v2beta2",
		Kind:    "HorizontalPodAutoscaler",
	}:
		return &asclv1.HorizontalPodAutoscaler{}

	case resid.Gvk{
		Group:   "tekton.dev",
		Version: "v1beta1",
		Kind:    "Pipeline",
	}:
		return &tektonv1beta1.Pipeline{}

	case resid.Gvk{
		Group:   "tekton.dev",
		Version: "v1beta1",
		Kind:    "Task",
	}:
		return &tektonv1beta1.Task{}

	case resid.Gvk{
		Group:   "route.openshift.io",
		Version: "v1",
		Kind:    "Route",
	}:
		return &routev1.Route{}

	default:
		return nil
	}
}
