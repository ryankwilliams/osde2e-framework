package osd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/openshift/osde2e-framework/pkg/clients/kubernetes"

	"github.com/Masterminds/semver"
	clustersmgmtv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
)

const (
	managedUpgradeOperatorDeploymentName = "managed-upgrade-operator"
	managedUpgradeOperatorNamespace      = "openshift-managed-upgrade-operator"
	versionGateLabel                     = "api.openshift.com/gate-ocp"
	upgradeMaxAttempts                   = 1080
)

// upgradeError represents the cluster upgrade custom error
type upgradeError struct {
	err error
}

// Error returns the formatted error message when upgradeError is invokved
func (e *upgradeError) Error() string {
	return fmt.Sprintf("osd upgrade failed: %v", e.err)
}

// versionGates returns a list of available version gates from ocm
func (o *Provider) versionGates(ctx context.Context) (*clustersmgmtv1.VersionGateList, error) {
	response, err := o.ClustersMgmt().V1().VersionGates().List().SendContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to list version gates: %v", err)
	}

	return response.Items(), nil
}

// getVersionGateID returns the version gate agreement id
func (o *Provider) getVersionGateID(ctx context.Context, version string) (string, error) {
	versionGates, err := o.versionGates(ctx)
	if err != nil {
		return "", fmt.Errorf("unable to get version gate id for version %q: %v", version, err)
	}

	for _, versionGate := range versionGates.Slice() {
		if versionGate.VersionRawIDPrefix() == version && versionGate.Label() == versionGateLabel {
			return versionGate.ID(), nil
		}
	}

	return "", fmt.Errorf("version gate does not exist for version %q", version)
}

// getGateAgreement returns the gate agreement ocm resource
func (o *Provider) getGateAgreement(ctx context.Context, versionGateID string) (*clustersmgmtv1.VersionGate, error) {
	response, err := o.ClustersMgmt().V1().VersionGates().VersionGate(versionGateID).Get().SendContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get version gate agreement id %q: %v", versionGateID, err)
	}

	return response.Body(), nil
}

// gateAgreementExistForCluster checks to see if the version gate agreement id provided for the cluster already exists
func (o *Provider) gateAgreementExistForCluster(ctx context.Context, clusterID, gateAgreementID string) (bool, error) {
	response, err := o.ClustersMgmt().V1().Clusters().Cluster(clusterID).GateAgreements().List().SendContext(ctx)
	if err != nil {
		return false, fmt.Errorf("unable to get cluster id %q version gate agreements: %v", clusterID, err)
	}

	for _, gateAgreement := range response.Items().Slice() {
		if gateAgreement.VersionGate().ID() == gateAgreementID {
			log.Printf("Cluster gate agreement id: %s already exists", gateAgreementID)
			return true, nil
		}
	}

	return false, nil
}

// addGateAgreement adds a version gate agreement to the cluster ocm resource.
// Version gate agreement are used to acknowledge the cluster can be upgraded between versions
func (o *Provider) addGateAgreement(ctx context.Context, clusterID string, currentVersion, upgradeVersion semver.Version) error {
	if !(currentVersion.Minor() < upgradeVersion.Minor()) {
		log.Println("No gate agreement is required for z-stream upgrade.")
		return nil
	}

	majorMinor := fmt.Sprintf("%d.%d", upgradeVersion.Major(), upgradeVersion.Minor())

	versionGateID, err := o.getVersionGateID(ctx, majorMinor)
	if err != nil {
		return err
	}

	exist, err := o.gateAgreementExistForCluster(ctx, clusterID, versionGateID)
	if err != nil {
		return err
	}
	if exist {
		return nil
	}

	gateAgreement, err := o.getGateAgreement(ctx, versionGateID)
	if err != nil {
		return err
	}

	versionGateAgreement, err := clustersmgmtv1.NewVersionGateAgreement().
		VersionGate(clustersmgmtv1.NewVersionGate().Copy(gateAgreement)).
		Build()
	if err != nil {
		return fmt.Errorf("building version gate agreement for cluster id %q failed: %v", clusterID, err)
	}

	_, err = o.ClustersMgmt().V1().Clusters().Cluster(clusterID).GateAgreements().Add().Body(versionGateAgreement).SendContext(ctx)
	if err != nil {
		return fmt.Errorf("applying version gate agreement to cluster id %q failed: %v", clusterID, err)
	}

	return nil
}

// initiateUpgrade initiates the upgrade for the cluster with ocm by applying a upgrade policy to the cluster
func (o *Provider) initiateUpgrade(ctx context.Context, clusterID, version string) error {
	upgradePolicy, err := clustersmgmtv1.NewUpgradePolicy().Version(version).
		NextRun(time.Now().UTC().Add(7 * time.Minute)).
		ScheduleType("manual").Build()
	if err != nil {
		return fmt.Errorf("unable to build upgrade policy for cluster id %q: %v", clusterID, err)
	}

	response, err := o.ClustersMgmt().V1().Clusters().Cluster(clusterID).UpgradePolicies().Add().Body(upgradePolicy).SendContext(ctx)
	if err != nil || response.Status() != http.StatusCreated {
		return fmt.Errorf("applying upgrade policy to cluster id %q failed: %v", clusterID, err)
	}

	log.Printf("Cluster id %q upgrade to version %q has been scheduled for %s\n", clusterID, response.Body().Version(), response.Body().NextRun().Format(time.RFC3339))

	return nil
}

