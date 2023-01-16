package plumber

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"path"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8syaml "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"gopkg.in/yaml.v2"
)

// BaseKustomizationPath is the location for the base kustomization.yaml file. This controller
// expects to find this file among the read files from the embed reference. This is the file
// that, after parse, is send over to all registered KMutators for further transformations.
const BaseKustomizationPath = "/kustomize/base/kustomization.yaml"

// KustomizeMutator is a function that is intended to mutate a Kustomization struct.
type KustomizeMutator func(context.Context, *types.Kustomization) error

// ObjectMutator is a function that is intended to mutate a Kubernetes object.
type ObjectMutator func(context.Context, client.Object) error

// PostApplyAction is a function that is intended to be executed after the creation of an
// object. These functions are called after each object is created. The creation of new
// objects resumes once these functions returns no error.
type PostApplyAction func(context.Context, client.Object) error

// FSMutator is a function that is intended to mutate a embed files prior to rendering them as
// a kustomize graph.
type FSMutator func(context.Context, filesys.FileSystem) error

// Renderer is a base controller to provide some tooling around rendering and creating resources
// based in a kustomize directory struct. Files are expected to be injected into this controller
// by means of an embed.FS struct. The filesystem struct, inside the embed.FS struct, is expected
// to comply with the following layout:
//
// /kustomize
// /kustomize/base/kustomization.yaml
// /kustomize/base/object_a.yaml
// /kustomize/base/object_a.yaml
// /kustomize/overlay0/kustomization.yaml
// /kustomize/overlay0/object_c.yaml
// /kustomize/overlay1/kustomization.yaml
// /kustomize/overlay1/object_d.yaml
//
// In other words, we have a base kustomization under base/ directory and each other directory is
// treated as an overlay to be applied on top of base.
type Renderer struct {
	cli          client.Client
	from         embed.FS
	fieldOwner   string
	forceOwner   bool
	unstructured bool
	kmutators    []KustomizeMutator
	omutators    []ObjectMutator
	postApply    []PostApplyAction
	fsmutators   []FSMutator
}

// NewRenderer returns a kustomize renderer reading and applying files provided by the embed.FS
// reference. Files are read from 'emb' into a filesys.FileSystem representation and then used
// as argument to Kustomize when generating objects.
func NewRenderer(cli client.Client, emb embed.FS, opts ...Option) *Renderer {
	ctrl := &Renderer{
		cli:        cli,
		from:       emb,
		fieldOwner: "plumber",
	}

	for _, opt := range opts {
		opt(ctrl)
	}

	return ctrl
}

// Apply applies provided overlay and creates objects in the kubernetes API using internal client.
// In case of failures there is no rollback so it is possible that this ends up partially creating
// the objects (returns at the first failure). Prior to object creation this function feeds all
// registered OMutators with the objects allowing for last time adjusts. Mutations in the default
// kustomization.yaml are also executed here.
func (r *Renderer) Apply(ctx context.Context, overlay string) error {
	objs, err := r.parse(ctx, overlay)
	if err != nil {
		return fmt.Errorf("error parsing kustomize files: %w", err)
	}
	for _, obj := range objs {
		for _, mut := range r.omutators {
			if err := mut(ctx, obj); err != nil {
				return fmt.Errorf("error mutating object: %w", err)
			}
		}

		opts := []client.PatchOption{client.FieldOwner(r.fieldOwner)}
		if r.forceOwner {
			opts = append(opts, client.ForceOwnership)
		}

		err := r.cli.Patch(ctx, obj, client.Apply, opts...)
		if err != nil {
			if !errors.IsNotFound(err) {
				return fmt.Errorf("error patching object: %w", err)
			}

			// XXX some verions of kubernetes fails to patch objects that do not
			// exist, at least I have seen this error in the past, this is kept
			// here for backwards compability. This should be removed in the future.
			if err := r.cli.Create(ctx, obj); err != nil {
				return fmt.Errorf("error creating object: %w", err)
			}
		}

		for _, action := range r.postApply {
			if err := action(ctx, obj); err != nil {
				return fmt.Errorf("error running post apply action: %w", err)
			}
		}
	}
	return nil
}

// Delete renders in memory the provided overlay and deletes all resulting objects from the
// kubernetes API. In case of failures there is no rollback so it is possible that this ends
// up partially deleting the objects (returns at the first failure).
func (r *Renderer) Delete(ctx context.Context, overlay string) error {
	objs, err := r.parse(ctx, overlay)
	if err != nil {
		return fmt.Errorf("error parsing kustomize files: %w", err)
	}

	for _, obj := range objs {
		for _, mut := range r.omutators {
			if err := mut(ctx, obj); err != nil {
				return fmt.Errorf("error mutating object: %w", err)
			}
		}
		if err := r.cli.Delete(ctx, obj); err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("error deleting object: %w", err)
		}
	}
	return nil
}

