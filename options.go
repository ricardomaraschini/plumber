package plumber

// Option is a function that sets an option in a Renderer.
type Option func(*Renderer)

// WithKustomizeMutator register a Kustomization mutator into the controller.
func WithKustomizeMutator(mutator KustomizeMutator) Option {
	return func(r *Renderer) {
		r.kmutators = append(r.kmutators, mutator)
	}
}

// WithObjectMutator register an Object mutator into the controller.
func WithObjectMutator(mutator ObjectMutator) Option {
	return func(r *Renderer) {
		r.omutators = append(r.omutators, mutator)
	}
}

// WithUnstructured uses unstructured objects instead of typed ones.
func WithUnstructured() Option {
	return func(r *Renderer) {
		r.unstructured = true
	}
}
