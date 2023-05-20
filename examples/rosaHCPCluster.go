package main

import (
	"context"
	"fmt"
	"os"

	ocmclient "github.com/openshift/osde2e-framework/pkg/clients/ocm"
	awscloud "github.com/openshift/osde2e-framework/pkg/providers/clouds/aws"
	"github.com/openshift/osde2e-framework/pkg/providers/rosa"
)

func main() {
	// These MUST be set
	var (
		action        = "create || delete"
		awsProfile    = ""
		awsRegion     = ""
		channelGroup  = "candidate"
		clusterName   = ""
		clusterID     = ""
		ocmEnviroment = ocmclient.Stage
		ocmToken      = os.Getenv("OCM_TOKEN")
		version       = "4.12.6"
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

	defer func() {
		_ = provider.Connection.Close()
	}()

	if action == "create" {
		_, err := provider.CreateCluster(
			ctx,
			&rosa.CreateClusterOptions{
				ClusterName:  clusterName,
				Version:      version,
				ChannelGroup: channelGroup,
				HostedCP:     true,
			},
		)
		if err != nil {
			panic(fmt.Sprintf("Failed to create rosa hcp cluster: %v", err))
		}
	} else if action == "delete" {
		err := provider.DeleteCluster(
			ctx,
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
