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

// Provider is a rosa provider
type Provider struct {
	*ocmclient.Client
	awsCredentials *awscloud.AWSCredentials
}

// providerError represents the provider custom error
type providerError struct {
	err error
}

// Error returns the formatted error message when providerError is invoked
func (r *providerError) Error() string {
	return fmt.Sprintf("failed to construct rosa provider: %v", r.err)
}

// cliExist checks whether the rosa cli is available on the file system
func cliExist() error {
	_, err := exec.LookPath("rosa")
	return err
}

// versionCheck verifies the rosa cli version meets the minimal version required
func versionCheck(ctx context.Context) error {
	stdout, _, err := cmd.Run(exec.CommandContext(ctx, "rosa", "version"))
	if err != nil {
		return err
	}

	versionSlice := strings.SplitAfter(fmt.Sprint(stdout), "\n")
	if len(versionSlice) == 0 {
		return fmt.Errorf("versionCheck failed to get version from cli standard out")
	}

	currentVersion, err := semver.NewVersion(strings.ReplaceAll(versionSlice[0], "\n", ""))
	if err != nil {
		return fmt.Errorf("versionCheck failed to parse version to semantic version: %v", err)
	}

	minVersion, err := semver.NewVersion(minimumVersion)
	if err != nil {
		return fmt.Errorf("versionCheck failed to parse minimum version to semantic version: %v", err)
	}

	if minVersion.Compare(currentVersion) == 1 {
		return fmt.Errorf("current rosa version is %q and must be >= %q", currentVersion.String(), minVersion)
	}

	return nil
}

// verifyCredentials validates the ocm token and aws credentials to ensure they are valid
func verifyCredentials(ctx context.Context, token, environment string, awsCredentials *awscloud.AWSCredentials) error {
	commandArgs := []string{"login", "--token", token, "--env", environment}

	return awsCredentials.CallFuncWithCredentials(ctx, func(ctx context.Context) error {
		_, _, err := cmd.Run(exec.CommandContext(ctx, "rosa", commandArgs...))
		if err != nil {
			return fmt.Errorf("login failed %v", err)
		}
		return nil
	})
}

// New handles constructing the rosa provider which creates a connection
// to openshift cluster manager "ocm". It is the callers responsibility
// to close the ocm connection when they are finished (defer provider.Connection.Close())
func New(ctx context.Context, token string, environment ocmclient.Environment, args ...any) (*Provider, error) {
	if environment == "" || token == "" {
		return nil, &providerError{err: fmt.Errorf("some parameters are undefined, unable to construct osd provider")}
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

	err = verifyCredentials(ctx, token, string(environment), awsCredentials)
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
