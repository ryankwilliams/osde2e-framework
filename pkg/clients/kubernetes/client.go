package kubernetes

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
)

type Client struct {
	*resources.Resources
}

func New() (*Client, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes config: %w", err)
	}
	client, err := resources.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to created dynamic client: %w", err)
	}
	return &Client{client}, nil
}
