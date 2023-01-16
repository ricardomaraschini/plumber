package plumber

// Option is a function that sets an option in a Renderer.
type Option func(*Renderer)

// WithFieldOwner specifies the string to be used when patching objects.
// This sets the field manager on the patch request.
func WithFieldOwner(owner string) Option {
	return func(r *Renderer) {
		r.fieldOwner = owner
	}
}

// WithForceOwnership sets the force ownership option during calls to Patch.
func WithForceOwnership() Option {
	return func(r *Renderer) {
		r.forceOwner = true
	}
}

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

// WithPostApplyAction register a post apply action into the controller. Post
// apply actions are called after the object is created in the API server. These
// actions are called sequentially and the creation of new objects is resumed
// once all actions return no error.
func WithPostApplyAction(action PostApplyAction) Option {
	return func(r *Renderer) {
		r.postApply = append(r.postApply, action)
	}
}

// WithFSMutator register a filesystem mutator into the controller. FS mutators
// are called before the Kustomize and Object mutators, allows for fine grained
// changes on the filesystem prior to rendering objects with kustomize.
func WithFSMutator(mutator FSMutator) Option {
	return func(r *Renderer) {
		r.fsmutators = append(r.fsmutators, mutator)
	}
}
