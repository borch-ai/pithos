package pipeline

import (
	"context"
	"errors"
)

// DeployOptions holds configuration parameters for the deploy command.
type DeployOptions struct {
	InputDir string
}

// Deploy packages the final assets and metadata for KDP upload.
func Deploy(ctx context.Context, opts DeployOptions) error {
	return errors.New("deploy engine is not implemented yet")
}
