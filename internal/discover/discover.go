package discover

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Discovery is the cluster snapshot used to plan a run.
type Discovery struct {
	K8sVersion     string
	CNI            string   // "cilium", "calico", "flannel", "kindnet", "unknown"
	Nodes          []string // names of ready, non-cordoned nodes
	StorageClasses []string // sorted ascending
}

// Run inspects the cluster and returns a Discovery snapshot.
func Run(ctx context.Context, cs kubernetes.Interface) (Discovery, error) {
	var d Discovery

	ver, err := cs.Discovery().ServerVersion()
	if err != nil {
		return d, fmt.Errorf("server version: %w", err)
	}
	d.K8sVersion = ver.GitVersion

	nodes, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return d, fmt.Errorf("list nodes: %w", err)
	}
	for _, n := range nodes.Items {
		if n.Spec.Unschedulable {
			continue
		}
		ready := false
		for _, c := range n.Status.Conditions {
			if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
				ready = true
				break
			}
		}
		if ready {
			d.Nodes = append(d.Nodes, n.Name)
		}
	}
	sort.Strings(d.Nodes)

	scs, err := cs.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return d, fmt.Errorf("list storage classes: %w", err)
	}
	for _, sc := range scs.Items {
		d.StorageClasses = append(d.StorageClasses, sc.Name)
	}
	sort.Strings(d.StorageClasses)

	d.CNI = detectCNI(ctx, cs)
	return d, nil
}

// detectCNI scans kube-system DaemonSets and Pods for well-known CNI names.
func detectCNI(ctx context.Context, cs kubernetes.Interface) string {
	known := []string{"cilium", "calico", "flannel", "kindnet", "weave", "antrea"}

	dss, err := cs.AppsV1().DaemonSets("kube-system").List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, ds := range dss.Items {
			lower := strings.ToLower(ds.Name)
			for _, k := range known {
				if strings.Contains(lower, k) {
					return k
				}
			}
		}
	}

	pods, err := cs.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, p := range pods.Items {
			lower := strings.ToLower(p.Name)
			for _, k := range known {
				if strings.Contains(lower, k) {
					return k
				}
			}
		}
	}
	return "unknown"
}
