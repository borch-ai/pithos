package pipeline

import (
	"context"
	"errors"
)

// ErrDeployNotImplemented is returned when the deploy engine is not yet implemented.
var ErrDeployNotImplemented = errors.New("deploy engine is not implemented yet")

// DeployOptions holds configuration parameters for the deploy command.
type DeployOptions struct {
	InputDir string
}

// Deploy packages the final assets and metadata for KDP upload.
func Deploy(_ context.Context, opts DeployOptions) error {
	if opts.InputDir == "" {
		return errors.New("input directory is required")
	}
	return ErrDeployNotImplemented
}
