package rosa

import (
	"context"
	"fmt"
	"os"

	ocmclient "github.com/openshift/osde2e-framework/pkg/clients/ocm"
)

// RosaProvider contains the data to perform rosa operations
type ROSAProvider struct {
	*ocmclient.Client
	awsCredentials *AWSCredentials
}

// AWSCredentials contains the data to be used to authenticate with aws
type AWSCredentials struct {
	AccessKeyID     string
	Profile         string
	Region          string
	SecretAccessKey string
}

// rosaProviderError contains the data to build a custom error for rosa provider
type rosaProviderError struct {
	err error
}

// Error creates the custom error for rosa provider
func (r *rosaProviderError) Error() string {
	return fmt.Sprintf("failed to construct rosa provider: %v", r.err)
}

// validateAndFetchAWSCredentials validates the aws credentials/ensures they are set
// Data can be passed as a parameter or fetched from the environment
// It returns the aws auth data collection and any errors encountered
func validateAndFetchAWSCredentials(credentials AWSCredentials) (*AWSCredentials, error) {
	if credentials == (AWSCredentials{}) {
		credentials.AccessKeyID = os.Getenv("AWS_ACCESS_KEY_ID")
		credentials.Profile = os.Getenv("AWS_PROFILE")
		credentials.Region = os.Getenv("AWS_REGION")
		credentials.SecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	}

	setByAccessKeys := true
	setByProfile := true

	if credentials.AccessKeyID == "" || credentials.SecretAccessKey == "" {
		setByAccessKeys = false
	}

	if credentials.Profile == "" {
		setByProfile = false
	}

	if !setByAccessKeys && !setByProfile {
		return nil, fmt.Errorf("credentials are not supplied")
	}

	if credentials.Region == "" {
		return nil, fmt.Errorf("region is not supplied")
	}

	return nil, nil
}

// New constructs a rosa provider and returns any errors encountered
// It is the callers responsibility to close the ocm connection when they are finished
// This can be done by closing the connection using defer `defer rosaProvider.Client.Close()`
func New(ctx context.Context, token, environment string, args ...any) (*ROSAProvider, error) {
	if environment == "" || token == "" {
		return nil, &rosaProviderError{err: fmt.Errorf("one or more parameters are empty when invoking `New()`")}
	}

	creds := AWSCredentials{}
	if len(args) == 1 {
		creds = args[0].(AWSCredentials)
	} else if len(args) > 1 {
		return nil, &rosaProviderError{err: fmt.Errorf("only one AWSCredentials can be provided")}
	}

	awsCredentials, err := validateAndFetchAWSCredentials(creds)
	if err != nil {
		return nil, &rosaProviderError{err: fmt.Errorf("aws authentication data check failed: %v", err)}
	}

	// TODO: Implement locating rosa binary and possibly downloading, needs a version check as well
	// TODO: Perform rosa login to validate credentials/ocm token

	ocmClient, err := ocmclient.New(ctx, token, environment)
	if err != nil {
		return nil, &rosaProviderError{err: err}
	}

	return &ROSAProvider{
		awsCredentials: awsCredentials,
		Client:         ocmClient,
	}, nil
}
