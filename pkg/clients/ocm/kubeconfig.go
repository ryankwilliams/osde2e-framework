package ocm

import (
	"context"
	"fmt"
	"os"
)

// KubeConfig returns the clusters kubeconfig content
func (c *Client) KubeConfig(ctx context.Context, clusterID string) (string, error) {
	response, err := c.ClustersMgmt().V1().Clusters().Cluster(clusterID).Credentials().Get().SendContext(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get credentials for cluster id %q: %v", clusterID, err)
	}
	return response.Body().Kubeconfig(), nil
}

// KubeConfigFile returns the clusters kubeconfig file
func (c *Client) KubeConfigFile(ctx context.Context, clusterID string) (string, error) {
	filename := fmt.Sprintf("%s-kubeconfig", clusterID)

	kubeConfig, err := c.KubeConfig(ctx, clusterID)
	if err != nil {
		return filename, err
	}

	err = os.WriteFile(filename, []byte(kubeConfig), 0o600)
	if err != nil {
		return filename, fmt.Errorf("failed to write kubeconfig file: %v", err)
	}

	return filename, nil
}
