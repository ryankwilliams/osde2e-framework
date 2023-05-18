package ocm

import (
	"context"
	"fmt"

	ocmsdk "github.com/openshift-online/ocm-sdk-go"
)

type Client struct {
	*ocmsdk.Connection
}

func New(ctx context.Context, token, environment string) (*Client, error) {
	url := "https://api.openshift.com"
	if environment == "stage" {
		url = "https://api.stage.openshift.com"
	}

	connection, err := ocmsdk.NewConnectionBuilder().
		URL(url).
		Tokens(token).
		BuildContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create ocm connection: %w", err)
	}

	return &Client{connection}, nil
}
