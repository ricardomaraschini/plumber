package ctrl

// Option is a function that sets an option in a KustCtrl controller.
type Option func(*KustCtrl)

// WithKMutator register a Kustomization mutator into the controller
func WithKMutator(mutator KMutator) Option {
	return func(k *KustCtrl) {
		k.KMutators = append(k.KMutators, mutator)
	}
}

// WithOMutator register an Object mutator into the controller
func WithOMutator(mutator OMutator) Option {
	return func(k *KustCtrl) {
		k.OMutators = append(k.OMutators, mutator)
	}
}

// WithResourceToObjectFn sets up a different translation function to be used when converting a
// resource.Resource into a client.Object.
func WithResourceToObjectFn(fn objectMappers) Option {
	return func(k *KustCtrl) {
		k.objmappers = append([]objectMappers{fn}, k.objmappers...)
	}
}