// restartManagedUpgradeOperator scales down/up the muo operator to speed up the cluster upgrade start time
func (o *Provider) restartManagedUpgradeOperator(ctx context.Context, client *kubernetes.Client) error {
	// TODO fix this
	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: managedUpgradeOperatorDeploymentName, Namespace: managedUpgradeOperatorNamespace}}
	err := wait.For(conditions.New(client.Resources).DeploymentConditionMatch(deployment, appsv1.DeploymentAvailable, corev1.ConditionTrue))
	if err != nil {
		return fmt.Errorf("unable to get managed upgrade operator deployment: %v", err)
	}

	deployment.Spec.Replicas = pointer.Int32(0)
	err = client.Update(ctx, deployment)
	if err != nil {
		return fmt.Errorf("scale down %s deployment replicas to 0 failed: %v", managedUpgradeOperatorDeploymentName, err)
	}

	deployment.Spec.Replicas = pointer.Int32(1)
	err = client.Update(ctx, deployment)
	if err != nil {
		return fmt.Errorf("scale up %s deployment replicas to 1 failed: %v", managedUpgradeOperatorDeploymentName, err)
	}

	log.Println("Successfully restarted managed upgrade operator")

	return nil
}

// managedUpgradeConfigExist waits/checks for the muo upgrade config to exist on the cluster
func (o *Provider) managedUpgradeConfigExist(ctx context.Context, dynamicClient *dynamic.DynamicClient) error {
	for i := 1; i <= 6; i++ {
		upgradeConfig, err := getManagedUpgradeOperatorConfig(ctx, dynamicClient)
		if err != nil || upgradeConfig == nil {
			time.Sleep(30 * time.Second)
			continue
		}
		return nil
	}

	return fmt.Errorf("managed upgrade config never existed on the cluster")
}

// TODO Come back and revise/retest, initial first draft/port
// OCMUpgrade handles the end to end process to upgrade an openshift dedicated cluster
func (o *Provider) OCMUpgrade(ctx context.Context, client *kubernetes.Client, clusterID string, currentVersion, upgradeVersion semver.Version) error {
	dynamicClient, err := getKubernetesDynamicClient(client)
	if err != nil {
		return &upgradeError{err: err}
	}

	err = o.addGateAgreement(ctx, clusterID, currentVersion, upgradeVersion)
	if err != nil {
		return &upgradeError{err: err}
	}

	err = o.initiateUpgrade(ctx, clusterID, upgradeVersion.String())
	if err != nil {
		return &upgradeError{err: err}
	}

	err = o.restartManagedUpgradeOperator(ctx, client)
	if err != nil {
		return &upgradeError{err: err}
	}

	err = o.managedUpgradeConfigExist(ctx, dynamicClient)
	if err != nil {
		return &upgradeError{err: err}
	}

	for i := 1; i <= upgradeMaxAttempts; i++ {
		var upgradeStatus string
		var conditionMessage string

		upgradeConfig, err := getManagedUpgradeOperatorConfig(ctx, dynamicClient)
		if err != nil {
			time.Sleep(10 * time.Second)
			continue
		}

		status, found, err := unstructured.NestedMap(upgradeConfig.Object, "status")
		if !found || err != nil {
			time.Sleep(10 * time.Second)
			continue
		}

		histories, found, err := unstructured.NestedSlice(status, "history")
		if !found || err != nil {
			time.Sleep(10 * time.Second)
			continue
		}

		for _, h := range histories {
			version, found, err := unstructured.NestedString(h.(map[string]interface{}), "version")
			if !found || err != nil {
				time.Sleep(10 * time.Second)
				continue
			}

			if version == upgradeVersion.String() {
				upgradeStatus, found, err = unstructured.NestedString(h.(map[string]interface{}), "phase")
				if !found || err != nil {
					time.Sleep(10 * time.Second)
					continue
				}

				conditions, found, err := unstructured.NestedSlice(h.(map[string]interface{}), "conditions")
				if !found || err != nil {
					time.Sleep(10 * time.Second)
					continue
				}

				conditionMessage, found, err = unstructured.NestedString(conditions[0].(map[string]interface{}), "message")
				if !found || err != nil {
					time.Sleep(10 * time.Second)
					continue
				}

				break
			}
			_ = h
		}

		if upgradeStatus == "" {
			log.Println("Upgrade has not started yet")
			time.Sleep(10 * time.Second)
			continue
		}

		if upgradeStatus == "Failed" {
			log.Printf("Upgrade %q, %s", upgradeStatus, conditionMessage)
			return fmt.Errorf("upgrade failed")
		}

		if upgradeStatus != "Upgraded" {
			log.Printf("Upgrade is %q, %s\n", upgradeStatus, conditionMessage)
			time.Sleep(10 * time.Second)
			continue
		}
	}

	return nil
}

// getKubernetesDynamicClient returns the kubernetes dynamic client
func getKubernetesDynamicClient(client *kubernetes.Client) (*dynamic.DynamicClient, error) {
	dynamicClient, err := dynamic.NewForConfig(client.GetConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes dynamic client: %w", err)
	}
	return dynamicClient, nil
}

// getManagedUpgradeOperatorConfig returns the upgrade config object
func getManagedUpgradeOperatorConfig(ctx context.Context, dynamicClient *dynamic.DynamicClient) (*unstructured.Unstructured, error) {
	upgradeConfigs, err := dynamicClient.Resource(
		schema.GroupVersionResource{
			Group:    "upgrade.managed.openshift.io",
			Version:  "v1alpha1",
			Resource: "upgradeconfigs",
		},
	).Namespace(managedUpgradeOperatorNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return &upgradeConfigs.Items[0], nil
}
