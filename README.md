# Plumber

Plumber is a Go package that allows managing objects creation and patching,
piping them from YAML to its concrete in-cluster representation. This package
leverages Kustomize and its goal is to make object creation and management
easier.

## Example

To see the full example please see https://github.com/ricardomaraschini/plumber/tree/main/example

```go
// start by embedding all YAML files we are going to render and create.
//go:embed kustomize
var resources embed.FS

func main() {
	cli, err := client.New(config.GetConfigOrDie(), client.Options{})
	if err != nil {
		panic(err)
	}

	// if we have objects that are not part of the objects supported by the
	// default controller runtime client we can register them here. below
	// we register the kurl types so we can create them in the cluster. if
	// no client can't be found to provide a concrete implementation of a
	// given type then we have to use Unstructured instead, see below.
	kurlkinds.AddToScheme(cli.Scheme())

	options := []plumber.Option{
		// when using WithUnstructured objects are kept as Unstructured,
		// no conversion to concrete objects are made by the library. be
		// aware that whenever you use this the Mutator will receive an
		// Unstructured object instead of a concrete type (Pod, Service,
		// Deployment, etc).
		// plumber.WithUnstructured(),
		plumber.WithObjectMutator(
			func(ctx context.Context, obj client.Object) error {
				// here we can edit the objects before they are
				// created in the cluster. if using Unstructured
				// the object is going to be of type Unstructured.
				deploy, ok := obj.(*appsv1.Deployment)
				if !ok {
					return nil
				}

				// as an example we append an annotation to the nginx
				// deployment we are creating.
				if deploy.Annotations == nil {
					deploy.Annotations = map[string]string{}
				}
				deploy.Annotations["this-is-a-mutation"] = "foo"
				return nil
			},
		),
		plumber.WithKustomizeMutator(
			func(ctx context.Context, k *types.Kustomization) error {
				// here we can edit the "kustomization.yaml"
				// before yaml files are rendered.
				return nil
			},
		),
	}

	// for sake of having a complete example, here we apply different overlays
	// on top of the base deployment.
	for _, overlay := range []string{"base", "scale-up", "scale-down"} {
		if err := plumber.NewRenderer(cli, resources, options...).Render(
			context.Background(), overlay,
		); err != nil {
			panic(err)
		}
		fmt.Printf("overlay %q applied\n", overlay)
	}
}
```
