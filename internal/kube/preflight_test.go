package kube

import (
	"context"
	"testing"

	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestCheckNamespace_Exists(t *testing.T) {
	cs := fake.NewSimpleClientset(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "benchbuddy"},
	})
	if err := CheckNamespace(context.Background(), cs, "benchbuddy"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCheckNamespace_Missing(t *testing.T) {
	cs := fake.NewSimpleClientset()
	err := CheckNamespace(context.Background(), cs, "missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCheckRBAC_AllAllowed(t *testing.T) {
	cs := fake.NewSimpleClientset()
	cs.PrependReactor("create", "selfsubjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, &authv1.SelfSubjectAccessReview{
			Status: authv1.SubjectAccessReviewStatus{Allowed: true},
		}, nil
	})
	missing, err := CheckRBAC(context.Background(), cs, "benchbuddy", DefaultRBACRequirements())
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Errorf("expected no missing permissions, got %v", missing)
	}
}

func TestCheckRBAC_SomeDenied(t *testing.T) {
	cs := fake.NewSimpleClientset()
	cs.PrependReactor("create", "selfsubjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		ssar := action.(k8stesting.CreateAction).GetObject().(*authv1.SelfSubjectAccessReview)
		// Deny create on pvc, allow everything else
		allowed := !(ssar.Spec.ResourceAttributes.Resource == "persistentvolumeclaims" &&
			ssar.Spec.ResourceAttributes.Verb == "create")
		return true, &authv1.SelfSubjectAccessReview{
			Status: authv1.SubjectAccessReviewStatus{Allowed: allowed},
		}, nil
	})
	missing, err := CheckRBAC(context.Background(), cs, "benchbuddy", DefaultRBACRequirements())
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) == 0 {
		t.Fatal("expected at least one missing permission")
	}
	if missing[0].Resource != "persistentvolumeclaims" || missing[0].Verb != "create" {
		t.Errorf("unexpected missing permission: %#v", missing[0])
	}
}

func TestCheckNodes_RequiresAtLeastTwoReady(t *testing.T) {
	readyNode := func(name string, cordoned bool, condStatus corev1.ConditionStatus) *corev1.Node {
		return &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       corev1.NodeSpec{Unschedulable: cordoned},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: condStatus}},
			},
		}
	}
	cs := fake.NewSimpleClientset(
		readyNode("a", false, corev1.ConditionTrue),
		readyNode("b", true, corev1.ConditionTrue),   // cordoned
		readyNode("c", false, corev1.ConditionFalse), // not ready
	)
	usable, err := CountUsableNodes(context.Background(), cs)
	if err != nil {
		t.Fatal(err)
	}
	if usable != 1 {
		t.Errorf("expected 1 usable node, got %d", usable)
	}
}
