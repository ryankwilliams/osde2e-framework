package ocm

import (
	"context"
	"fmt"

	ocmsdk "github.com/openshift-online/ocm-sdk-go"
)

const (
	Production  = "https://api.openshift.com"
	Stage       = "https://api.stage.openshift.com"
	Integration = "https://api.integration.openshift.com"
)

type Client struct {
	*ocmsdk.Connection
}

func New(ctx context.Context, token, environment string) (*Client, error) {
	connection, err := ocmsdk.NewConnectionBuilder().
		URL(environment).
		Tokens(token).
		BuildContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create ocm connection: %w", err)
	}

	return &Client{connection}, nil
}
