package gomegamatchers

import (
	"fmt"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

type beAvailableMatcher struct{}

func BeAvailable() types.GomegaMatcher {
	return &beAvailableMatcher{}
}

func (d *beAvailableMatcher) Match(actual any) (bool, error) {
	deployment, ok := actual.(*appsv1.Deployment)
	if !ok {
		return false, fmt.Errorf("BeAvailable expected an appsv1.Deployment object but got %s", format.Object(actual, 1))
	}
	for _, cond := range deployment.Status.Conditions {
		if cond.Type == appsv1.DeploymentAvailable {
			return cond.Status == corev1.ConditionTrue, nil
		}
	}
	return false, nil
}

func (d *beAvailableMatcher) FailureMessage(actual any) string {
	deployment, ok := actual.(*appsv1.Deployment)
	if !ok {
		return fmt.Sprintf("wanted an appsv1.Deployment but got %s", format.Object(actual, 1))
	}
	return format.Message(deployment.Status.Conditions, "deployment to be available")
}

func (d *beAvailableMatcher) NegatedFailureMessage(actual any) string {
	deployment, ok := actual.(*appsv1.Deployment)
	if !ok {
		return fmt.Sprintf("wanted an appsv1.Deployment but got %s", format.Object(actual, 1))
	}
	return format.Message(deployment.Status.Conditions, "deployment to not be available")
}
