package rosa

import (
	"context"
	"fmt"
	"log"
	"os/exec"

	"github.com/openshift/osde2e-framework/internal/cmd"

	clustersmgmtv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
)

type CreateClusterOptions struct {
	ChannelGroup       string
	ClusterName        string
	ComputeMachineType string
	HostedCP           bool
	InstallerRoleArn   string
	MachineCidr        string
	Mode               string
	OIDCConfigManaged  bool
	Properties         string
	Replicas           int
	STS                bool
	Version            string

	oidcConfigID string
	subnetIDs    string
}

type DeleteClusterOptions struct {
	ClusterID   string
	ClusterName string
	HostedCP    bool
	STS         bool
}

type clusterError struct {
	action string
	err    error
}

func (c *clusterError) Error() string {
	return fmt.Sprintf("%s cluster failed: %v", c.action, c.err)
}

// CreateCluster creates a rosa cluster using the provided inputs
func (r *Provider) CreateCluster(ctx context.Context, options *CreateClusterOptions) error {
	const action = "create"

	defer func() {
		_ = r.Connection.Close()
	}()

	if options.HostedCP {
		// TODO: region check for hcp support

		oidcConfigID, err := r.createOIDCConfig(
			ctx,
			options.ClusterName,
			options.InstallerRoleArn,
			options.OIDCConfigManaged,
		)
		if err != nil {
			return &clusterError{action: action, err: err}
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
			return &clusterError{action: action, err: err}
		}

		options.subnetIDs = fmt.Sprintf("%s,%s", vpc.privateSubnet, vpc.publicSubnet)
	}

	clusterID, err := r.createCluster(ctx, options)
	if err != nil {
		return &clusterError{action: action, err: err}
	}

	log.Printf("Cluster ID: %s\n", clusterID)

	err = r.waitForClusterToBeReady()
	if err != nil {
		return &clusterError{action: action, err: err}
	}

	err = r.waitForClusterHealthChecksToSucceed()
	if err != nil {
		return &clusterError{action: action, err: err}
	}

	return nil
}

// DeleteCluster deletes a rosa cluster using the provided inputs
func (r *Provider) DeleteCluster(ctx context.Context, options *DeleteClusterOptions) error {
	const action = "delete"
	var oidcConfigID string

	defer func() {
		_ = r.Connection.Close()
	}()

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

	// TODO: Wait for cluster to be deleted, have the code, just need to add it

	if options.STS {
		// TODO: Delete operator roles, have the code, just need to add it

		// TODO: Delete oidc config provider, have the code, just need to add it
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

	return nil
}

// validateCreateClusterOptions verifies required create cluster options are set
// and sets defaults for ones undefined
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
	commandArgs = append(commandArgs, "--properties", options.Version)

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
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve cluster: %v", err)
	}

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
func (r *Provider) waitForClusterToBeReady() error {
	// TODO: Finish, have the code, just need to add it
	return nil
}

// waitForClusterHealthChecksToSucceed waits for the cluster health check job to succeed
func (r *Provider) waitForClusterHealthChecksToSucceed() error {
	// TODO: Implement this
	return nil
}
