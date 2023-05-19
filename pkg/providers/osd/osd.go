package osd

import (
	"context"
	"fmt"

	ocmclient "github.com/openshift/osde2e-framework/pkg/clients/ocm"
)

// Provider contains the data required to work with ocm api
type Provider struct {
	*ocmclient.Client
}

// providerError contains the data to build a custom error for ocm provider
type providerError struct {
	err error
}

// Error creates the custom error for osd provider
func (o *providerError) Error() string {
	return fmt.Sprintf("failed to construct osd provider: %v", o.err)
}

// New constructs a osd provider and returns any errors encountered
// It is the callers responsibility to close the ocm connection when they are finished
// This can be done by closing the connection using defer `defer osdProvider.Client.Close()`
func New(ctx context.Context, token string, environment ocmclient.Environment) (*Provider, error) {
	if environment == "" || token == "" {
		return nil, &providerError{err: fmt.Errorf("one or more parameters are empty when invoking `New()`")}
	}

	ocmClient, err := ocmclient.New(ctx, token, environment)
	if err != nil {
		return nil, &providerError{err: err}
	}

	return &Provider{
		ocmClient,
	}, nil
}
