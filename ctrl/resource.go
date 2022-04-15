package ctrl

import (
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	asclv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/kustomize/kyaml/resid"
)

// resourceToObject converts provided resource.Resource into a client.Object representation by
// marshaling and unmarshaling into a kubernetes struct. This function will return an error if
// Resource GVK is not mapped to a struct.
func resourceToObject(res *resource.Resource) (client.Object, error) {
	var obj client.Object

	switch res.GetGvk() {
	case resid.Gvk{
		Version: "v1",
		Kind:    "Secret",
	}:
		obj = &corev1.Secret{}

	case resid.Gvk{
		Version: "v1",
		Kind:    "Service",
	}:
		obj = &corev1.Service{}

	case resid.Gvk{
		Version: "v1",
		Kind:    "ServiceAccount",
	}:
		obj = &corev1.ServiceAccount{}

	case resid.Gvk{
		Version: "v1",
		Kind:    "PersistentVolumeClaim",
	}:
		obj = &corev1.PersistentVolumeClaim{}

	case resid.Gvk{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}:
		obj = &appsv1.Deployment{}

	case resid.Gvk{
		Group:   "autoscaling",
		Version: "v2beta2",
		Kind:    "HorizontalPodAutoscaler",
	}:
		obj = &asclv1.HorizontalPodAutoscaler{}

	default:
		return nil, fmt.Errorf("unmapped type %+v", res.GetGvk())
	}

	rawjson, err := res.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("error marshaling resource: %w", err)
	}

	if err := json.Unmarshal(rawjson, obj); err != nil {
		return nil, fmt.Errorf("error unmarshaling object: %w", err)
	}
	return obj, nil
}
