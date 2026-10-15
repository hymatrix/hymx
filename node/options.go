package node

// DefaultRecoveryConcurrency limits whole-process recovery, including checkpoint
// decoding, module loading and runtime initialization, on memory-constrained nodes.
const DefaultRecoveryConcurrency = 1

type nodeOptions struct {
	recoveryConcurrency int
}

type Option func(*nodeOptions)

// WithRecoveryConcurrency sets the maximum number of concurrent recovery/replay
// tasks. Values less than one are programming/configuration errors.
func WithRecoveryConcurrency(limit int) Option {
	if limit < 1 {
		panic("recovery concurrency must be at least 1")
	}
	return func(options *nodeOptions) {
		options.recoveryConcurrency = limit
	}
}
