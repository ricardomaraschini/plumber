package plumber

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"path"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"

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
	cli       client.Client
	from      embed.FS
	fowner    string
	kmutators []KustomizeMutator
	omutators []ObjectMutator
}

// NewRenderer returns a kustomize renderer reading and applying files provided by the embed.FS
// reference. Files are read from 'emb' into a filesys.FileSystem representation and then used
// as argument to Kustomize when generating objects.
func NewRenderer(cli client.Client, emb embed.FS, opts ...Option) *Renderer {
	ctrl := &Renderer{
		cli:    cli,
		from:   emb,
		fowner: "undefined",
	}

	for _, opt := range opts {
		opt(ctrl)
	}

	return ctrl
}

// Render applies provided overlay and creates objects in the kubernetes API using internal client.
// In case of failures there is no rollback so it is possible that this ends up partially creating
// the objects (returns at the first failure). Prior to object creation this function feeds all
// registered OMutators with the objects allowing for last time adjusts. Mutations in the default
// kustomization.yaml are also executed here.
func (r *Renderer) Render(ctx context.Context, overlay string) error {
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

		err := r.cli.Patch(ctx, obj, client.Apply, client.FieldOwner(r.fowner))
		if err == nil {
			continue
		}

		if !errors.IsNotFound(err) {
			return fmt.Errorf("error patching object: %w", err)
		}

		if err := r.cli.Create(ctx, obj); err != nil {
			return fmt.Errorf("error creating object: %w", err)
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
		runtimeobj, err := r.cli.Scheme().New(
			schema.GroupVersionKind{
				Group:   rsc.GetGvk().Group,
				Version: rsc.GetGvk().Version,
				Kind:    rsc.GetGvk().Kind,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("unable to create object: %w", err)
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

		objs = append(objs, clientobj)
	}
	return objs, nil
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
