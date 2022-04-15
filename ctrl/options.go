package ctrl

// Option is a function that sets an option in a KustCtrl controller.
type Option func(*KustCtrl)

// WithResourceToObjectFn sets up a different translation function to be used when converting a
// resource.Resource into a client.Object.
func WithResourceToObjectFn(fn ResourceToObjectFn) Option {
	return func(k *KustCtrl) {
		k.restoobj = fn
	}
}
