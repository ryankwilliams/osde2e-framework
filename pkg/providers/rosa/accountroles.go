package rosa

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/openshift/osde2e-framework/internal/cmd"
)

type accountRoles struct {
	controlPlaneRoleARN string
	installerRoleARN    string
	supportRoleARN      string
	workerRoleARN       string
}

type accountRolesError struct {
	action string
	err    error
}

func (a *accountRolesError) Error() string {
	return fmt.Sprintf("%s account roles failed: %v", a.action, a.err)
}

// createAccountRoles creates the account roles to be used when creating rosa clusters
func (r *Provider) createAccountRoles(ctx context.Context, prefix, version, channelGroup string) (*accountRoles, error) {
	const action = "create"
	var accountRoles *accountRoles

	accountRoles, err := r.getAccountRoles(ctx, prefix, version)
	if err != nil {
		return nil, &accountRolesError{action: action, err: err}
	}

	// TODO: Open an RFE to rosa to support --output option
	if accountRoles == nil {
		log.Printf("Creating account roles with prefix/version \"%s/%s\n", prefix, version)

		commandArgs := []string{
			"create",
			"account-roles",
			"--prefix",
			prefix,
			"--version",
			version,
			"--channel-group",
			channelGroup,
			"--mode",
			"auto",
			"--yes",
		}

		err := r.awsCredentials.CallFuncWithCredentials(ctx, func(ctx context.Context) error {
			_, _, err := cmd.Run(exec.CommandContext(ctx, "rosa", commandArgs...))
			if err != nil {
				return err
			}

			accountRoles, err = r.getAccountRoles(ctx, prefix, version)
			if err != nil {
				return fmt.Errorf("unable to get account roles post account roles creation: %v", err)
			}

			return nil
		})
		if err != nil {
			return nil, &accountRolesError{action: action, err: err}
		}

		log.Printf("Account roles created with prefix/version \"%s/%s\n", prefix, version)

		return accountRoles, nil
	}

	log.Printf("Account roles already exist with prefix/version \"%s/%s\n", prefix, version)

	return accountRoles, nil
}

// deleteAccountRoles deletes the account roles that were created to create rosa clusters
func (r *Provider) deleteAccountRoles(ctx context.Context, prefix string) error {
	log.Printf("Deleting account roles with prefix %q", prefix)

	commandArgs := []string{"delete", "account-roles", "--prefix", prefix, "--mode", "auto", "--yes"}

	err := r.awsCredentials.CallFuncWithCredentials(ctx, func(ctx context.Context) error {
		_, _, err := cmd.Run(exec.CommandContext(ctx, "rosa", commandArgs...))
		return err
	})
	if err != nil {
		return &accountRolesError{action: "delete", err: err}
	}

	log.Printf("Account roles with prefix %q deleted!", prefix)

	return nil
}

// getAccountRoles gets the account roles matching the provided prefix and version
func (r *Provider) getAccountRoles(ctx context.Context, prefix, version string) (*accountRoles, error) {
	var (
		availableAccountRoles []map[string]any
		accountRolesFound     = 0
		roles                 = &accountRoles{}
	)

	commandArgs := []string{"list", "account-roles", "--output", "json"}

	err := r.awsCredentials.CallFuncWithCredentials(ctx, func(ctx context.Context) error {
		stdout, _, err := cmd.Run(exec.CommandContext(ctx, "rosa", commandArgs...))
		if err != nil {
			return err
		}

		output, err := cmd.ConvertJSONStringToListOfMaps(stdout)
		if err != nil {
			return fmt.Errorf("failed to convert output to map: %v", err)
		}

		availableAccountRoles = output

		return nil
	})
	if err != nil {
		return nil, err
	}

	for _, accountRole := range availableAccountRoles {
		roleName := fmt.Sprint(accountRole["RoleName"])
		roleARN := fmt.Sprint(accountRole["RoleARN"])
		roleVersion := fmt.Sprint(accountRole["Version"])
		roleType := fmt.Sprint(accountRole["RoleType"])

		if !strings.HasPrefix(roleName, prefix) {
			continue
		}

		if version != roleVersion {
			continue
		}

		switch roleType {
		case "Control plane":
			roles.controlPlaneRoleARN = roleARN
			accountRolesFound += 1
		case "Installer":
			roles.installerRoleARN = roleARN
			accountRolesFound += 1
		case "Support":
			roles.supportRoleARN = roleARN
			accountRolesFound += 1
		case "Worker":
			roles.workerRoleARN = roleARN
			accountRolesFound += 1
		}

	}

	switch {
	case accountRolesFound == 0:
		return nil, nil
	case accountRolesFound != 4:
		return nil, fmt.Errorf("one or more prefixed %q account roles does not exist: %+v", prefix, roles)
	default:
		return roles, nil
	}
}
