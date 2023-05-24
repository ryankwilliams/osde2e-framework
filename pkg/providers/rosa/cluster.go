package rosa

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"time"

	"github.com/Masterminds/semver"
	"github.com/openshift/osde2e-framework/internal/cmd"

	clustersmgmtv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
)

// CreateClusterOptions represents data used to create clusters
type CreateClusterOptions struct {
	ChannelGroup       string
	ClusterName        string
	ComputeMachineType string
	HostedCP           bool
	MachineCidr        string
	Mode               string
	OIDCConfigManaged  bool
	Properties         string
	Replicas           int
	STS                bool
	Version            string

	accountRoles accountRoles
	oidcConfigID string
	subnetIDs    string
}

// DeleteClusterOptions represents data used to delete clusters
type DeleteClusterOptions struct {
	ClusterID   string
	ClusterName string
	HostedCP    bool
	STS         bool
}

// clusterError represents the custom error
type clusterError struct {
	action string
	err    error
}

// Error returns the formatted error message when clusterError is invoked
func (c *clusterError) Error() string {
	return fmt.Sprintf("%s cluster failed: %v", c.action, c.err)
}

// CreateCluster creates a rosa cluster using the provided inputs
func (r *Provider) CreateCluster(ctx context.Context, options *CreateClusterOptions) (string, error) {
	const action = "create"
	clusterReadyAttempts := 120

	options.setDefaultCreateClusterOptions()

	if options.STS {
		version, err := semver.NewVersion(options.Version)
		if err != nil {
			return "", &clusterError{action: action, err: fmt.Errorf("failed to parse version into semantic version: %v", err)}
		}
		majorMinor := fmt.Sprintf("%d.%d", version.Major(), version.Minor())

		accountRoles, err := r.createAccountRoles(ctx, options.ClusterName, majorMinor, options.ChannelGroup)
		if err != nil {
			return "", &clusterError{action: action, err: err}
		}
		options.accountRoles = *accountRoles
	}

	if options.HostedCP {
		clusterReadyAttempts = 30

		// TODO: region check for hcp support

		oidcConfigID, err := r.createOIDCConfig(
			ctx,
			options.ClusterName,
			options.accountRoles.installerRoleARN,
			options.OIDCConfigManaged,
		)
		if err != nil {
			return "", &clusterError{action: action, err: err}
		}

		options.oidcConfigID = oidcConfigID

		// TODO: Handle working directory
		vpc, err := r.createHostedControlPlaneVPC(
			ctx,
			options.ClusterName,
			r.awsCredentials.Region,
			"/tmp",
		)
		if err != nil {
			return "", &clusterError{action: action, err: err}
		}

		options.subnetIDs = fmt.Sprintf("%s,%s", vpc.privateSubnet, vpc.publicSubnet)
	}

	clusterID, err := r.createCluster(ctx, options)
	if err != nil {
		return "", &clusterError{action: action, err: err}
	}

	log.Printf("Cluster ID: %s\n", clusterID)

	err = r.waitForClusterToBeReady(ctx, clusterID, clusterReadyAttempts)
	if err != nil {
		return clusterID, &clusterError{action: action, err: err}
	}

	err = r.waitForClusterHealthChecksToSucceed()
	if err != nil {
		return clusterID, &clusterError{action: action, err: err}
	}

	return clusterID, nil
}

// DeleteCluster deletes a rosa cluster using the provided inputs
func (r *Provider) DeleteCluster(ctx context.Context, options *DeleteClusterOptions) error {
	const action = "delete"
	var (
		clusterDeletedAttempts = 30
		oidcConfigID           string
	)

	options.setDefaultDeleteClusterOptions()

	if options.HostedCP {
		oidcConfig, err := r.getClusterOIDCConfig(ctx, options.ClusterID)
		if err != nil {
			return &clusterError{action: action, err: err}
		}
		oidcConfigID = oidcConfig.ID()
	}

	err := r.deleteCluster(ctx, options.ClusterID)
	if err != nil {
		return &clusterError{action: action, err: err}
	}

	err = r.waitForClusterToBeDeleted(ctx, options.ClusterName, clusterDeletedAttempts)
	if err != nil {
		return &clusterError{action: action, err: err}
	}

	if options.STS {
		err = r.deleteOperatorRoles(ctx, options.ClusterID)
		if err != nil {
			return &clusterError{action: action, err: err}
		}

		err = r.deleteOIDCConfigProvider(ctx, options.ClusterID)
		if err != nil {
			return &clusterError{action: action, err: err}
		}
	}

	if options.HostedCP {
		err := r.deleteOIDCConfig(ctx, oidcConfigID)
		if err != nil {
			return &clusterError{action: action, err: err}
		}

		// TODO: Handle working directory
		err = r.deleteHostedControlPlaneVPC(
			ctx,
			options.ClusterName,
			r.awsCredentials.Region,
			"/tmp",
		)
		if err != nil {
			return &clusterError{action: action, err: err}
		}
	}

	if options.STS {
		err = r.deleteAccountRoles(ctx, options.ClusterName)
		if err != nil {
			return &clusterError{action: action, err: err}
		}
	}

	return nil
}

