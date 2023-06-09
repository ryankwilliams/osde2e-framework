package rosa

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/openshift/osde2e-framework/internal/cmd"

	clustersmgmtv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
)

// oidcConfigError represents the custom error
type oidcConfigError struct {
	action string
	err    error
}

// Error returns the formatted error message when oidcConfigError is invoked
func (o *oidcConfigError) Error() string {
	return fmt.Sprintf("%s oidc config failed: %v", o.action, o.err)
}

// createOIDCConfig creates an oidc config if one does not already exist
func (r *Provider) createOIDCConfig(ctx context.Context, prefix, installerRoleArn string, managed bool) (string, error) {
	const action = "create"
	var oidcConfigID string

	if prefix == "" || installerRoleArn == "" {
		return "", &oidcConfigError{action: action, err: fmt.Errorf("some parameters are undefined")}
	}

	oidcConfig, err := r.oidcConfigLookup(ctx, prefix)
	if oidcConfig != nil {
		return oidcConfig.ID(), nil
	} else if err != nil {
		return "", &oidcConfigError{action: action, err: err}
	}

	commandArgs := []string{"create", "oidc-config", "--output", "json", "--mode", "auto", "--yes"}
	commandArgs = append(commandArgs, fmt.Sprintf("--managed=%s", strconv.FormatBool(managed)))
	commandArgs = append(commandArgs, "--installer-role-arn", installerRoleArn)
	commandArgs = append(commandArgs, "--prefix", prefix)

	err = r.awsCredentials.CallFuncWithCredentials(ctx, func(ctx context.Context) error {
		stdout, _, err := cmd.Run(exec.CommandContext(ctx, r.rosaBinary, commandArgs...))
		if err != nil {
			return err
		}

		output, err := cmd.ConvertJSONStringToMap(stdout)
		if err != nil {
			return fmt.Errorf("failed to convert output to map: %v", err)
		}

		oidcConfigID = fmt.Sprint(output["id"])

		return nil
	})
	if err != nil {
		return "", &oidcConfigError{action: action, err: err}
	}

	return oidcConfigID, nil
}

// deleteOIDCConfig deletes the oidc config using the id
func (r *Provider) deleteOIDCConfig(ctx context.Context, oidcConfigID string) error {
	commandArgs := []string{"delete", "oidc-config", "--mode", "auto", "--oidc-config-id", oidcConfigID, "--yes"}

	err := r.awsCredentials.CallFuncWithCredentials(ctx, func(ctx context.Context) error {
		_, _, err := cmd.Run(exec.CommandContext(ctx, r.rosaBinary, commandArgs...))
		return err
	})
	if err != nil {
		return &oidcConfigError{action: "delete", err: err}
	}

	return nil
}

// getClusterOIDCConfig retrieves the oidc config associated with the cluster
func (r *Provider) getClusterOIDCConfig(ctx context.Context, clusterID string) (*clustersmgmtv1.OidcConfig, error) {
	response, err := r.ClustersMgmt().V1().Clusters().Cluster(clusterID).Get().SendContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve cluster: %v", err)
	}

	return response.Body().AWS().STS().OidcConfig(), nil
}

// oidcConfigLookup checks if an oidc config already exists using the provided prefix
func (r *Provider) oidcConfigLookup(ctx context.Context, prefix string) (*clustersmgmtv1.OidcConfig, error) {
	response, err := r.ClustersMgmt().V1().OidcConfigs().List().SendContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve oidc configs from ocm: %v", err)
	}

	for _, oidcConfig := range response.Items().Slice() {
		if strings.Contains(oidcConfig.SecretArn(), prefix) {
			return oidcConfig, nil
		}
	}

	return nil, nil
}

// deleteOIDCConfigProvider deletes the oidc config provider associated to the cluster
func (r *Provider) deleteOIDCConfigProvider(ctx context.Context, clusterID string) error {
	commandArgs := []string{"delete", "oidc-provider", "--cluster", clusterID, "--mode", "auto", "--yes"}

	err := r.awsCredentials.CallFuncWithCredentials(ctx, func(ctx context.Context) error {
		_, _, err := cmd.Run(exec.CommandContext(ctx, r.rosaBinary, commandArgs...))
		return err
	})
	if err != nil {
		return &oidcConfigError{action: "delete", err: err}
	}

	return nil
}
