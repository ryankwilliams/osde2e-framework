package rosa

import (
	"archive/tar"
	"compress/gzip"
	"runtime"

	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/openshift/osde2e-framework/internal/cmd"
	ocmclient "github.com/openshift/osde2e-framework/pkg/clients/ocm"
	awscloud "github.com/openshift/osde2e-framework/pkg/providers/clouds/aws"
)

const (
	minimumVersion = "1.2.22"
	tarFilename    = "rosa.tar.gz"
)

// Provider is a rosa provider
type Provider struct {
	*ocmclient.Client
	awsCredentials *awscloud.AWSCredentials
	rosaBinary     string
}

// providerError represents the provider custom error
type providerError struct {
	err error
}

// Error returns the formatted error message when providerError is invoked
func (r *providerError) Error() string {
	return fmt.Sprintf("failed to construct rosa provider: %v", r.err)
}

// cliExist checks if rosa cli is available else it will download it
func cliCheck() (string, error) {
	var (
		url          = fmt.Sprintf("https://mirror.openshift.com/pub/openshift-v4/clients/rosa/%s", minimumVersion)
		rosaFilename = fmt.Sprintf("%s/rosa", os.TempDir())
	)

	defer func() {
		_ = os.Remove(tarFilename)
	}()

	runtimeOS := runtime.GOOS
	switch runtimeOS {
	case "linux":
		url = fmt.Sprintf("%s/rosa-linux.tar.gz", url)
	case "darwin":
		url = fmt.Sprintf("%s/rosa-macosx.tar.gz", url)
	default:
		return "", fmt.Errorf("operating system %q is not supported", runtimeOS)
	}

	path, err := exec.LookPath("rosa")
	if path != "" && err == nil {
		return path, nil
	}

	response, err := http.Get(url)
	if err != nil || response.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("failed to download %s: %v", url, err)
	}
	defer response.Body.Close()

	tarFile, err := os.Create(tarFilename)
	if err != nil {
		return "", fmt.Errorf("failed to create %s tar file: %v", tarFilename, err)
	}
	defer tarFile.Close()

	rosaFile, err := os.Create(rosaFilename)
	if err != nil {
		return "", fmt.Errorf("failed to create %s tar file: %v", rosaFilename, err)
	}

	err = os.Chmod(rosaFilename, 0o755)
	if err != nil {
		return "", fmt.Errorf("failed to set file permissions to 0755 for %s: %v", rosaFilename, err)
	}

	defer rosaFile.Close()

	_, err = io.Copy(tarFile, response.Body)
	if err != nil {
		return "", fmt.Errorf("failed to write content to %s: %v", tarFilename, err)
	}

	tarFileReader, err := os.Open(tarFilename)
	if err != nil {
		return "", fmt.Errorf("failed to open %s: %v", tarFilename, err)
	}
	defer tarFileReader.Close()

	gzipReader, err := gzip.NewReader(tarFileReader)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader for %s: %v", tarFilename, err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	for {
		_, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			break
		}
		_, err = io.Copy(rosaFile, tarReader)
		if err != nil {
			break
		}
	}

	return rosaFilename, nil
}

// versionCheck verifies the rosa cli version meets the minimal version required
func versionCheck(ctx context.Context, rosaBinary string) error {
	stdout, _, err := cmd.Run(exec.CommandContext(ctx, rosaBinary, "version"))
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
func verifyCredentials(ctx context.Context, rosaBinary string, token, environment string, awsCredentials *awscloud.AWSCredentials) error {
	commandArgs := []string{"login", "--token", token, "--env", environment}

	return awsCredentials.CallFuncWithCredentials(ctx, func(ctx context.Context) error {
		_, _, err := cmd.Run(exec.CommandContext(ctx, rosaBinary, commandArgs...))
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

	rosaBinary, err := cliCheck()
	if err != nil {
		return nil, &providerError{err: err}
	}

	err = versionCheck(ctx, rosaBinary)
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

	err = verifyCredentials(ctx, rosaBinary, token, string(environment), awsCredentials)
	if err != nil {
		return nil, &providerError{err: err}
	}

	ocmClient, err := ocmclient.New(ctx, token, environment)
	if err != nil {
		return nil, &providerError{err: err}
	}

	return &Provider{
		awsCredentials: awsCredentials,
		rosaBinary:     rosaBinary,
		Client:         ocmClient,
	}, nil
}