// validateCreateClusterOptions verifies required options are set and sets defaults if undefined
func validateCreateClusterOptions(options *CreateClusterOptions) (*CreateClusterOptions, error) {
	if options.ClusterName == "" {
		return options, fmt.Errorf("cluster name is required")
	}

	if options.ChannelGroup == "" {
		options.ChannelGroup = "stable"
	}

	if options.ComputeMachineType == "" {
		options.ComputeMachineType = "m5.xlarge"
	}

	if options.MachineCidr == "" {
		options.MachineCidr = "10.0.0.0/16"
	}

	if options.Version == "" {
		return options, fmt.Errorf("version is required")
	}

	if options.Replicas == 0 {
		options.Replicas = 2
	}

	if options.HostedCP {
		if options.oidcConfigID == "" {
			return options, fmt.Errorf("oidc config id is required for hosted control plane clusters")
		}

		if options.subnetIDs == "" {
			return options, fmt.Errorf("subnet ids is required for hosted control plane clusters")
		}
	}

	if options.accountRoles.controlPlaneRoleARN == "" {
		return options, fmt.Errorf("iam role arn for control plane is required")
	}

	if options.accountRoles.installerRoleARN == "" {
		return options, fmt.Errorf("iam role arn for installer is required")
	}

	if options.accountRoles.supportRoleARN == "" {
		return options, fmt.Errorf("iam role arn for support role is required")
	}

	if options.accountRoles.workerRoleARN == "" {
		return options, fmt.Errorf("iam role for worker role is required")
	}

	return options, nil
}

// createCluster handles sending the request to create the cluster
func (r *Provider) createCluster(ctx context.Context, options *CreateClusterOptions) (string, error) {
	options, err := validateCreateClusterOptions(options)
	if err != nil {
		return "", fmt.Errorf("cluster options validation failed: %v", err)
	}

	commandArgs := []string{"create", "cluster", "--output", "json", "--mode", "auto", "--yes"}
	commandArgs = append(commandArgs, "--cluster-name", options.ClusterName)
	commandArgs = append(commandArgs, "--channel-group", options.ChannelGroup)
	commandArgs = append(commandArgs, "--compute-machine-type", options.ComputeMachineType)
	commandArgs = append(commandArgs, "--machine-cidr", options.MachineCidr)
	commandArgs = append(commandArgs, "--region", r.awsCredentials.Region)
	commandArgs = append(commandArgs, "--version", options.Version)
	commandArgs = append(commandArgs, "--replicas", fmt.Sprint(options.Replicas))
	commandArgs = append(commandArgs, "--properties", options.Properties)
	commandArgs = append(commandArgs, "--controlplane-iam-role", options.accountRoles.controlPlaneRoleARN)
	commandArgs = append(commandArgs, "--role-arn", options.accountRoles.installerRoleARN)
	commandArgs = append(commandArgs, "--support-role-arn", options.accountRoles.supportRoleARN)
	commandArgs = append(commandArgs, "--worker-iam-role", options.accountRoles.workerRoleARN)

	if options.HostedCP {
		commandArgs = append(commandArgs, "--hosted-cp")
		commandArgs = append(commandArgs, "--oidc-config-id", options.oidcConfigID)
		commandArgs = append(commandArgs, "--subnet-ids", options.subnetIDs)
	}

	if options.STS {
		commandArgs = append(commandArgs, "--sts")
	}

	err = r.awsCredentials.CallFuncWithCredentials(ctx, func(ctx context.Context) error {
		_, _, err := cmd.Run(exec.CommandContext(ctx, "rosa", commandArgs...))
		return err
	})
	if err != nil {
		return "", err
	}

	cluster, err := r.getCluster(ctx, options.ClusterName)
	if err != nil {
		return "", err
	}

	return cluster.ID(), err
}

