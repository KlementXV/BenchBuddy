package discover

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func node(name string, cordoned bool, ready corev1.ConditionStatus) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       corev1.NodeSpec{Unschedulable: cordoned},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: ready}},
		},
	}
}

func TestRun_FiltersUnusableNodes(t *testing.T) {
	cs := fake.NewSimpleClientset(
		node("n1", false, corev1.ConditionTrue),
		node("n2", false, corev1.ConditionTrue),
		node("n3-cordoned", true, corev1.ConditionTrue),
		node("n4-notready", false, corev1.ConditionFalse),
	)
	cs.Discovery().(*fakediscovery.FakeDiscovery).FakedServerVersion = &version.Info{GitVersion: "v1.30.0"}

	d, err := Run(context.Background(), cs)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Nodes) != 2 {
		t.Errorf("nodes: got %d, want 2; %+v", len(d.Nodes), d.Nodes)
	}
	if d.K8sVersion != "v1.30.0" {
		t.Errorf("k8s version: %q", d.K8sVersion)
	}
}

func TestRun_SortsStorageClasses(t *testing.T) {
	cs := fake.NewSimpleClientset(
		node("n1", false, corev1.ConditionTrue),
		node("n2", false, corev1.ConditionTrue),
		&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "zzz"}},
		&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "aaa"}},
	)
	cs.Discovery().(*fakediscovery.FakeDiscovery).FakedServerVersion = &version.Info{GitVersion: "v1.30.0"}

	d, err := Run(context.Background(), cs)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.StorageClasses) != 2 || d.StorageClasses[0] != "aaa" || d.StorageClasses[1] != "zzz" {
		t.Errorf("storage classes not sorted: %v", d.StorageClasses)
	}
}

func TestRun_DetectsCNIFromKubeSystem(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cilium",
			Namespace: "kube-system",
		},
	}
	cs := fake.NewSimpleClientset(
		node("n1", false, corev1.ConditionTrue),
		node("n2", false, corev1.ConditionTrue),
		ds,
	)
	cs.Discovery().(*fakediscovery.FakeDiscovery).FakedServerVersion = &version.Info{GitVersion: "v1.30.0"}

	d, err := Run(context.Background(), cs)
	if err != nil {
		t.Fatal(err)
	}
	if d.CNI != "cilium" {
		t.Errorf("CNI: %q, want cilium", d.CNI)
	}
}
