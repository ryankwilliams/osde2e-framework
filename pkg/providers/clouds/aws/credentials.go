package aws

import (
	"context"
	"fmt"
	"os"
)

// AWSCredentials contains the data to be used to authenticate with aws
type AWSCredentials struct {
	AccessKeyID     string
	Profile         string
	Region          string
	SecretAccessKey string
}

// priority determines the priority of which credentials are used
func (c *AWSCredentials) priority() (int, error) {
	switch {
	case c.Profile != "":
		return 0, nil
	case c.AccessKeyID != "" && c.SecretAccessKey != "":
		return 1, nil
	}

	return -1, fmt.Errorf("no credentials are set, unable to determine priority")
}

// ValidateAndFetchCredentials validates the aws credentials/ensures they are set
// Data can be passed as a parameter or fetched from the environment
func (c *AWSCredentials) ValidateAndFetchCredentials() error {
	if *c == (AWSCredentials{}) {
		c.AccessKeyID = os.Getenv("AWS_ACCESS_KEY_ID")
		c.Profile = os.Getenv("AWS_PROFILE")
		c.Region = os.Getenv("AWS_REGION")
		c.SecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	}

	setByAccessKeys := true
	setByProfile := true

	if c.AccessKeyID == "" || c.SecretAccessKey == "" {
		setByAccessKeys = false
	}

	if c.Profile == "" {
		setByProfile = false
	}

	if !setByAccessKeys && !setByProfile {
		return fmt.Errorf("credentials are not supplied")
	}

	if c.Region == "" {
		return fmt.Errorf("region is not supplied")
	}

	return nil
}

// CallFuncWithCredentials injects aws credentials into the environment
// and calls the function provided
func (c *AWSCredentials) CallFuncWithCredentials(ctx context.Context, f func(ctx context.Context) error) error {
	priorityLevel, err := c.priority()
	if err != nil {
		return err
	}

	// TODO: Should we unset these environment variables when done (defer func..)?

	os.Setenv("AWS_REGION", c.Region)

	if priorityLevel == 0 {
		os.Setenv("AWS_PROFILE", c.Profile)
	} else if priorityLevel == 1 {
		os.Setenv("AWS_ACCESS_KEY_ID", c.AccessKeyID)
		os.Setenv("AWS_SECRET_ACCESS_KEY", c.SecretAccessKey)
	}

	return f(ctx)
}
