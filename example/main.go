package main

import (
	"context"
	"embed"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	kurlkinds "github.com/replicatedhq/kurlkinds/pkg/apis"
	"github.com/replicatedhq/plumber"
)

//go:embed kustomize
var resources embed.FS

func main() {
	cli, err := client.New(config.GetConfigOrDie(), client.Options{})
	if err != nil {
		panic(err)
	}

	// if we have objects that are not part of the objects supported by the
	// default controller runtime client we may register them here. below
	// we register the kurl types (Installer) so we can create them in the
	// cluster. if no client can't be found to provide a concrete implementation
	// for a given type then you have to use Unstructured, see WithUnstructured.
	kurlkinds.AddToScheme(cli.Scheme())

	options := []plumber.Option{
		// WithUnstructured does not attempt to convert objects to a concrete
		// implementation, it will use Unstructured instead. This affects the
		// WithObjectMutator below as it will start to receive unstructured
		// objects as well:
		// plumber.WithUnstructured(),
		plumber.WithObjectMutator(
			func(ctx context.Context, obj client.Object) error {
				// here we can edit the objects before they are
				// created in the cluster.
				deploy, ok := obj.(*appsv1.Deployment)
				if !ok {
					return nil
				}

				// as an example we append an annotation to the nginx
				// deployment.
				if deploy.Annotations == nil {
					deploy.Annotations = map[string]string{}
				}
				deploy.Annotations["this-was-a-mutation"] = time.Now().String()
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
		plumber.WithFSMutator(
			func(ctx context.Context, fs filesys.FileSystem) error {
				// here we can edit the filesystem before yaml files are
				// rendered with kustomize.
				return nil
			},
		),
	}

	renderer := plumber.NewRenderer(cli, resources, options...)
	for _, overlay := range []string{"base", "scale-up", "scale-down"} {
		if err := renderer.Render(context.Background(), overlay); err != nil {
			panic(err)
		}
		fmt.Printf("overlay %q applied\n", overlay)
	}
}
