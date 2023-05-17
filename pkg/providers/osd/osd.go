package ocm

import (
	"context"
	"fmt"

	ocmclient "github.com/openshift/osde2e-framework/pkg/clients/ocm"
)

// OSDProvider contains the data required to work with ocm api
type OSDProvider struct {
	*ocmclient.Client
}

// osdProviderError contains the data to build a custom error for ocm provider
type osdProviderError struct {
	err error
}

// Error creates the custom error for osd provider
func (o *osdProviderError) Error() string {
	return fmt.Sprintf("failed to construct osd provider: %v", o.err)
}

// New constructs a osd provider and returns any errors encountered
// It is the callers responsibility to close the ocm connection when they are finished
// This can be done by closing the connection using defer `defer osdProvider.Client.Close()`
func New(ctx context.Context, token, environment string) (*OSDProvider, error) {
	if environment == "" || token == "" {
		return nil, &osdProviderError{err: fmt.Errorf("one or more parameters are empty when invoking `New()`")}
	}

	ocmClient, err := ocmclient.New(ctx, token, environment)
	if err != nil {
		return nil, &osdProviderError{err: err}
	}

	return &OSDProvider{
		ocmClient,
	}, nil
}