// parse reads kustomize files and returns them all parsed as valid client.Object structs. Loads
// everything from the embed.FS into a filesys.FileSystem instance, mutates the base kustomization
// and returns the objects as a slice of client.Object.
func (r *Renderer) parse(ctx context.Context, overlay string) ([]client.Object, error) {
	virtfs, err := LoadFS(r.from)
	if err != nil {
		return nil, fmt.Errorf("unable to load overlay: %w", err)
	}

	for _, mut := range r.fsmutators {
		if err := mut(ctx, virtfs); err != nil {
			return nil, fmt.Errorf("error mutating filesystem: %w", err)
		}
	}

	if err := r.mutateKustomization(ctx, virtfs); err != nil {
		return nil, fmt.Errorf("error setting object name prefix: %w", err)
	}

	res, err := krusty.MakeKustomizer(krusty.MakeDefaultOptions()).Run(
		virtfs, path.Join("kustomize", overlay),
	)
	if err != nil {
		return nil, fmt.Errorf("error running kustomize: %w", err)
	}

	var objs []client.Object
	for _, rsc := range res.Resources() {
		if r.unstructured {
			clientobj, err := r.unstructuredObject(rsc)
			if err != nil {
				return nil, fmt.Errorf("error converting type to unstructure: %w", err)
			}
			objs = append(objs, clientobj)
			continue
		}

		clientobj, err := r.typedObject(rsc)
		if err != nil {
			return nil, err
		}
		objs = append(objs, clientobj)
	}
	return objs, nil
}

// unstructuredObject converts a kustomize resource into a client.Object. This is useful when
// the object is not registered in the scheme.
func (r *Renderer) unstructuredObject(rsc *resource.Resource) (client.Object, error) {
	data, err := rsc.AsYAML()
	if err != nil {
		return nil, fmt.Errorf("error converting resource to yaml: %w", err)
	}

	obj := &unstructured.Unstructured{}
	dec := k8syaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	if _, _, err := dec.Decode(data, nil, obj); err != nil {
		return nil, fmt.Errorf("error decoding unstructured object: %w", err)
	}
	return obj, nil
}

// typedObject converts a kustomize resource to a typed client.Object. This is done by
// marshaling the resource to yaml and then unmarshaling it to the correct type.
func (r *Renderer) typedObject(rsc *resource.Resource) (client.Object, error) {
	runtimeobj, err := r.cli.Scheme().New(
		schema.GroupVersionKind{
			Group:   rsc.GetGvk().Group,
			Version: rsc.GetGvk().Version,
			Kind:    rsc.GetGvk().Kind,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("unable to instantiate runtime object: %w", err)
	}

	rawjson, err := rsc.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("error marshaling resource: %w", err)
	}

	if err := json.Unmarshal(rawjson, runtimeobj); err != nil {
		return nil, fmt.Errorf("error unmarshaling into object: %w", err)
	}

	clientobj, ok := runtimeobj.(client.Object)
	if !ok {
		// this should not happen as all runtime.Object also implement
		// client.Object. keeping this as a safeguard.
		gvkstr := runtimeobj.GetObjectKind().GroupVersionKind().String()
		return nil, fmt.Errorf("%s is not client.Object", gvkstr)
	}

	return clientobj, nil
}

// mutateKustomization feeds all registered KMutators with the parsed BaseKustomizationPath.
// After feeding KMutators the output is marshaled and written back to the filesys.FileSystem.
func (r *Renderer) mutateKustomization(ctx context.Context, fs filesys.FileSystem) error {
	if len(r.kmutators) == 0 {
		return nil
	}

	olddt, err := fs.ReadFile(BaseKustomizationPath)
	if err != nil {
		return fmt.Errorf("error reading base kustomization: %w", err)
	}

	var kust types.Kustomization
	if err := yaml.Unmarshal(olddt, &kust); err != nil {
		return fmt.Errorf("error parsing base kustomization: %w", err)
	}

	for _, mut := range r.kmutators {
		if err := mut(ctx, &kust); err != nil {
			return fmt.Errorf("error mutating kustomization: %w", err)
		}
	}

	newdt, err := yaml.Marshal(kust)
	if err != nil {
		return fmt.Errorf("error marshaling base kustomization: %w", err)
	}

	if err := fs.WriteFile(BaseKustomizationPath, newdt); err != nil {
		return fmt.Errorf("error writing base kustomization: %w", err)
	}
	return nil
}
