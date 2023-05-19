package rosa

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/openshift/osde2e-framework/internal/cmd"
	ocmclient "github.com/openshift/osde2e-framework/pkg/clients/ocm"
	awscloud "github.com/openshift/osde2e-framework/pkg/providers/clouds/aws"
)

const minimumVersion = "1.2.21"

// RosaProvider contains the data to perform rosa operations
type Provider struct {
	*ocmclient.Client
	awsCredentials *awscloud.AWSCredentials
}

// providerError contains the data to build a custom error for rosa provider
type providerError struct {
	err error
}

// Error creates the custom error for rosa provider
func (r *providerError) Error() string {
	return fmt.Sprintf("failed to construct rosa provider: %v", r.err)
}

// cliExist checks whether the rosa cli is available on the file system
func cliExist() error {
	_, err := exec.LookPath("rosa")
	return err
}

// versionCheck validates whether the rosa cli available in path meets the
// minimal version required
func versionCheck(ctx context.Context) error {
	stdout, _, err := cmd.Run(exec.CommandContext(ctx, "rosa", "version"))
	if err != nil {
		return err
	}

	currentVerison, err := semver.NewVersion(strings.ReplaceAll(fmt.Sprint(stdout), "\n", ""))
	if err != nil {
		return fmt.Errorf("failed to parse rosa version %q into semantic version: %v", currentVerison.String(), err)
	}

	minVersion, err := semver.NewVersion(minimumVersion)
	if err != nil {
		return fmt.Errorf("failed to parse minimum rosa version %q into semantic version: %v", minimumVersion, err)
	}

	if minVersion.Compare(currentVerison) == 1 {
		return fmt.Errorf("current rosa version is %q and must be >= %q", currentVerison.String(), minVersion)
	}

	return nil
}

// validateLogin validates the token/aws credentials provided are valid
func validateLogin(ctx context.Context, token, environment string, awsCredentials *awscloud.AWSCredentials) error {
	commandArgs := []string{"login", "--token", token, "--env", environment}

	err := awsCredentials.CallFuncWithCredentials(func() error {
		_, _, err := cmd.Run(exec.CommandContext(ctx, "rosa", commandArgs...))
		if err != nil {
			return fmt.Errorf("login failed %v", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// New constructs a rosa provider and returns any errors encountered
// It is the callers responsibility to close the ocm connection when they are finished
// This can be done by closing the connection using defer `defer rosaProvider.Client.Close()`
func New(ctx context.Context, token string, environment ocmclient.Environment, args ...any) (*Provider, error) {
	if environment == "" || token == "" {
		return nil, &providerError{err: fmt.Errorf("one or more parameters are empty when invoking `New()`")}
	}

	// TODO: Implement downloading rosa cli when not found in path
	err := cliExist()
	if err != nil {
		return nil, &providerError{err: err}
	}

	err = versionCheck(ctx)
	if err != nil {
		return nil, &providerError{err: err}
	}

	awsCredentials := &awscloud.AWSCredentials{}
	if len(args) == 1 {
		awsCredentials = args[0].(*awscloud.AWSCredentials)
	} else if len(args) > 1 {
		return nil, &providerError{err: fmt.Errorf("only one AWSCredentials can be provided")}
	}

	err = awsCredentials.ValidateAndFetchCredentials()
	if err != nil {
		return nil, &providerError{err: fmt.Errorf("aws authentication data check failed: %v", err)}
	}

	err = validateLogin(ctx, token, string(environment), awsCredentials)
	if err != nil {
		return nil, &providerError{err: err}
	}

	ocmClient, err := ocmclient.New(ctx, token, environment)
	if err != nil {
		return nil, &providerError{err: err}
	}

	return &Provider{
		awsCredentials: awsCredentials,
		Client:         ocmClient,
	}, nil
}
