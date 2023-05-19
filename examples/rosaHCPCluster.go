package main

import (
	"context"
	"fmt"
	"os"

	ocmclient "github.com/openshift/osde2e-framework/pkg/clients/ocm"
	awscloud "github.com/openshift/osde2e-framework/pkg/providers/clouds/aws"
	"github.com/openshift/osde2e-framework/pkg/providers/rosa"
)

func create(ctx context.Context, provider *rosa.Provider, options *rosa.CreateClusterOptions) (string, error) {
	return provider.CreateCluster(ctx, options)
}

func delete(ctx context.Context, provider *rosa.Provider, options *rosa.DeleteClusterOptions) error {
	return provider.DeleteCluster(ctx, options)
}

func main() {
	// These MUST be set
	var (
		action           = "create || delete"
		awsProfile       = ""
		awsRegion        = ""
		channelGroup     = "candidate"
		clusterName      = ""
		clusterID        = ""
		installerRoleARN = ""
		ocmEnviroment    = ocmclient.Stage
		ocmToken         = os.Getenv("OCM_TOKEN")
		version          = "4.12.6"
	)

	ctx := context.Background()

	provider, err := rosa.New(
		ctx,
		ocmToken,
		ocmEnviroment,
		&awscloud.AWSCredentials{Profile: awsProfile, Region: awsRegion},
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to create rosa provider: %v", err))
	}

	if action == "create" {
		_, err := create(
			ctx,
			provider,
			&rosa.CreateClusterOptions{
				ClusterName:      clusterName,
				InstallerRoleArn: installerRoleARN,
				Version:          version,
				ChannelGroup:     channelGroup,
				HostedCP:         true,
			},
		)
		if err != nil {
			panic(fmt.Sprintf("Failed to create rosa hcp cluster: %v", err))
		}
	} else if action == "delete" {
		err := delete(
			ctx,
			provider,
			&rosa.DeleteClusterOptions{
				ClusterName: clusterName,
				ClusterID:   clusterID,
				HostedCP:    true,
			},
		)
		if err != nil {
			panic(fmt.Sprintf("Failed to delete rosa hcp cluster: %v", err))
		}
	} else {
		panic(fmt.Sprintf("Action %q not supported", action))
	}
}