// getCluster gets the cluster the body
func (r *Provider) getCluster(ctx context.Context, clusterName string) (*clustersmgmtv1.Cluster, error) {
	query := fmt.Sprintf("product.id = 'rosa' AND name = '%s'", clusterName)
	response, err := r.ClustersMgmt().V1().Clusters().List().
		Search(query).
		Page(1).
		Size(1).
		SendContext(ctx)

	switch response.Total() {
	case 1:
		return response.Items().Slice()[0], nil
	default:
		return nil, fmt.Errorf("cluster %q not found: %v", clusterName, err)
	}
}

// deleteCluster handles sending the request to delete the cluster
func (r *Provider) deleteCluster(ctx context.Context, clusterID string) error {
	if clusterID == "" {
		return fmt.Errorf("cluster ID is undefined and is required")
	}

	commandArgs := []string{"delete", "cluster", "--cluster", clusterID, "--yes"}
	err := r.awsCredentials.CallFuncWithCredentials(ctx, func(ctx context.Context) error {
		_, _, err := cmd.Run(exec.CommandContext(ctx, "rosa", commandArgs...))
		return err
	})

	return err
}

// waitForClusterToBeReady waits for the cluster to be in a ready state
func (r *Provider) waitForClusterToBeReady(ctx context.Context, clusterID string, attempts int) error {
	getClusterState := func() (string, error) {
		var clusterState string

		commandArgs := []string{"describe", "cluster", "--cluster", clusterID, "--output", "json"}
		err := r.awsCredentials.CallFuncWithCredentials(ctx, func(ctx context.Context) error {
			stdout, _, err := cmd.Run(exec.CommandContext(ctx, "rosa", commandArgs...))
			if err != nil {
				return err
			}

			output, err := cmd.ConvertJSONStringToMap(stdout)
			if err != nil {
				return fmt.Errorf("failed to convert output to map: %v", err)
			}

			clusterState = fmt.Sprint(output["status"].(map[string]any)["state"])

			return nil
		})
		return clusterState, err
	}

	for i := 1; i <= attempts; i++ {
		clusterState, err := getClusterState()
		if err != nil {
			clusterState = "n/a"
		}

		if clusterState != "ready" {
			fmt.Printf("%d/%d : Cluster %q not in ready state (state=%s)\n", i, attempts, clusterID, clusterState)
			time.Sleep(1 * time.Minute)
			continue
		}

		fmt.Printf("Cluster id: %q is ready!", clusterID)
		return nil
	}

	return fmt.Errorf("cluster %q failed to enter ready state in the alloted attempts", clusterID)
}

// waitForClusterToBeDeleted waits for the cluster to be deleted
func (r *Provider) waitForClusterToBeDeleted(ctx context.Context, clusterName string, attempts int) error {
	for i := 1; i <= attempts; i++ {
		cluster, err := r.getCluster(ctx, clusterName)
		if err == nil && cluster != nil {
			fmt.Printf("%d/%d : Cluster %q is still uninstalling (state=%s)\n", i, attempts, clusterName, cluster.State())
			time.Sleep(1 * time.Minute)
			continue
		}

		fmt.Printf("Cluster %q no longer exists!", clusterName)
		return nil
	}

	return fmt.Errorf("cluster %q failed to finish uninstalling in the alloted attempts", clusterName)
}

// waitForClusterHealthChecksToSucceed waits for the cluster health check job to succeed
func (r *Provider) waitForClusterHealthChecksToSucceed() error {
	// TODO: Implement this
	return nil
}

// setDefaultCreateClusterOptions sets default options when creating clusters
func (o *CreateClusterOptions) setDefaultCreateClusterOptions() {
	if o.HostedCP {
		o.STS = true
	}
}

// setDefaultDeleteClusterOptions sets default options when creating clusters
func (o *DeleteClusterOptions) setDefaultDeleteClusterOptions() {
	if o.HostedCP {
		o.STS = true
	}
}
