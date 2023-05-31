package gomegamatchers

import (
	"errors"
	"fmt"
	"strings"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

type containItemWithPrefixMatcher struct {
	prefix string
}

// ContainItemWithPrefix is a gomega matcher that can be used to assert that a
// Kubernetes list object contains an item name with the provided prefix
//
//	var rolebindings rbacv1.RoleBindingList
//	err = k8s.List(ctx, &rolebindings)
//	Expect(err).ShouldNot(HaveOccurred(), "failed to list rolebindings")
//	Expect(roleBindingsList).Should(ContainItemWithPrefix("test"))
func ContainItemWithPrefix(prefix string) types.GomegaMatcher {
	return &containItemWithPrefixMatcher{prefix}
}

func (matcher *containItemWithPrefixMatcher) Match(actual any) (bool, error) {
	obj, ok := actual.(runtime.Object)
	if !ok {
		return false, errors.New("type must be a runtime.Object")
	}
	items, err := meta.ExtractList(obj)
	if err != nil {
		return false, fmt.Errorf("not a list type: %w", err)
	}
	for _, item := range items {
		accessor, err := meta.Accessor(item)
		if err != nil {
			return false, fmt.Errorf("unable to get item's objectmeta: %w", err)
		}
		if strings.HasPrefix(accessor.GetName(), matcher.prefix) {
			return true, nil
		}
	}
	return false, nil
}

func (matcher *containItemWithPrefixMatcher) FailureMessage(actual any) string {
	return format.Message(actual, fmt.Sprintf("did not contain item with prefix %s", matcher.prefix))
}

func (l *containItemWithPrefixMatcher) NegatedFailureMessage(actual any) string {
	return format.Message(actual, "not a list")
}
