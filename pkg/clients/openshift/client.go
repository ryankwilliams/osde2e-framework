package openshift

import (
	"fmt"

	"github.com/openshift/api"
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
	if err = api.Install(client.GetScheme()); err != nil {
		return nil, fmt.Errorf("unable to register openshift api schemes: %w", err)
	}
	return &Client{client}, nil
}
