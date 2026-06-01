package kube

import (
	"context"
	"fmt"

	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// RBACRequirement is a single (verb, resource) pair that must be allowed.
type RBACRequirement struct {
	Verb       string
	Group      string
	Resource   string
	Namespaced bool
}

// DefaultRBACRequirements returns the verbs/resources needed for a v1
// benchbuddy run scoped to a target namespace.
func DefaultRBACRequirements() []RBACRequirement {
	return []RBACRequirement{
		{Verb: "create", Resource: "pods", Namespaced: true},
		{Verb: "delete", Resource: "pods", Namespaced: true},
		{Verb: "list", Resource: "pods", Namespaced: true},
		{Verb: "get", Resource: "pods", Namespaced: true},
		{Verb: "create", Resource: "persistentvolumeclaims", Namespaced: true},
		{Verb: "delete", Resource: "persistentvolumeclaims", Namespaced: true},
		{Verb: "list", Resource: "persistentvolumeclaims", Namespaced: true},
		{Verb: "create", Resource: "configmaps", Namespaced: true},
		{Verb: "delete", Resource: "configmaps", Namespaced: true},
		{Verb: "list", Resource: "nodes"},
		{Verb: "list", Resource: "storageclasses", Group: "storage.k8s.io"},
		{Verb: "list", Resource: "namespaces"},
	}
}

// CheckNamespace returns an error if the namespace does not exist.
func CheckNamespace(ctx context.Context, cs kubernetes.Interface, namespace string) error {
	_, err := cs.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return fmt.Errorf("namespace %q does not exist", namespace)
	}
	if err != nil {
		return fmt.Errorf("check namespace: %w", err)
	}
	return nil
}

// CheckRBAC verifies each requirement via SelfSubjectAccessReview. Returns the
// list of unmet requirements (caller renders the remediation message).
func CheckRBAC(ctx context.Context, cs kubernetes.Interface, namespace string, reqs []RBACRequirement) ([]RBACRequirement, error) {
	var missing []RBACRequirement
	for _, r := range reqs {
		attrs := &authv1.ResourceAttributes{
			Verb:     r.Verb,
			Group:    r.Group,
			Resource: r.Resource,
		}
		if r.Namespaced {
			attrs.Namespace = namespace
		}
		ssar := &authv1.SelfSubjectAccessReview{
			Spec: authv1.SelfSubjectAccessReviewSpec{ResourceAttributes: attrs},
		}
		got, err := cs.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, ssar, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("RBAC check (%s %s): %w", r.Verb, r.Resource, err)
		}
		if !got.Status.Allowed {
			missing = append(missing, r)
		}
	}
	return missing, nil
}

// CountUsableNodes returns the number of nodes that are Ready and not
// cordoned (Spec.Unschedulable == false).
func CountUsableNodes(ctx context.Context, cs kubernetes.Interface) (int, error) {
	nodes, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, fmt.Errorf("list nodes: %w", err)
	}
	var n int
	for _, node := range nodes.Items {
		if node.Spec.Unschedulable {
			continue
		}
		for _, c := range node.Status.Conditions {
			if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
				n++
				break
			}
		}
	}
	return n, nil
}
