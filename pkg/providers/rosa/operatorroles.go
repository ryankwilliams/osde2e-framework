package rosa

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/openshift/osde2e-framework/internal/cmd"
)

// operatorRoleError represents the custom error
type operatorRoleError struct {
	action string
	err    error
}

// Error returns the formatted error message when operatorRoleError is invoked
func (o *operatorRoleError) Error() string {
	return fmt.Sprintf("%s operator role failed: %v", o.action, o.err)
}

// deleteOIDCConfigProvider deletes the oidc config provider associated to the cluster
func (r *Provider) deleteOperatorRoles(ctx context.Context, clusterID string) error {
	commandArgs := []string{"delete", "operator-roles", "--cluster", clusterID, "--mode", "auto", "--yes"}

	err := r.awsCredentials.CallFuncWithCredentials(ctx, func(ctx context.Context) error {
		_, _, err := cmd.Run(exec.CommandContext(ctx, r.rosaBinary, commandArgs...))
		return err
	})
	if err != nil {
		return &operatorRoleError{action: "delete", err: err}
	}

	return nil
}
